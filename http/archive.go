package fbhttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	gopath "path"
	"strings"

	"github.com/filebrowser/filebrowser/v2/fileutils"
	"github.com/mholt/archives"
)

// Limits on what one extraction may write, against archive bombs.
var (
	maxExtractBytes   int64 = 20 << 30
	maxExtractEntries       = 1_000_000
)

type archiveRequest struct {
	Action      string   `json:"action"`      // "compress" or "extract"
	Paths       []string `json:"paths"`       // compress: what to pack
	Path        string   `json:"path"`        // extract: the archive
	Destination string   `json:"destination"` // compress: the archive file; extract: the folder
	Format      string   `json:"format"`      // compress: zip, tar, tar.gz, tar.bz2, tar.xz, tar.zst
	Overwrite   bool     `json:"overwrite"`   // extract: replace files that exist
}

type archiveResult struct {
	Path    string   `json:"path"`
	Files   int      `json:"files"`
	Size    int64    `json:"size"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func archiveFormat(name string) (string, archives.Archival, bool) {
	switch name {
	case "zip":
		return ".zip", archives.Zip{}, true
	case "tar":
		return ".tar", archives.Tar{}, true
	case "tar.gz":
		return ".tar.gz", archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}, true
	case "tar.bz2":
		return ".tar.bz2", archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}}, true
	case "tar.xz":
		return ".tar.xz", archives.CompressedArchive{Compression: archives.Xz{}, Archival: archives.Tar{}}, true
	case "tar.zst":
		return ".tar.zst", archives.CompressedArchive{Compression: archives.Zstd{}, Archival: archives.Tar{}}, true
	}

	return "", nil, false
}

var errBadArchiveRequest = errors.New("bad archive request")

func archiveBadRequest(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errBadArchiveRequest, fmt.Sprintf(format, args...))
}

func archiveErrToStatus(err error) (int, error) {
	switch {
	case errors.Is(err, errBadArchiveRequest):
		return http.StatusBadRequest, err
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden, err
	case errors.Is(err, errExtractLimit):
		return http.StatusRequestEntityTooLarge, err
	default:
		return errToStatus(err), err
	}
}

var archivePostHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Create {
		return http.StatusForbidden, nil
	}

	var req archiveRequest
	if err := decodeJSONBody(r, &req); err != nil {
		return http.StatusBadRequest, archiveBadRequest("%v", err)
	}

	var (
		res archiveResult
		err error
	)
	switch req.Action {
	case "compress":
		res, err = compress(r.Context(), d, req)
		if err == nil {
			d.audit(r, "compress", res.Path, "", fmt.Sprintf("%d paths, %s", len(req.Paths), filesizeLabel(res.Size)))
		}
	case "extract":
		res, err = extract(r.Context(), d, req)
		if err == nil {
			d.audit(r, "extract", slashClean(req.Path), res.Path, fmt.Sprintf("%d files, %s, %d skipped", res.Files, filesizeLabel(res.Size), res.Skipped))
		}
	default:
		return http.StatusBadRequest, archiveBadRequest("action must be compress or extract")
	}
	if err != nil {
		return archiveErrToStatus(err)
	}

	return renderJSON(w, r, res)
})

func compress(ctx context.Context, d *data, req archiveRequest) (archiveResult, error) {
	ext, archiver, ok := archiveFormat(req.Format)
	if !ok {
		return archiveResult{}, archiveBadRequest("unknown format %q", req.Format)
	}
	if len(req.Paths) == 0 {
		return archiveResult{}, archiveBadRequest("nothing to compress")
	}

	dest := slashClean(req.Destination)
	if dest == "/" || !d.Check(dest) || isInTrash(dest) {
		return archiveResult{}, fmt.Errorf("%w: archive path not allowed", os.ErrPermission)
	}
	if !strings.HasSuffix(strings.ToLower(dest), ext) {
		dest += ext
	}
	if _, err := d.user.Fs.Stat(dest); err == nil {
		dest = addVersionSuffix(dest, d.user.Fs)
	}

	paths := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		p = slashClean(p)
		if p == "/" {
			return archiveResult{}, archiveBadRequest("choose files or folders, not the scope root")
		}
		if !d.Check(p) {
			return archiveResult{}, fmt.Errorf("%w: %s", os.ErrPermission, p)
		}
		paths = append(paths, p)
	}

	// An item inside another selected folder is packed with that folder.
	paths = dropNested(paths)

	// Names inside the archive are relative to the folder the items share.
	commonDir := fileutils.CommonPrefix('/', paths...)
	if len(paths) == 1 {
		commonDir = gopath.Dir(paths[0])
	}

	var all []archives.FileInfo
	for _, p := range paths {
		infos, err := getFiles(d, p, commonDir)
		if err != nil {
			return archiveResult{}, err
		}
		all = append(all, infos...)
	}

	// Never pack the archive into itself.
	kept := all[:0]
	for _, info := range all {
		if gopath.Join(commonDir, info.NameInArchive) != dest {
			kept = append(kept, info)
		}
	}
	all = kept
	if len(all) == 0 {
		return archiveResult{}, archiveBadRequest("nothing to compress")
	}

	if err := d.user.Fs.MkdirAll(gopath.Dir(dest), d.settings.DirMode); err != nil {
		return archiveResult{}, err
	}
	out, err := d.user.Fs.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, d.settings.FileMode)
	if err != nil {
		return archiveResult{}, err
	}

	err = archiver.Archive(ctx, out, all)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = d.user.Fs.Remove(dest)
		return archiveResult{}, err
	}

	info, err := d.user.Fs.Stat(dest)
	if err != nil {
		return archiveResult{}, err
	}

	files := 0
	for _, f := range all {
		if !f.IsDir() {
			files++
		}
	}

	return archiveResult{Path: dest, Files: files, Size: info.Size(), Errors: []string{}}, nil
}

// dropNested removes the paths that lie inside another path of the list.
func dropNested(paths []string) []string {
	var out []string
	for _, p := range paths {
		nested := false
		for _, other := range paths {
			if other != p && strings.HasPrefix(p, other+"/") {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, p)
		}
	}

	return out
}

var errExtractLimit = errors.New("the archive is larger than the extraction limit")

// extractTarget turns the name of an archive entry into a path inside dest, or
// reports false for names that must not be extracted.
func extractTarget(dest, name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return "", false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", false
		}
	}

	cleaned := gopath.Clean(name)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}

	return gopath.Join(dest, cleaned), true
}

func extract(ctx context.Context, d *data, req archiveRequest) (archiveResult, error) {
	src := slashClean(req.Path)
	dest := slashClean(req.Destination)
	if src == "/" || dest == "/" {
		return archiveResult{}, archiveBadRequest("choose an archive and a folder inside the scope")
	}
	if !d.Check(src) || !d.Check(dest) || isInTrash(dest) {
		return archiveResult{}, fmt.Errorf("%w: path not allowed", os.ErrPermission)
	}

	in, err := d.user.Fs.Open(src)
	if err != nil {
		return archiveResult{}, err
	}
	defer in.Close()

	if info, statErr := in.Stat(); statErr != nil || info.IsDir() {
		return archiveResult{}, archiveBadRequest("%s is not a file", src)
	}

	format, _, err := archives.Identify(ctx, gopath.Base(src), in)
	if err != nil {
		return archiveResult{}, archiveBadRequest("%s is not a supported archive", gopath.Base(src))
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return archiveResult{}, archiveBadRequest("%s cannot be extracted", gopath.Base(src))
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return archiveResult{}, err
	}

	_, existed := d.user.Fs.Stat(dest)
	createdDest := existed != nil
	if err := d.user.Fs.MkdirAll(dest, d.settings.DirMode); err != nil {
		return archiveResult{}, err
	}

	res := archiveResult{Path: dest, Errors: []string{}}
	note := func(format string, args ...interface{}) {
		res.Skipped++
		if len(res.Errors) < 5 {
			res.Errors = append(res.Errors, fmt.Sprintf(format, args...))
		}
	}
	entries := 0

	err = extractor.Extract(ctx, in, func(ctx context.Context, info archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxExtractEntries {
			return errExtractLimit
		}

		target, ok := extractTarget(dest, info.NameInArchive)
		if !ok {
			note("%s: unsafe or empty name", info.NameInArchive)
			return nil
		}
		if !d.CheckRules(target) {
			note("%s: not allowed here", info.NameInArchive)
			return nil
		}

		mode := info.Mode()
		switch {
		case mode&fs.ModeSymlink != 0 || info.LinkTarget != "":
			note("%s: links are not extracted", info.NameInArchive)
			return nil
		case info.IsDir():
			return d.user.Fs.MkdirAll(target, d.settings.DirMode)
		case !mode.IsRegular():
			note("%s: not a regular file", info.NameInArchive)
			return nil
		}

		if _, statErr := d.user.Fs.Stat(target); statErr == nil && !req.Overwrite {
			note("%s: already exists", info.NameInArchive)
			return nil
		}
		if err := d.user.Fs.MkdirAll(gopath.Dir(target), d.settings.DirMode); err != nil {
			return err
		}

		src, err := info.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		out, err := d.user.Fs.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, d.settings.FileMode)
		if err != nil {
			return err
		}
		// Read one byte more than the budget allows to tell when it is crossed.
		written, copyErr := io.Copy(out, io.LimitReader(src, maxExtractBytes-res.Size+1))
		if closeErr := out.Close(); copyErr == nil {
			copyErr = closeErr
		}
		res.Size += written
		if copyErr != nil {
			return copyErr
		}
		if res.Size > maxExtractBytes {
			return errExtractLimit
		}

		if !info.ModTime().IsZero() {
			_ = d.user.Fs.Chtimes(target, info.ModTime(), info.ModTime())
		}
		res.Files++
		return nil
	})
	if err != nil {
		// A folder this extraction made is not left half full.
		if createdDest {
			_ = d.user.Fs.RemoveAll(dest)
		}

		// What is not a limit or a file system problem is the archive's fault.
		var pathErr *fs.PathError
		if !errors.Is(err, errExtractLimit) && !errors.Is(err, os.ErrPermission) && !errors.As(err, &pathErr) &&
			!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return archiveResult{}, archiveBadRequest("could not read %s: %v", gopath.Base(src), err)
		}
		return archiveResult{}, err
	}

	return res, nil
}
