package fbhttp

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	gopath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/syncthing"
)

// Syncthing renames the losing side of a conflict to
// <name>.sync-conflict-<date>-<time>[-<device>]<.ext>.
var syncConflictPattern = regexp.MustCompile(`^(.+)\.sync-conflict-(\d{8})-(\d{6})(?:-([A-Z0-9]{7}))?(\.[^./]*)?$`)

const (
	maxSyncConflicts = 1000
	maxIgnoreLines   = 1000
	maxIgnoreLength  = 500
	maxKbps          = 10_000_000
)

type syncConflict struct {
	FolderID         string    `json:"folderID"`
	FolderLabel      string    `json:"folderLabel"`
	Path             string    `json:"path"`
	Original         string    `json:"original"`
	Size             int64     `json:"size"`
	Modified         time.Time `json:"modified"`
	ConflictedAt     time.Time `json:"conflictedAt"`
	OriginalExists   bool      `json:"originalExists"`
	OriginalSize     int64     `json:"originalSize"`
	OriginalModified time.Time `json:"originalModified"`
}

// conflictOriginal returns the name a conflict file was made from.
func conflictOriginal(name string) (original string, at time.Time, ok bool) {
	m := syncConflictPattern.FindStringSubmatch(name)
	if m == nil {
		return "", time.Time{}, false
	}

	at, _ = time.ParseInLocation("20060102 150405", m[2]+" "+m[3], time.Local)
	return m[1] + m[5], at, true
}

// findSyncConflicts walks the synced folders inside the user's scope.
func findSyncConflicts(d *data, folders []syncthing.Folder) []syncConflict {
	root, err := realScopeRoot(d)
	if err != nil {
		return nil
	}

	var found []syncConflict
	for _, folder := range folders {
		rel, relErr := filepath.Rel(root, folder.Path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == "." {
			continue
		}

		_ = filepath.WalkDir(folder.Path, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if name := entry.Name(); name == ".stfolder" || name == ".stversions" {
					return fs.SkipDir
				}
				return nil
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil
			}

			originalName, at, ok := conflictOriginal(entry.Name())
			if !ok {
				return nil
			}
			if len(found) >= maxSyncConflicts {
				return fs.SkipAll
			}

			fileRel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			virtual := gopath.Join("/", filepath.ToSlash(fileRel))
			original := gopath.Join(gopath.Dir(virtual), originalName)
			if !d.Check(virtual) {
				return nil
			}

			c := syncConflict{
				FolderID: folder.ID, FolderLabel: folder.Label, Path: virtual, Original: original, ConflictedAt: at,
			}
			if info, infoErr := entry.Info(); infoErr == nil {
				c.Size, c.Modified = info.Size(), info.ModTime()
			}
			if info, statErr := os.Lstat(filepath.Join(filepath.Dir(path), originalName)); statErr == nil {
				c.OriginalExists, c.OriginalSize, c.OriginalModified = true, info.Size(), info.ModTime()
			}
			found = append(found, c)
			return nil
		})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].ConflictedAt.After(found[j].ConflictedAt) })
	return found
}

var syncConflictsGetHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	folders, err := c.Folders(r.Context())
	if err != nil {
		return syncErrToStatus(err)
	}

	found := findSyncConflicts(d, folders)
	if found == nil {
		found = []syncConflict{}
	}

	return renderJSON(w, r, found)
})

// discard gets rid of a file: into the trash when that is on, else for good.
func discardFile(d *data, p string) error {
	if d.settings.TrashConfig().Enabled && !isInTrash(p) {
		if ok, err := moveToTrash(d, p); err != nil || ok {
			return err
		}
	}

	return d.user.Fs.RemoveAll(p)
}

var syncConflictResolveHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req struct {
		Path string `json:"path"`
		Keep string `json:"keep"` // "conflict" or "original"
	}
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}
	if req.Keep != "conflict" && req.Keep != "original" {
		return syncErrToStatus(syncBadRequest(`keep must be "conflict" or "original"`))
	}

	p := slashClean(req.Path)
	originalName, _, ok := conflictOriginal(gopath.Base(p))
	if !ok {
		return syncErrToStatus(syncBadRequest("%s is not a sync conflict file", p))
	}
	if _, info, err := resolvePermissionTarget(d, p); err != nil {
		return syncPathErr(err)
	} else if info.IsDir() {
		return syncErrToStatus(syncBadRequest("%s is a folder", p))
	}

	// The file must lie in a folder that is synced.
	folders, err := c.Folders(r.Context())
	if err != nil {
		return syncErrToStatus(err)
	}
	inSynced := false
	for _, f := range findSyncConflicts(d, folders) {
		if f.Path == p {
			inSynced = true
			break
		}
	}
	if !inSynced {
		return http.StatusNotFound, nil
	}

	original := gopath.Join(gopath.Dir(p), originalName)
	if !d.Check(original) {
		return http.StatusForbidden, nil
	}

	if req.Keep == "original" {
		if err := discardFile(d, p); err != nil {
			return errToStatus(err), err
		}
	} else {
		if _, err := d.user.Fs.Stat(original); err == nil {
			if err := discardFile(d, original); err != nil {
				return errToStatus(err), err
			}
		}
		if err := d.user.Fs.Rename(p, original); err != nil {
			return errToStatus(err), err
		}
	}

	d.audit(r, "sync_resolve", original, "", "kept the "+req.Keep+" version")
	return renderJSON(w, r, map[string]string{"path": original})
})

type syncIgnores struct {
	Lines []string `json:"lines"`
}

var syncIgnoresGetHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	lines, err := c.Ignores(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		return syncErrToStatus(err)
	}

	return renderJSON(w, r, syncIgnores{Lines: lines})
})

var syncIgnoresPutHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req syncIgnores
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}
	if len(req.Lines) > maxIgnoreLines {
		return syncErrToStatus(syncBadRequest("at most %d lines", maxIgnoreLines))
	}
	for _, line := range req.Lines {
		switch {
		case len(line) > maxIgnoreLength:
			return syncErrToStatus(syncBadRequest("a line is longer than %d characters", maxIgnoreLength))
		case strings.ContainsAny(line, "\x00\r\n"):
			return syncErrToStatus(syncBadRequest("a line has a control character"))
		case strings.HasPrefix(strings.TrimSpace(line), "#include"):
			// It would make Syncthing read another file.
			return syncErrToStatus(syncBadRequest("#include is not allowed"))
		}
	}

	if err := c.SetIgnores(r.Context(), mux.Vars(r)["id"], req.Lines); err != nil {
		return syncErrToStatus(err)
	}

	d.audit(r, "sync_ignores", "", "", fmt.Sprintf("%s: %d patterns", mux.Vars(r)["id"], len(req.Lines)))
	return http.StatusNoContent, nil
})

var syncLimitsGetHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	limits, err := c.Limits(r.Context())
	if err != nil {
		return syncErrToStatus(err)
	}

	return renderJSON(w, r, limits)
})

var syncLimitsPutHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req syncthing.Limits
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}
	if req.MaxRecvKbps < 0 || req.MaxSendKbps < 0 || req.MaxRecvKbps > maxKbps || req.MaxSendKbps > maxKbps {
		return syncErrToStatus(syncBadRequest("limits are between 0 and %d KiB/s", maxKbps))
	}

	if err := c.SetLimits(r.Context(), req); err != nil {
		return syncErrToStatus(err)
	}

	d.audit(r, "sync_limits", "", "", fmt.Sprintf("down %d KiB/s, up %d KiB/s", req.MaxRecvKbps, req.MaxSendKbps))
	return http.StatusNoContent, nil
})
