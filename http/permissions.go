package fbhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	osuser "os/user"
	gopath "path"
	"path/filepath"
	"strconv"
	"strings"
)

// maxPermissionEntries bounds a recursive permission change.
const maxPermissionEntries = 200000

var errBadPermissions = errors.New("bad permissions request")

func permBadRequest(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errBadPermissions, fmt.Sprintf(format, args...))
}

type permissionsInfo struct {
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Mode     string `json:"mode"` // octal permission bits, e.g. "0644"
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Owner    string `json:"owner"` // name of UID in this server's user database, if any
	Group    string `json:"group"`
	CanChown bool   `json:"canChown"`
}

type permissionsRequest struct {
	Mode      string  `json:"mode"`      // for the item itself
	Recursive bool    `json:"recursive"` // also change everything inside a folder
	FileMode  string  `json:"fileMode"`  // for files inside, with recursive
	DirMode   string  `json:"dirMode"`   // for folders inside, with recursive
	Owner     *string `json:"owner"`     // user name or numeric id
	Group     *string `json:"group"`     // group name or numeric id
}

type permissionsResult struct {
	Changed int      `json:"changed"`
	Skipped int      `json:"skipped"` // symlinks and entries the rules hide
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// canChown reports whether the user may change owners: only administrators,
// and only when the server process is able to (it must run as root).
func canChown(d *data) bool {
	return d.user.Perm.Admin && canChownProcess()
}

// parsePermissionMode parses an octal permission string. Only the nine
// rwx bits are accepted: setuid, setgid and the sticky bit are refused.
func parsePermissionMode(s string) (os.FileMode, error) {
	s = strings.TrimSpace(s)
	value, err := strconv.ParseUint(s, 8, 32)
	if err != nil || s == "" || value > 0o777 {
		return 0, permBadRequest("%q is not a valid mode, use octal like 0644 (no setuid, setgid or sticky bit)", s)
	}

	return os.FileMode(value), nil
}

func optionalMode(s string) (*os.FileMode, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	mode, err := parsePermissionMode(s)
	if err != nil {
		return nil, err
	}

	return &mode, nil
}

// lookupOwnerID resolves a user or group given by number or by name.
func lookupOwnerID(value string, group bool) (int, error) {
	value = strings.TrimSpace(value)
	if id, err := strconv.Atoi(value); err == nil {
		if id < 0 {
			return 0, permBadRequest("ids cannot be negative")
		}
		return id, nil
	}

	var raw string
	if group {
		g, err := osuser.LookupGroup(value)
		if err != nil {
			return 0, permBadRequest("unknown group %q, use a name known to the server or a numeric id", value)
		}
		raw = g.Gid
	} else {
		u, err := osuser.Lookup(value)
		if err != nil {
			return 0, permBadRequest("unknown user %q, use a name known to the server or a numeric id", value)
		}
		raw = u.Uid
	}

	id, err := strconv.Atoi(raw)
	if err != nil {
		return 0, permBadRequest("%q has no numeric id", value)
	}

	return id, nil
}

func optionalOwnerID(value *string, group bool) (*int, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	id, err := lookupOwnerID(*value, group)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

// resolvePermissionTarget validates the request path and returns its on-disk
// path and metadata. The scope root is refused (changing the permissions of,
// say, a served home directory can lock users out of SSH), and so is any path
// that is, or goes through, a symbolic link.
func resolvePermissionTarget(d *data, p string) (string, os.FileInfo, error) {
	p = slashClean(p)
	if p == "/" {
		return "", nil, permBadRequest("the scope root cannot be changed here")
	}
	if !d.Check(p) {
		return "", nil, fmt.Errorf("%w: path not allowed", os.ErrPermission)
	}

	root, err := realScopeRoot(d)
	if err != nil {
		return "", nil, err
	}
	real, err := filepath.EvalSymlinks(d.user.FullPath(p))
	if err != nil {
		return "", nil, err
	}
	if real != filepath.Join(root, filepath.FromSlash(p)) {
		return "", nil, permBadRequest("%s is, or is inside, a symbolic link", p)
	}

	info, err := os.Lstat(real)
	if err != nil {
		return "", nil, err
	}

	return real, info, nil
}

func decodeJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("empty request")
	}

	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

func permissionsErrToStatus(err error) (int, error) {
	switch {
	case errors.Is(err, errBadPermissions):
		return http.StatusBadRequest, err
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden, err
	default:
		return errToStatus(err), err
	}
}

var permissionsGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	_, info, err := resolvePermissionTarget(d, r.URL.Path)
	if err != nil {
		return permissionsErrToStatus(err)
	}

	out := permissionsInfo{
		Path:     slashClean(r.URL.Path),
		IsDir:    info.IsDir(),
		Mode:     fmt.Sprintf("%04o", info.Mode().Perm()),
		CanChown: canChown(d),
	}
	if uid, gid, ok := fileOwner(info); ok {
		out.UID, out.GID = uid, gid
		if u, lookupErr := osuser.LookupId(strconv.Itoa(uid)); lookupErr == nil {
			out.Owner = u.Username
		}
		if g, lookupErr := osuser.LookupGroupId(strconv.Itoa(gid)); lookupErr == nil {
			out.Group = g.Name
		}
	}

	return renderJSON(w, r, out)
})

type permissionEntry struct {
	real  string
	rel   string
	isDir bool
}

var permissionsPatchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Modify {
		return http.StatusForbidden, nil
	}

	var req permissionsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		return http.StatusBadRequest, permBadRequest("%v", err)
	}

	mode, err := optionalMode(req.Mode)
	if err != nil {
		return permissionsErrToStatus(err)
	}
	fileMode, err := optionalMode(req.FileMode)
	if err != nil {
		return permissionsErrToStatus(err)
	}
	dirMode, err := optionalMode(req.DirMode)
	if err != nil {
		return permissionsErrToStatus(err)
	}
	uid, err := optionalOwnerID(req.Owner, false)
	if err != nil {
		return permissionsErrToStatus(err)
	}
	gid, err := optionalOwnerID(req.Group, true)
	if err != nil {
		return permissionsErrToStatus(err)
	}

	if (uid != nil || gid != nil) && !canChown(d) {
		return http.StatusForbidden, nil
	}
	if mode == nil && uid == nil && gid == nil && fileMode == nil && dirMode == nil {
		return http.StatusBadRequest, permBadRequest("nothing to change")
	}
	if !req.Recursive && (fileMode != nil || dirMode != nil) {
		return http.StatusBadRequest, permBadRequest("fileMode and dirMode need recursive")
	}

	p := slashClean(r.URL.Path)
	real, info, err := resolvePermissionTarget(d, p)
	if err != nil {
		return permissionsErrToStatus(err)
	}
	if req.Recursive && !info.IsDir() {
		return http.StatusBadRequest, permBadRequest("%s is not a folder", p)
	}

	// Collect everything first and apply deepest-first, so that tightening a
	// folder's mode cannot stop us from reaching what is inside it.
	entries := []permissionEntry{{real: real, rel: "", isDir: info.IsDir()}}
	result := permissionsResult{Errors: []string{}}
	if req.Recursive {
		walkErr := filepath.WalkDir(real, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				result.Failed++
				return nil
			}
			if path == real {
				return nil
			}
			if err := r.Context().Err(); err != nil {
				return err
			}

			rel, relErr := filepath.Rel(real, path)
			if relErr != nil {
				return nil
			}
			virtual := gopath.Join(p, filepath.ToSlash(rel))

			if entry.Type()&fs.ModeSymlink != 0 || !d.CheckRules(virtual) {
				result.Skipped++
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if len(entries) >= maxPermissionEntries {
				return permBadRequest("more than %d items, change a smaller folder", maxPermissionEntries)
			}

			entries = append(entries, permissionEntry{real: path, rel: rel, isDir: entry.IsDir()})
			return nil
		})
		if walkErr != nil {
			return permissionsErrToStatus(walkErr)
		}
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		var entryMode *os.FileMode
		switch {
		case i == 0:
			entryMode = mode
		case entry.isDir:
			entryMode = dirMode
		default:
			entryMode = fileMode
		}

		if entryMode == nil && uid == nil && gid == nil {
			continue // nothing to change for this entry
		}

		if err := applyPermissions(entry.real, entryMode, uid, gid); err != nil {
			result.Failed++
			if i == 0 && len(entries) == 1 {
				return permissionsErrToStatus(err)
			}
			if len(result.Errors) < 5 {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", gopath.Join(p, filepath.ToSlash(entry.rel)), err))
			}
			continue
		}
		result.Changed++
	}

	log.Printf("permissions: user %q changed %s mode=%q fileMode=%q dirMode=%q owner=%v group=%v recursive=%v: %d changed, %d skipped, %d failed",
		d.user.Username, p, req.Mode, req.FileMode, req.DirMode, derefString(req.Owner), derefString(req.Group), req.Recursive, result.Changed, result.Skipped, result.Failed)

	return renderJSON(w, r, result)
})

func derefString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func applyPermissions(path string, mode *os.FileMode, uid, gid *int) error {
	if mode != nil {
		if err := os.Chmod(path, *mode); err != nil {
			return err
		}
	}

	if uid != nil || gid != nil {
		newUID, newGID := -1, -1
		if uid != nil {
			newUID = *uid
		}
		if gid != nil {
			newGID = *gid
		}

		if err := chownNoFollow(path, newUID, newGID); err != nil {
			return err
		}
	}

	return nil
}
