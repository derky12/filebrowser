package fbhttp

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/activity"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/users"
)

const (
	davTestUser     = "dav"
	davTestPassword = "correct horse battery"
)

type davEnv struct {
	t      *testing.T
	root   string
	st     *storage.Storage
	server *settings.Server
	key    []byte
	srv    *httptest.Server
	dav    *davServer
}

// newDavEnv starts a WebDAV server over a temporary root with these shares
// (name -> read-only); each share's folder is created under the root.
func newDavEnv(t *testing.T, shares map[string]bool, mutate ...func(*settings.Server)) *davEnv {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, root, users.Permissions{Admin: true, Create: true, Modify: true, Rename: true, Delete: true, Download: true}, key)

	cfg := &settings.Settings{Key: key}
	for name, readOnly := range shares {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		cfg.WebDAVShares = append(cfg.WebDAVShares, settings.WebDAVShare{Name: name, Dir: "/" + name, Path: "/" + name, ReadOnly: readOnly})
	}
	if err := st.Settings.Save(cfg); err != nil {
		t.Fatal(err)
	}
	davShareVersion.Add(1)

	server := &settings.Server{
		Root: root, WebDAV: true, WebDAVPort: "8090", WebDAVUser: davTestUser,
		WebDAVPassword: davTestPassword, ActivityDays: 30,
	}
	for _, m := range mutate {
		m(server)
	}

	dav := newDavServer(st, server)
	srv := httptest.NewServer(dav)
	t.Cleanup(srv.Close)

	return &davEnv{t: t, root: root, st: st, server: server, key: key, srv: srv, dav: dav}
}

// do sends a WebDAV request and returns the status and body.
func (e *davEnv) do(method, path, body string, headers map[string]string) (int, string) {
	e.t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	req.SetBasicAuth(davTestUser, davTestPassword)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func (e *davEnv) destination(path string) map[string]string {
	return map[string]string{"Destination": e.srv.URL + path}
}

func (e *davEnv) write(rel, content string) {
	e.t.Helper()
	p := filepath.Join(e.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *davEnv) read(rel string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(e.root, rel))
	return string(raw), err == nil
}

func TestWebDAVAuthentication(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": true})

	req, _ := http.NewRequest("PROPFIND", e.srv.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("no credentials: status=%d challenge=%q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	for name, creds := range map[string][2]string{"wrong password": {davTestUser, "nope"}, "wrong user": {"someone", davTestPassword}, "empty": {"", ""}} {
		req, _ := http.NewRequest("PROPFIND", e.srv.URL+"/", nil)
		req.SetBasicAuth(creds[0], creds[1])
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status=%d, want 401", name, resp.StatusCode)
		}
	}

	if status, _ := e.do("PROPFIND", "/", "", map[string]string{"Depth": "0"}); status != http.StatusMultiStatus {
		t.Fatalf("right credentials: status=%d, want 207", status)
	}
}

func TestWebDAVRootListsTheShares(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": true, "photos": false})
	// A share whose folder is gone is not listed.
	e.server.Root = e.root
	cfg, _ := e.st.Settings.Get()
	cfg.WebDAVShares = append(cfg.WebDAVShares, settings.WebDAVShare{Name: "missing", Dir: "/missing"})
	_ = e.st.Settings.Save(cfg)
	davShareVersion.Add(1)

	status, body := e.do("PROPFIND", "/", "", map[string]string{"Depth": "1"})
	if status != http.StatusMultiStatus {
		t.Fatalf("status=%d", status)
	}
	for _, want := range []string{"/docs/", "/photos/"} {
		if !strings.Contains(body, want) {
			t.Errorf("share %s missing from the root listing: %s", want, body)
		}
	}
	if strings.Contains(body, "missing") {
		t.Error("an unusable share must not be listed")
	}
}

func TestWebDAVReadOnlyShare(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": true})
	e.write("docs/a.txt", "hello")

	if status, body := e.do("GET", "/docs/a.txt", "", nil); status != http.StatusOK || body != "hello" {
		t.Fatalf("read: status=%d body=%q", status, body)
	}
	if status, _ := e.do("PROPFIND", "/docs/", "", map[string]string{"Depth": "1"}); status != http.StatusMultiStatus {
		t.Fatalf("list: status=%d", status)
	}

	for name, c := range map[string]struct {
		method, path, body string
		headers            map[string]string
	}{
		"PUT":       {"PUT", "/docs/new.txt", "x", nil},
		"overwrite": {"PUT", "/docs/a.txt", "changed", nil},
		"DELETE":    {"DELETE", "/docs/a.txt", "", nil},
		"MKCOL":     {"MKCOL", "/docs/sub", "", nil},
		"MOVE":      {"MOVE", "/docs/a.txt", "", e.destination("/docs/b.txt")},
		"COPY":      {"COPY", "/docs/a.txt", "", e.destination("/docs/b.txt")},
		"PROPPATCH": {"PROPPATCH", "/docs/a.txt", `<?xml version="1.0"?><propertyupdate xmlns="DAV:"><set><prop><x xmlns="urn:x">1</x></prop></set></propertyupdate>`, nil},
		"LOCK":      {"LOCK", "/docs/a.txt", `<?xml version="1.0"?><lockinfo xmlns="DAV:"><lockscope><exclusive/></lockscope><locktype><write/></locktype></lockinfo>`, nil},
	} {
		if status, _ := e.do(c.method, c.path, c.body, c.headers); status != http.StatusForbidden {
			t.Errorf("%s on a read-only share: status=%d, want 403", name, status)
		}
	}

	if got, _ := e.read("docs/a.txt"); got != "hello" {
		t.Fatalf("the file was changed: %q", got)
	}
	for _, rel := range []string{"docs/new.txt", "docs/b.txt", "docs/sub"} {
		if _, err := os.Stat(filepath.Join(e.root, rel)); err == nil {
			t.Errorf("%s was created", rel)
		}
	}
}

func TestWebDAVReadWriteShare(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"work": false})

	if status, _ := e.do("PUT", "/work/a.txt", "hello", nil); status != http.StatusCreated {
		t.Fatalf("PUT: status=%d", status)
	}
	if got, ok := e.read("work/a.txt"); !ok || got != "hello" {
		t.Fatalf("on disk: %q %v", got, ok)
	}
	if status, _ := e.do("MKCOL", "/work/sub", "", nil); status != http.StatusCreated {
		t.Fatalf("MKCOL: status=%d", status)
	}
	if status, _ := e.do("MOVE", "/work/a.txt", "", e.destination("/work/sub/moved.txt")); status != http.StatusCreated {
		t.Fatalf("MOVE: status=%d", status)
	}
	if got, ok := e.read("work/sub/moved.txt"); !ok || got != "hello" {
		t.Fatalf("after MOVE: %q %v", got, ok)
	}
	if status, _ := e.do("COPY", "/work/sub/moved.txt", "", e.destination("/work/copy.txt")); status != http.StatusCreated {
		t.Fatalf("COPY: status=%d", status)
	}
	if got, _ := e.read("work/copy.txt"); got != "hello" {
		t.Fatalf("after COPY: %q", got)
	}
	if status, _ := e.do("DELETE", "/work/sub", "", nil); status != http.StatusNoContent {
		t.Fatalf("DELETE: status=%d", status)
	}
	if _, ok := e.read("work/sub/moved.txt"); ok {
		t.Fatal("the folder was not deleted")
	}

	big := strings.Repeat("0123456789", 600000) // 6 MB
	if status, _ := e.do("PUT", "/work/big.bin", big, nil); status != http.StatusCreated {
		t.Fatalf("big PUT: status=%d", status)
	}
	if status, body := e.do("GET", "/work/big.bin", "", nil); status != http.StatusOK || body != big {
		t.Fatalf("big GET: status=%d, %d bytes", status, len(body))
	}
}

func TestWebDAVRootAndShareFoldersAreProtected(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"work": false, "other": false})
	e.write("work/a.txt", "x")

	for name, c := range map[string]struct {
		method, path string
		headers      map[string]string
	}{
		"new top level folder": {"MKCOL", "/newshare", nil},
		"file at the root":     {"PUT", "/loose.txt", nil},
		"delete a share":       {"DELETE", "/work", nil},
		"delete the root":      {"DELETE", "/", nil},
		"rename a share":       {"MOVE", "/work", e.destination("/renamed")},
		"move out of a share":  {"MOVE", "/work/a.txt", e.destination("/a.txt")},
		"between shares":       {"MOVE", "/work/a.txt", e.destination("/other/a.txt")},
		"copy between shares":  {"COPY", "/work/a.txt", e.destination("/other/a.txt")},
	} {
		if status, _ := e.do(c.method, c.path, "x", c.headers); status != http.StatusForbidden {
			t.Errorf("%s: status=%d, want 403", name, status)
		}
	}
	if _, ok := e.read("other/a.txt"); ok {
		t.Fatal("a file crossed from one share to another")
	}
	if _, err := os.Stat(filepath.Join(e.root, "work")); err != nil {
		t.Fatal("a share's folder was touched")
	}
}

func TestWebDAVCannotLeaveAShare(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": false})
	e.write("secret.txt", "TOP SECRET") // in the root, not in any share
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("OUTSIDE"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.write("docs/inside.txt", "inside")
	if err := os.Symlink(outside, filepath.Join(e.root, "docs", "link-out")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join(e.root, "docs", "inside.txt"), filepath.Join(e.root, "docs", "link-in")); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/docs/../secret.txt", "/docs/%2e%2e/secret.txt", "/../secret.txt", "/docs/../../etc/passwd", "/secret.txt", "/docs/link-out/outside.txt"} {
		status, body := e.do("GET", path, "", nil)
		if status == http.StatusOK || strings.Contains(body, "TOP SECRET") || strings.Contains(body, "OUTSIDE") {
			t.Errorf("GET %s: status=%d body=%q leaked something", path, status, body)
		}
	}
	// A write through the symlink must not land outside either.
	e.do("PUT", "/docs/link-out/evil.txt", "evil", nil)
	if _, err := os.Stat(filepath.Join(outside, "evil.txt")); err == nil {
		t.Fatal("a write went through a symlink to outside the share")
	}
	// A link that stays inside the share still works.
	if status, body := e.do("GET", "/docs/link-in", "", nil); status != http.StatusOK || body != "inside" {
		t.Errorf("a link inside the share: status=%d body=%q", status, body)
	}
}

func TestWebDAVShareReplacedBySymlinkStopsWorking(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": false})
	e.write("docs/a.txt", "x")
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.WriteFile(filepath.Join(outside, "o.txt"), []byte("OUTSIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(e.root, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(e.root, "docs")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	davShareVersion.Add(1)

	if status, body := e.do("GET", "/docs/o.txt", "", nil); status == http.StatusOK || strings.Contains(body, "OUTSIDE") {
		t.Fatalf("a share that became a symlink still serves: status=%d body=%q", status, body)
	}
}

func TestWebDAVPrefix(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"work": false}, func(s *settings.Server) { s.WebDAVPrefix = "/dav" })

	if status, _ := e.do("PUT", "/dav/work/a.txt", "hello", nil); status != http.StatusCreated {
		t.Fatalf("under the prefix: status=%d", status)
	}
	if got, ok := e.read("work/a.txt"); !ok || got != "hello" {
		t.Fatalf("on disk: %q %v", got, ok)
	}
	if status, _ := e.do("PROPFIND", "/dav", "", map[string]string{"Depth": "0"}); status != http.StatusMultiStatus {
		t.Fatalf("the prefix itself: status=%d", status)
	}
	if status, _ := e.do("MOVE", "/dav/work/a.txt", "", e.destination("/dav/work/b.txt")); status != http.StatusCreated {
		t.Fatalf("MOVE under the prefix: status=%d", status)
	}
	if status, _ := e.do("GET", "/work/b.txt", "", nil); status != http.StatusNotFound {
		t.Fatalf("outside the prefix: status=%d, want 404", status)
	}
}

func TestWebDAVDestinationHostMustMatch(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"work": false})
	e.write("work/a.txt", "x")

	// This is what a reverse proxy that rewrites the Host header causes.
	status, _ := e.do("MOVE", "/work/a.txt", "", map[string]string{"Destination": "https://dav.example.com/work/b.txt"})
	if status != http.StatusBadGateway {
		t.Fatalf("a Destination on another host: status=%d, want 502", status)
	}
	// With the Host the client used, it works.
	status, _ = e.do("MOVE", "/work/a.txt", "", map[string]string{
		"Host": "dav.example.com", "Destination": "https://dav.example.com/work/b.txt",
	})
	_ = status // net/http ignores a Host header set through Header; the Host field is what counts
	req, _ := http.NewRequest("MOVE", e.srv.URL+"/work/a.txt", nil)
	req.Host = "dav.example.com"
	req.Header.Set("Destination", "https://dav.example.com/work/b.txt")
	req.SetBasicAuth(davTestUser, davTestPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("matching Host and Destination: status=%d, want 201", resp.StatusCode)
	}
}

func TestWebDAVLimitsWrongPasswords(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": true})
	now := time.Now()
	e.dav.limiter.now = func() time.Time { return now }

	attempt := func(password string, headers map[string]string) int {
		req, _ := http.NewRequest("PROPFIND", e.srv.URL+"/", nil)
		req.SetBasicAuth(davTestUser, password)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	for i := 0; i < 5; i++ {
		if status := attempt("wrong", nil); status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status=%d, want 401", i, status)
		}
	}
	// Blocked now, even with the right password.
	if status := attempt(davTestPassword, nil); status != http.StatusTooManyRequests {
		t.Fatalf("after 5 failures: status=%d, want 429", status)
	}
	// A client cannot dodge it by claiming another address.
	if status := attempt(davTestPassword, map[string]string{"X-Forwarded-For": "203.0.113.9"}); status != http.StatusTooManyRequests {
		t.Fatalf("a spoofed X-Forwarded-For without a proxy: status=%d, want 429", status)
	}

	now = now.Add(6 * time.Minute)
	if status := attempt(davTestPassword, nil); status != http.StatusMultiStatus {
		t.Fatalf("after the block ended: status=%d, want 207", status)
	}
}

func TestWebDAVLimitsPerClientBehindAProxy(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"docs": true}, func(s *settings.Server) { s.WebDAVBehindProxy = true })

	attempt := func(password, forwarded string) int {
		req, _ := http.NewRequest("PROPFIND", e.srv.URL+"/", nil)
		req.SetBasicAuth(davTestUser, password)
		req.Header.Set("X-Forwarded-For", forwarded)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// The proxy appends the real client address last; what the client put in
	// front of it is ignored.
	for i := 0; i < 5; i++ {
		attempt("wrong", "1.2.3.4, 198.51.100.7")
	}
	if status := attempt(davTestPassword, "9.9.9.9, 198.51.100.7"); status != http.StatusTooManyRequests {
		t.Fatalf("the same client with another claimed address: status=%d, want 429", status)
	}
	if status := attempt(davTestPassword, "198.51.100.8"); status != http.StatusMultiStatus {
		t.Fatalf("another client behind the proxy: status=%d, want 207", status)
	}
}

func TestDavClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:5555"
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.5")

	if got := davClientIP(req, false); got != "10.0.0.2" {
		t.Errorf("without a proxy: %q", got)
	}
	if got := davClientIP(req, true); got != "203.0.113.5" {
		t.Errorf("behind a proxy: %q", got)
	}
	req.Header.Set("X-Forwarded-For", "not an address")
	if got := davClientIP(req, true); got != "10.0.0.2" {
		t.Errorf("garbage in X-Forwarded-For: %q", got)
	}
}

func TestWebDAVChangesAreLogged(t *testing.T) {
	e := newDavEnv(t, map[string]bool{"work": false})
	e.do("PUT", "/work/a.txt", "hello", nil)
	e.do("MKCOL", "/work/sub", "", nil)
	e.do("COPY", "/work/a.txt", "", e.destination("/work/b.txt"))
	e.do("MOVE", "/work/b.txt", "", e.destination("/work/sub/c.txt"))
	e.do("DELETE", "/work/a.txt", "", nil)
	e.do("GET", "/work/sub/c.txt", "", nil)
	e.do("PROPFIND", "/work/", "", map[string]string{"Depth": "1"})

	items, _, err := e.st.Activity.List(activity.Filter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		got = append(got, it.Action+" "+it.Path+" "+it.Dest+" by "+it.Username)
	}
	want := []string{
		"webdav_put /work/a.txt  by dav",
		"webdav_mkdir /work/sub  by dav",
		"webdav_copy /work/a.txt /work/b.txt by dav",
		"webdav_move /work/b.txt /work/sub/c.txt by dav",
		"webdav_delete /work/a.txt  by dav",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("activity:\n got %q\nwant %q", got, want)
	}
	if items[len(items)-1].IP == "" {
		t.Error("the client address is not recorded")
	}
}

// ---- the management API

type davAPI struct {
	t      *testing.T
	st     *storage.Storage
	server *settings.Server
	key    []byte
	scope  string
	root   string
}

func newDavAPI(t *testing.T, perm users.Permissions, mutate ...func(*settings.Server)) *davAPI {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The user's scope is a folder below the server root, as with a per-user home.
	scope := filepath.Join(root, "users", "u")
	if err := os.MkdirAll(scope, 0o755); err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	server := &settings.Server{Root: root, WebDAV: true, WebDAVPort: "8090", WebDAVHostPort: "38004", WebDAVUser: davTestUser, WebDAVPassword: davTestPassword}
	for _, m := range mutate {
		m(server)
	}

	return &davAPI{t: t, st: scopedUserStorage(t, scope, perm, key), server: server, key: key, scope: scope, root: root}
}

func (a *davAPI) call(h handleFunc, perm users.Permissions, method, target, body string, vars map[string]string) *httptest.ResponseRecorder {
	a.t.Helper()
	req, _ := http.NewRequest(method, target, strings.NewReader(body))
	req.Host = "100.64.0.7:38002"
	if vars != nil {
		req = muxVars(req, vars)
	}
	req.Header.Set("X-Auth", signToken(a.t, perm, a.key))
	rec := httptest.NewRecorder()
	handle(h, "", a.st, a.server).ServeHTTP(rec, req)
	return rec
}

var davAdminPerm = users.Permissions{Admin: true, Create: true, Modify: true, Download: true}

func TestWebDAVShareManagement(t *testing.T) {
	a := newDavAPI(t, davAdminPerm)
	if err := os.MkdirAll(filepath.Join(a.scope, "Photos 2024"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Created read-only, named after the folder, with the URL clients use.
	rec := a.call(webdavShareCreateHandler, davAdminPerm, http.MethodPost, "/api/webdav/shares", `{"path":"/Photos 2024"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: status=%d body=%q", rec.Code, rec.Body.String())
	}
	var created webdavShareInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Name != "Photos 2024" || !created.ReadOnly || !created.Available || created.Path != "/Photos 2024" ||
		created.URL != "http://100.64.0.7:38004/Photos%202024/" {
		t.Fatalf("created: %+v", created)
	}

	cfg, _ := a.st.Settings.Get()
	if len(cfg.WebDAVShares) != 1 || cfg.WebDAVShares[0].Dir != "/users/u/Photos 2024" {
		t.Fatalf("stored share (relative to the server root, not the user's scope): %+v", cfg.WebDAVShares)
	}

	// Make it writable, list, then remove.
	rec = a.call(webdavSharePatchHandler, davAdminPerm, http.MethodPatch, "/x", `{"readOnly":false}`, map[string]string{"name": "photos 2024"})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	rec = a.call(webdavGetHandler, davAdminPerm, http.MethodGet, "/api/webdav", "", nil)
	var info webdavInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if !info.Enabled || info.User != davTestUser || info.BaseURL != "http://100.64.0.7:38004" || len(info.Shares) != 1 || info.Shares[0].ReadOnly {
		t.Fatalf("info: %+v", info)
	}
	// A folder replaced by a symlink is reported as gone, like the server treats it.
	real := filepath.Join(a.scope, "Photos 2024")
	if err := os.RemoveAll(real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a.root, real); err == nil {
		rec = a.call(webdavGetHandler, davAdminPerm, http.MethodGet, "/api/webdav", "", nil)
		_ = json.Unmarshal(rec.Body.Bytes(), &info)
		if len(info.Shares) != 1 || info.Shares[0].Available {
			t.Fatalf("a share behind a symlink must not be available: %+v", info.Shares)
		}
		_ = os.Remove(real)
	}
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}

	rec = a.call(webdavShareDeleteHandler, davAdminPerm, http.MethodDelete, "/x", "", map[string]string{"name": "Photos 2024"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status=%d", rec.Code)
	}
	if cfg, _ := a.st.Settings.Get(); len(cfg.WebDAVShares) != 0 {
		t.Fatalf("the share is still stored: %+v", cfg.WebDAVShares)
	}
	if _, err := os.Stat(filepath.Join(a.scope, "Photos 2024")); err != nil {
		t.Fatal("removing a share must not touch the folder")
	}
}

func TestWebDAVShareURLs(t *testing.T) {
	a := newDavAPI(t, davAdminPerm, func(s *settings.Server) {
		s.WebDAVPublicURL = "https://dav.example.com/files"
		s.WebDAVPrefix = "/files"
	})
	if err := os.MkdirAll(filepath.Join(a.scope, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := a.call(webdavShareCreateHandler, davAdminPerm, http.MethodPost, "/api/webdav/shares", `{"path":"/docs","name":"Docs","readOnly":false}`, nil)
	var created webdavShareInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.URL != "https://dav.example.com/files/Docs/" || created.ReadOnly {
		t.Fatalf("with a public URL: %+v", created)
	}

	b := newDavAPI(t, davAdminPerm, func(s *settings.Server) { s.WebDAVPrefix = "/dav"; s.WebDAVHostPort = "" })
	if got := davBaseURL(httptest.NewRequest(http.MethodGet, "http://[fd7a::1]:38002/", nil), b.server); got != "http://[fd7a::1]:8090/dav" {
		t.Fatalf("without a public URL and host port: %q", got)
	}
}

func TestWebDAVShareRejections(t *testing.T) {
	a := newDavAPI(t, davAdminPerm)
	for _, d := range []string{"docs", "real"} {
		if err := os.MkdirAll(filepath.Join(a.scope, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a.scope, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(a.scope, "real"), filepath.Join(a.scope, "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if rec := a.call(webdavShareCreateHandler, davAdminPerm, http.MethodPost, "/", `{"path":"/docs","name":"docs"}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("first share: status=%d", rec.Code)
	}

	for name, body := range map[string]string{
		"scope root":      `{"path":"/","name":"x"}`,
		"a file":          `{"path":"/file.txt","name":"x"}`,
		"a symlink":       `{"path":"/link","name":"x"}`,
		"missing":         `{"path":"/nope","name":"x"}`,
		"duplicate":       `{"path":"/real","name":"DOCS"}`,
		"slash in name":   `{"path":"/real","name":"a/b"}`,
		"dots":            `{"path":"/real","name":".."}`,
		"leading space":   `{"path":"/real","name":" -x"}`,
		"too long a name": `{"path":"/real","name":"` + strings.Repeat("a", 80) + `"}`,
		"invalid json":    `{`,
	} {
		rec := a.call(webdavShareCreateHandler, davAdminPerm, http.MethodPost, "/", body, nil)
		if rec.Code < 400 {
			t.Errorf("%s: status=%d, want an error", name, rec.Code)
		}
	}
	if cfg, _ := a.st.Settings.Get(); len(cfg.WebDAVShares) != 1 {
		t.Fatalf("a rejected request created a share: %+v", cfg.WebDAVShares)
	}
}

func TestWebDAVManagementNeedsAdminAndTheServer(t *testing.T) {
	a := newDavAPI(t, users.Permissions{Create: true})
	if rec := a.call(webdavGetHandler, users.Permissions{Create: true}, http.MethodGet, "/", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: status=%d, want 403", rec.Code)
	}

	off := newDavAPI(t, davAdminPerm, func(s *settings.Server) { s.WebDAV = false })
	for _, h := range []handleFunc{webdavGetHandler, webdavShareCreateHandler} {
		if rec := off.call(h, davAdminPerm, http.MethodGet, "/", `{}`, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("WebDAV off: status=%d, want 404", rec.Code)
		}
	}
}

func TestWebDAVNewShareIsServedAtOnce(t *testing.T) {
	e := newDavEnv(t, nil)
	e.write("docs/a.txt", "hello")
	if status, _ := e.do("GET", "/docs/a.txt", "", nil); status != http.StatusNotFound {
		t.Fatalf("before sharing: status=%d, want 404", status)
	}

	cfg, _ := e.st.Settings.Get()
	cfg.WebDAVShares = append(cfg.WebDAVShares, settings.WebDAVShare{Name: "docs", Dir: "/docs", ReadOnly: true})
	_ = e.st.Settings.Save(cfg)
	davShareVersion.Add(1) // what the management API does

	if status, body := e.do("GET", "/docs/a.txt", "", nil); status != http.StatusOK || body != "hello" {
		t.Fatalf("after sharing: status=%d body=%q", status, body)
	}

	cfg.WebDAVShares = nil
	_ = e.st.Settings.Save(cfg)
	davShareVersion.Add(1)
	if status, _ := e.do("GET", "/docs/a.txt", "", nil); status != http.StatusNotFound {
		t.Fatalf("after unsharing: status=%d, want 404", status)
	}
}

func muxVars(req *http.Request, vars map[string]string) *http.Request {
	return mux.SetURLVars(req, vars)
}
