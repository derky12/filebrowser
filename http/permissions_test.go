package fbhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

type permEnv struct {
	scope string
}

func newPermEnv(t *testing.T, perm users.Permissions, denied ...string) (*permEnv, func(h handleFunc, method, target, body string) *httptest.ResponseRecorder) {
	t.Helper()
	scope, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, scope, perm, key)
	if len(denied) > 0 {
		set := &settings.Settings{Key: key}
		for _, p := range denied {
			set.Rules = append(set.Rules, rules.Rule{Path: p, Allow: false})
		}
		if err := st.Settings.Save(set); err != nil {
			t.Fatal(err)
		}
	}

	call := func(h handleFunc, method, target, body string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("X-Auth", signToken(t, perm, key))
		rec := httptest.NewRecorder()
		handle(h, "/api/permissions", st, &settings.Server{}).ServeHTTP(rec, req)
		return rec
	}

	return &permEnv{scope: scope}, call
}

func writePermFile(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func permModeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

var modifyPerm = users.Permissions{Modify: true, Create: true, Rename: true, Download: true}

func TestPermissionsGet(t *testing.T) {
	env, call := newPermEnv(t, modifyPerm)
	writePermFile(t, filepath.Join(env.scope, "a.txt"), 0o640)
	if err := os.Mkdir(filepath.Join(env.scope, "dir"), 0o750); err != nil {
		t.Fatal(err)
	}

	rec := call(permissionsGetHandler, http.MethodGet, "/api/permissions/a.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var got permissionsInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Mode != "0640" || got.IsDir || got.Path != "/a.txt" || got.CanChown {
		t.Fatalf("unexpected info %+v", got)
	}

	rec = call(permissionsGetHandler, http.MethodGet, "/api/permissions/dir", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Mode != "0750" || !got.IsDir {
		t.Fatalf("unexpected dir info %+v", got)
	}
}

func TestPermissionsChmodFile(t *testing.T) {
	env, call := newPermEnv(t, modifyPerm)
	file := filepath.Join(env.scope, "a.txt")
	writePermFile(t, file, 0o644)

	rec := call(permissionsPatchHandler, http.MethodPatch, "/api/permissions/a.txt", `{"mode":"0600"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := permModeOf(t, file); got != 0o600 {
		t.Fatalf("mode=%o, want 600", got)
	}
}

func TestPermissionsChmodRecursive(t *testing.T) {
	env, call := newPermEnv(t, modifyPerm, "/tree/hidden")
	writePermFile(t, filepath.Join(env.scope, "tree", "f.txt"), 0o600)
	writePermFile(t, filepath.Join(env.scope, "tree", "sub", "g.txt"), 0o600)
	writePermFile(t, filepath.Join(env.scope, "tree", "hidden", "h.txt"), 0o600)

	outside := filepath.Join(env.scope, "outside.txt")
	writePermFile(t, outside, 0o600)
	if err := os.Symlink(outside, filepath.Join(env.scope, "tree", "link.txt")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	body := `{"recursive":true,"mode":"0750","dirMode":"0755","fileMode":"0644"}`
	rec := call(permissionsPatchHandler, http.MethodPatch, "/api/permissions/tree", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}

	var res permissionsResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	// tree, f.txt, sub, g.txt changed; link.txt and hidden skipped.
	if res.Changed != 4 || res.Skipped != 2 || res.Failed != 0 {
		t.Fatalf("result %+v", res)
	}

	want := map[string]os.FileMode{
		"tree":              0o750,
		"tree/f.txt":        0o644,
		"tree/sub":          0o755,
		"tree/sub/g.txt":    0o644,
		"tree/hidden/h.txt": 0o600, // hidden by a rule: untouched
		"outside.txt":       0o600, // symlink target: untouched
	}
	for rel, mode := range want {
		if got := permModeOf(t, filepath.Join(env.scope, rel)); got != mode {
			t.Errorf("%s: mode=%o, want %o", rel, got, mode)
		}
	}
}

func TestPermissionsRejections(t *testing.T) {
	env, call := newPermEnv(t, modifyPerm, "/secret")
	writePermFile(t, filepath.Join(env.scope, "a.txt"), 0o644)
	writePermFile(t, filepath.Join(env.scope, "secret", "s.txt"), 0o644)
	writePermFile(t, filepath.Join(env.scope, "real", "r.txt"), 0o644)
	if err := os.Symlink(filepath.Join(env.scope, "real"), filepath.Join(env.scope, "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	cases := map[string]struct {
		path string
		body string
		want int
	}{
		"scope root":              {"/", `{"mode":"0777"}`, http.StatusBadRequest},
		"symlink itself":          {"/link", `{"mode":"0777"}`, http.StatusBadRequest},
		"through a symlink":       {"/link/r.txt", `{"mode":"0777"}`, http.StatusBadRequest},
		"setuid bit":              {"/a.txt", `{"mode":"4755"}`, http.StatusBadRequest},
		"sticky bit":              {"/a.txt", `{"mode":"1777"}`, http.StatusBadRequest},
		"not octal":               {"/a.txt", `{"mode":"rwx"}`, http.StatusBadRequest},
		"empty request":           {"/a.txt", `{}`, http.StatusBadRequest},
		"recursive on a file":     {"/a.txt", `{"mode":"0644","recursive":true}`, http.StatusBadRequest},
		"file mode not recursive": {"/a.txt", `{"fileMode":"0644"}`, http.StatusBadRequest},
		"rule denied":             {"/secret", `{"mode":"0777"}`, http.StatusForbidden},
		"missing":                 {"/nope", `{"mode":"0600"}`, http.StatusNotFound},
		"chown as non-admin":      {"/a.txt", `{"owner":"0"}`, http.StatusForbidden},
		"invalid json":            {"/a.txt", `{`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := call(permissionsPatchHandler, http.MethodPatch, "/api/permissions"+tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status=%d, want %d (%q)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	for rel, mode := range map[string]os.FileMode{"a.txt": 0o644, "secret/s.txt": 0o644, "real/r.txt": 0o644} {
		if got := permModeOf(t, filepath.Join(env.scope, rel)); got != mode {
			t.Errorf("%s was changed to %o despite the rejection", rel, got)
		}
	}
}

func TestPermissionsNeedModifyPermission(t *testing.T) {
	env, call := newPermEnv(t, users.Permissions{Download: true})
	writePermFile(t, filepath.Join(env.scope, "a.txt"), 0o644)

	rec := call(permissionsPatchHandler, http.MethodPatch, "/api/permissions/a.txt", `{"mode":"0600"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
	if got := permModeOf(t, filepath.Join(env.scope, "a.txt")); got != 0o644 {
		t.Fatalf("mode changed to %o", got)
	}
}

func TestPermissionsChownByAdmin(t *testing.T) {
	if !canChownProcess() {
		t.Skip("changing owners needs a root process")
	}

	admin := modifyPerm
	admin.Admin = true
	env, call := newPermEnv(t, admin)
	file := filepath.Join(env.scope, "tree", "a.txt")
	writePermFile(t, file, 0o644)

	rec := call(permissionsGetHandler, http.MethodGet, "/api/permissions/tree/a.txt", "")
	var info permissionsInfo
	_ = json.Unmarshal(rec.Body.Bytes(), &info)
	if !info.CanChown {
		t.Fatal("an admin running as root must be able to chown")
	}

	rec = call(permissionsPatchHandler, http.MethodPatch, "/api/permissions/tree", `{"recursive":true,"owner":"12345","group":"23456"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	for _, p := range []string{filepath.Join(env.scope, "tree"), file} {
		stat, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		uid, gid, _ := fileOwner(stat)
		if uid != 12345 || gid != 23456 {
			t.Fatalf("%s owned by %d:%d, want 12345:23456", p, uid, gid)
		}
	}

	// Unknown names are refused with a message, nothing is changed.
	rec = call(permissionsPatchHandler, http.MethodPatch, "/api/permissions/tree/a.txt", `{"owner":"no-such-user-xyz"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown user: status=%d, want 400", rec.Code)
	}
}

func TestParsePermissionMode(t *testing.T) {
	for in, want := range map[string]os.FileMode{"644": 0o644, "0755": 0o755, "0": 0, " 600 ": 0o600, "777": 0o777} {
		got, err := parsePermissionMode(in)
		if err != nil || got != want {
			t.Errorf("%q: got %o, %v; want %o", in, got, err, want)
		}
	}
	for _, in := range []string{"", "8", "1000", "4755", "-1", "0o644", "rwx"} {
		if _, err := parsePermissionMode(in); err == nil {
			t.Errorf("%q: expected an error", in)
		}
	}
}
