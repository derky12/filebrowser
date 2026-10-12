package fbhttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	gopath "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/settings"
)

// davShareName is what a share may be called: it becomes a URL path element.
var davShareName = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._-]{0,62}$`)

var errDavBadRequest = errors.New("bad webdav request")

func davBadRequest(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errDavBadRequest, fmt.Sprintf(format, args...))
}

// davAdminMu keeps two changes of the shares from overwriting each other.
var davAdminMu sync.Mutex

type webdavShareInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	ReadOnly  bool   `json:"readOnly"`
	URL       string `json:"url"`
	Available bool   `json:"available"` // the folder still exists
	By        string `json:"by"`
}

type webdavInfo struct {
	Enabled bool              `json:"enabled"`
	User    string            `json:"user"`
	BaseURL string            `json:"baseURL"`
	Shares  []webdavShareInfo `json:"shares"`
}

// davBaseURL is the address clients use for the WebDAV server: the configured
// public URL, or this host with the published port.
func davBaseURL(r *http.Request, server *settings.Server) string {
	if server.WebDAVPublicURL != "" {
		return server.WebDAVPublicURL
	}

	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	port := server.WebDAVHostPort
	if port == "" {
		port = server.WebDAVPort
	}

	return "http://" + net.JoinHostPort(host, port) + server.WebDAVPrefix
}

func davShareURL(base, name string) string {
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(name) + "/"
}

func davShareInfoOf(base, rootReal string, share settings.WebDAVShare) webdavShareInfo {
	// Usable the way the server judges it: a real folder, not behind a symlink.
	available := false
	if rootReal != "" {
		base := filepath.Join(rootReal, filepath.FromSlash(share.Dir))
		real, err := filepath.EvalSymlinks(base)
		if err == nil && real == base {
			info, statErr := os.Stat(real)
			available = statErr == nil && info.IsDir()
		}
	}

	return webdavShareInfo{
		Name: share.Name, Path: share.Path, ReadOnly: share.ReadOnly,
		URL: davShareURL(base, share.Name), Available: available, By: share.By,
	}
}

// withWebDAV restricts a handler to administrators and to servers where the
// WebDAV server is switched on.
func withWebDAV(fn handleFunc) handleFunc {
	return withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.server.WebDAV {
			return http.StatusNotFound, nil
		}

		return fn(w, r, d)
	})
}

func davErrToStatus(err error) (int, error) {
	switch {
	case errors.Is(err, errDavBadRequest), errors.Is(err, errBadPermissions):
		return http.StatusBadRequest, err
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden, err
	default:
		return errToStatus(err), err
	}
}

var webdavGetHandler = withWebDAV(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	base := davBaseURL(r, d.server)
	rootReal, _ := filepath.EvalSymlinks(d.server.Root)

	out := webdavInfo{Enabled: true, User: d.server.WebDAVUser, BaseURL: base, Shares: []webdavShareInfo{}}
	for _, share := range d.settings.WebDAVShares {
		out.Shares = append(out.Shares, davShareInfoOf(base, rootReal, share))
	}

	return renderJSON(w, r, out)
})

var webdavShareCreateHandler = withWebDAV(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var req struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		ReadOnly *bool  `json:"readOnly"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		return http.StatusBadRequest, davBadRequest("%v", err)
	}

	real, info, err := resolvePermissionTarget(d, req.Path)
	if err != nil {
		return davErrToStatus(err)
	}
	if !info.IsDir() {
		return http.StatusBadRequest, davBadRequest("%s is not a folder", slashClean(req.Path))
	}

	rootReal, err := filepath.EvalSymlinks(d.server.Root)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	rel, err := filepath.Rel(rootReal, real)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return http.StatusForbidden, nil
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = gopath.Base(slashClean(req.Path))
	}
	if !davShareName.MatchString(name) {
		return http.StatusBadRequest, davBadRequest("%q cannot be a share name: use letters, digits, spaces, dots, dashes and underscores", name)
	}

	davAdminMu.Lock()
	defer davAdminMu.Unlock()

	for _, existing := range d.settings.WebDAVShares {
		if strings.EqualFold(existing.Name, name) {
			return http.StatusBadRequest, davBadRequest("a share called %q already exists", existing.Name)
		}
	}

	share := settings.WebDAVShare{
		Name:     name,
		Dir:      "/" + filepath.ToSlash(rel),
		Path:     slashClean(req.Path),
		ReadOnly: req.ReadOnly == nil || *req.ReadOnly, // read-only unless asked otherwise
		By:       d.user.Username,
		Created:  time.Now(),
	}
	d.settings.WebDAVShares = append(d.settings.WebDAVShares, share)
	if err := d.store.Settings.Save(d.settings); err != nil {
		return http.StatusInternalServerError, err
	}
	davShareVersion.Add(1)

	mode := "read-write"
	if share.ReadOnly {
		mode = "read-only"
	}
	d.audit(r, "webdav_share", share.Path, "", name+", "+mode)

	return renderJSON(w, r, davShareInfoOf(davBaseURL(r, d.server), rootReal, share))
})

var webdavSharePatchHandler = withWebDAV(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var req struct {
		ReadOnly *bool `json:"readOnly"`
	}
	if err := decodeJSONBody(r, &req); err != nil || req.ReadOnly == nil {
		return http.StatusBadRequest, davBadRequest("readOnly is required")
	}

	davAdminMu.Lock()
	defer davAdminMu.Unlock()

	name := mux.Vars(r)["name"]
	for i := range d.settings.WebDAVShares {
		if !strings.EqualFold(d.settings.WebDAVShares[i].Name, name) {
			continue
		}

		d.settings.WebDAVShares[i].ReadOnly = *req.ReadOnly
		if err := d.store.Settings.Save(d.settings); err != nil {
			return http.StatusInternalServerError, err
		}
		davShareVersion.Add(1)

		mode := "read-write"
		if *req.ReadOnly {
			mode = "read-only"
		}
		d.audit(r, "webdav_mode", d.settings.WebDAVShares[i].Path, "", d.settings.WebDAVShares[i].Name+", "+mode)

		rootReal, _ := filepath.EvalSymlinks(d.server.Root)
		return renderJSON(w, r, davShareInfoOf(davBaseURL(r, d.server), rootReal, d.settings.WebDAVShares[i]))
	}

	return http.StatusNotFound, nil
})

var webdavShareDeleteHandler = withWebDAV(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	davAdminMu.Lock()
	defer davAdminMu.Unlock()

	name := mux.Vars(r)["name"]
	for i, share := range d.settings.WebDAVShares {
		if !strings.EqualFold(share.Name, name) {
			continue
		}

		d.settings.WebDAVShares = append(d.settings.WebDAVShares[:i:i], d.settings.WebDAVShares[i+1:]...)
		if err := d.store.Settings.Save(d.settings); err != nil {
			return http.StatusInternalServerError, err
		}
		davShareVersion.Add(1)
		d.audit(r, "webdav_unshare", share.Path, "", share.Name)

		return http.StatusNoContent, nil
	}

	return http.StatusNotFound, nil
})
