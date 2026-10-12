package fbhttp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	gopath "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
	"golang.org/x/net/webdav"

	"github.com/filebrowser/filebrowser/v2/activity"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
)

// The WebDAV server shares folders chosen in the web interface. Clients log in
// with one user name and password from the environment and see the shares as
// the top level folders. Shares are read-only unless made writable.

// davShareVersion is bumped whenever the shares change, so that the server
// picks the change up at once instead of when its snapshot expires.
var davShareVersion atomic.Int64

const davSnapshotTTL = 5 * time.Second

type davShare struct {
	settings.WebDAVShare
	fs afero.Fs // confined to the folder; nil when the folder is not usable
}

type davSnapshot struct {
	version int64
	at      time.Time
	byName  map[string]*davShare // keyed by the lower-case name
	list    []*davShare
}

// davFS is the webdav.FileSystem over all shares.
type davFS struct {
	store  *storage.Storage
	server *settings.Server

	mu   sync.Mutex
	snap *davSnapshot
}

func (f *davFS) snapshot() *davSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()

	version := davShareVersion.Load()
	if f.snap != nil && f.snap.version == version && time.Since(f.snap.at) < davSnapshotTTL {
		return f.snap
	}

	snap := &davSnapshot{version: version, at: time.Now(), byName: map[string]*davShare{}}
	cfg, err := f.store.Settings.Get()
	if err != nil {
		log.Printf("webdav: could not read the shares: %v", err)
		f.snap = snap
		return snap
	}

	rootReal, rootErr := filepath.EvalSymlinks(f.server.Root)
	for _, share := range cfg.WebDAVShares {
		entry := &davShare{WebDAVShare: share}
		if rootErr == nil {
			base := filepath.Join(rootReal, filepath.FromSlash(share.Dir))
			real, err := filepath.EvalSymlinks(base)
			// The folder must still be a real folder inside the server root.
			if err == nil && real == base {
				if info, statErr := os.Stat(real); statErr == nil && info.IsDir() {
					entry.fs = files.NewScopedFs(afero.NewOsFs(), real)
				}
			}
		}
		snap.byName[strings.ToLower(share.Name)] = entry
		snap.list = append(snap.list, entry)
	}
	sort.Slice(snap.list, func(i, j int) bool { return snap.list[i].Name < snap.list[j].Name })

	f.snap = snap
	return snap
}

// davSplit splits a WebDAV path into the share name and the path inside it.
func davSplit(name string) (share, rest string) {
	name = gopath.Clean("/" + name)
	if name == "/" {
		return "", "/"
	}

	trimmed := strings.TrimPrefix(name, "/")
	first, remainder, found := strings.Cut(trimmed, "/")
	if !found {
		return first, "/"
	}

	return first, "/" + remainder
}

// target finds the share a path belongs to. Anything that would change the
// shares themselves (the root, or a share's own folder) is refused for writes.
func (f *davFS) target(name string, write bool) (*davShare, string, error) {
	shareName, rest := davSplit(name)
	if shareName == "" {
		if write {
			return nil, "", os.ErrPermission
		}
		return nil, "/", nil
	}

	share, ok := f.snapshot().byName[strings.ToLower(shareName)]
	if !ok || share.fs == nil {
		return nil, "", os.ErrNotExist
	}
	if write && (share.ReadOnly || rest == "/") {
		return nil, "", os.ErrPermission
	}

	return share, rest, nil
}

type davInfo struct {
	name    string
	modTime time.Time
}

func (i davInfo) Name() string       { return i.name }
func (i davInfo) Size() int64        { return 0 }
func (i davInfo) Mode() os.FileMode  { return os.ModeDir | 0o555 }
func (i davInfo) ModTime() time.Time { return i.modTime }
func (i davInfo) IsDir() bool        { return true }
func (i davInfo) Sys() interface{}   { return nil }

// renamedInfo shows a share's folder under the name of the share.
type renamedInfo struct {
	os.FileInfo
	name string
}

func (i renamedInfo) Name() string { return i.name }

// davRoot is the folder that lists the shares.
type davRoot struct {
	infos []os.FileInfo
	pos   int
}

func (r *davRoot) Close() error                   { return nil }
func (r *davRoot) Read([]byte) (int, error)       { return 0, io.EOF }
func (r *davRoot) Seek(int64, int) (int64, error) { return 0, nil }
func (r *davRoot) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (r *davRoot) Stat() (os.FileInfo, error)     { return davInfo{name: "/", modTime: time.Now()}, nil }

func (r *davRoot) Readdir(count int) ([]os.FileInfo, error) {
	if r.pos >= len(r.infos) {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}

	end := len(r.infos)
	if count > 0 && r.pos+count < end {
		end = r.pos + count
	}
	out := r.infos[r.pos:end]
	r.pos = end

	return out, nil
}

func (f *davFS) Mkdir(_ context.Context, name string, perm os.FileMode) error {
	share, rest, err := f.target(name, true)
	if err != nil {
		return err
	}

	return share.fs.Mkdir(rest, perm)
}

func (f *davFS) OpenFile(_ context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	write := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
	share, rest, err := f.target(name, write)
	if err != nil {
		return nil, err
	}
	if share == nil { // the root
		snap := f.snapshot()
		infos := make([]os.FileInfo, 0, len(snap.list))
		for _, s := range snap.list {
			if s.fs != nil {
				infos = append(infos, davInfo{name: s.Name, modTime: time.Now()})
			}
		}
		return &davRoot{infos: infos}, nil
	}

	return share.fs.OpenFile(rest, flag, perm)
}

func (f *davFS) RemoveAll(_ context.Context, name string) error {
	share, rest, err := f.target(name, true)
	if err != nil {
		return err
	}

	return share.fs.RemoveAll(rest)
}

func (f *davFS) Rename(_ context.Context, oldName, newName string) error {
	from, fromRest, err := f.target(oldName, true)
	if err != nil {
		return err
	}
	to, toRest, err := f.target(newName, true)
	if err != nil {
		return err
	}
	if from != to {
		return os.ErrPermission // not between shares
	}

	return from.fs.Rename(fromRest, toRest)
}

func (f *davFS) Stat(_ context.Context, name string) (os.FileInfo, error) {
	share, rest, err := f.target(name, false)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return davInfo{name: "/", modTime: time.Now()}, nil
	}

	info, err := share.fs.Stat(rest)
	if err != nil {
		return nil, err
	}
	if rest == "/" {
		return renamedInfo{FileInfo: info, name: share.Name}, nil
	}

	return info, nil
}

// ---- authentication

func davCredentialsOK(user, password, wantUser, wantPassword string) bool {
	u, wu := sha256.Sum256([]byte(user)), sha256.Sum256([]byte(wantUser))
	p, wp := sha256.Sum256([]byte(password)), sha256.Sum256([]byte(wantPassword))

	return subtle.ConstantTimeCompare(u[:], wu[:])&subtle.ConstantTimeCompare(p[:], wp[:]) == 1
}

// authLimiter blocks an address for a while after repeated wrong passwords.
type authLimiter struct {
	mu       sync.Mutex
	entries  map[string]*limitEntry
	now      func() time.Time
	maxFails int
	window   time.Duration
	block    time.Duration
}

type limitEntry struct {
	fails        int
	windowStart  time.Time
	blockedUntil time.Time
}

func newAuthLimiter() *authLimiter {
	return &authLimiter{
		entries:  map[string]*limitEntry{},
		now:      time.Now,
		maxFails: 5,
		window:   5 * time.Minute,
		block:    5 * time.Minute,
	}
}

// blocked reports for how much longer an address stays blocked.
func (l *authLimiter) blocked(ip string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		return 0, false
	}
	if remaining := e.blockedUntil.Sub(l.now()); remaining > 0 {
		return remaining, true
	}

	return 0, false
}

func (l *authLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.entries) > 10000 {
		for k, e := range l.entries {
			if now.Sub(e.windowStart) > l.window && now.After(e.blockedUntil) {
				delete(l.entries, k)
			}
		}
	}

	e, ok := l.entries[ip]
	if !ok || now.Sub(e.windowStart) > l.window {
		e = &limitEntry{windowStart: now}
		l.entries[ip] = e
	}
	e.fails++
	if e.fails >= l.maxFails {
		e.blockedUntil = now.Add(l.block)
		e.fails = 0
		e.windowStart = now
	}
}

func (l *authLimiter) success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, ip)
}

// davClientIP is the address of the client. Behind a reverse proxy it is the
// last X-Forwarded-For entry, which the proxy added itself; the earlier ones
// come from the client and can be anything.
func davClientIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// ---- the server

type davServer struct {
	store   *storage.Storage
	server  *settings.Server
	fs      *davFS
	dav     *webdav.Handler
	limiter *authLimiter
}

// NewWebDAVServer returns the WebDAV server; the caller listens and serves.
func NewWebDAVServer(store *storage.Storage, server *settings.Server) *http.Server {
	return &http.Server{
		Handler:           newDavServer(store, server),
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}

func newDavServer(store *storage.Storage, server *settings.Server) *davServer {
	d := &davServer{
		store:   store,
		server:  server,
		fs:      &davFS{store: store, server: server},
		limiter: newAuthLimiter(),
	}
	d.dav = &webdav.Handler{
		Prefix:     server.WebDAVPrefix,
		FileSystem: d.fs,
		LockSystem: webdav.NewMemLS(),
		Logger:     d.logRequest,
	}

	return d
}

// stripPrefix returns the WebDAV path of a URL path, or false when the URL is
// not under the prefix.
func (d *davServer) stripPrefix(p string) (string, bool) {
	prefix := d.server.WebDAVPrefix
	if prefix == "" {
		return p, true
	}
	if p == prefix {
		return "/", true
	}
	if strings.HasPrefix(p, prefix+"/") {
		return strings.TrimPrefix(p, prefix), true
	}

	return "", false
}

// destinationPath is the WebDAV path of a COPY or MOVE destination.
func (d *davServer) destinationPath(r *http.Request) (string, bool) {
	raw := r.Header.Get("Destination")
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	return d.stripPrefix(u.Path)
}

// checkWrite refuses, with 403, the requests that would change something a
// client may not change: the root, a share's own folder, a read-only share, or
// the move of something to another share.
func (d *davServer) checkWrite(r *http.Request, p string) bool {
	targetOK := func(name string) (string, bool) {
		shareName, rest := davSplit(name)
		if shareName == "" || rest == "/" {
			return "", false
		}
		share, ok := d.fs.snapshot().byName[strings.ToLower(shareName)]
		if !ok {
			return "", true // not found: the handler says so
		}
		return strings.ToLower(shareName), !share.ReadOnly
	}

	switch r.Method {
	case "PUT", "DELETE", "MKCOL", "PROPPATCH", "LOCK", "UNLOCK":
		_, ok := targetOK(p)
		return ok
	case "MOVE", "COPY":
		src, ok := targetOK(p)
		if !ok {
			return false
		}
		destination, valid := d.destinationPath(r)
		if !valid {
			return true // the handler answers 400
		}
		dst, ok := targetOK(destination)
		return ok && src == dst
	}

	return true
}

func (d *davServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := davClientIP(r, d.server.WebDAVBehindProxy)

	if wait, blocked := d.limiter.blocked(ip); blocked {
		w.Header().Set("Retry-After", time.Now().Add(wait).Format(http.TimeFormat))
		http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
		return
	}

	user, password, ok := r.BasicAuth()
	if !ok || !davCredentialsOK(user, password, d.server.WebDAVUser, d.server.WebDAVPassword) {
		if ok {
			d.limiter.fail(ip)
			log.Printf("webdav: wrong credentials from %s", ip)
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="File Browser WebDAV", charset="UTF-8"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	d.limiter.success(ip)

	p, ok := d.stripPrefix(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !d.checkWrite(r, p) {
		http.Error(w, "read-only share, or not allowed here", http.StatusForbidden)
		return
	}

	d.dav.ServeHTTP(w, r)
}

// logRequest records the changes made through WebDAV in the activity log.
func (d *davServer) logRequest(r *http.Request, err error) {
	if err != nil || d.store.Activity == nil || d.server.ActivityDays <= 0 {
		return
	}

	p, _ := d.stripPrefix(r.URL.Path)
	entry := &activity.Entry{
		Time:     time.Now(),
		Username: d.server.WebDAVUser,
		Path:     gopath.Clean("/" + p),
		IP:       davClientIP(r, d.server.WebDAVBehindProxy),
	}
	switch r.Method {
	case "PUT":
		entry.Action = "webdav_put"
		if r.ContentLength >= 0 {
			entry.Detail = filesizeLabel(r.ContentLength)
		}
	case "DELETE":
		entry.Action = "webdav_delete"
	case "MKCOL":
		entry.Action = "webdav_mkdir"
	case "MOVE", "COPY":
		entry.Action = "webdav_" + strings.ToLower(r.Method)
		if destination, ok := d.destinationPath(r); ok {
			entry.Dest = gopath.Clean("/" + destination)
		}
	default:
		return
	}

	if addErr := d.store.Activity.Add(entry); addErr != nil {
		log.Printf("activity: could not record %s %s: %v", entry.Action, entry.Path, addErr)
	}
}
