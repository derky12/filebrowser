package fbhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/activity"
	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/users"
)

type activityEnv struct {
	st     *storage.Storage
	key    []byte
	scope  string
	server *settings.Server
	perm   users.Permissions
}

func newActivityEnv(t *testing.T, perm users.Permissions) *activityEnv {
	t.Helper()
	scope, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	return &activityEnv{
		st:     scopedUserStorage(t, scope, perm, key),
		key:    key,
		scope:  scope,
		server: &settings.Server{ActivityDays: 30},
		perm:   perm,
	}
}

func (e *activityEnv) call(t *testing.T, h handleFunc, prefix, method, target, body string, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, target, strings.NewReader(body))
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	req.Header.Set("X-Auth", signToken(t, e.perm, e.key))
	rec := httptest.NewRecorder()
	handle(h, prefix, e.st, e.server).ServeHTTP(rec, req)
	return rec
}

func (e *activityEnv) entries(t *testing.T) []*activity.Entry {
	t.Helper()
	items, _, err := e.st.Activity.List(activity.Filter{Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestActivityRecordsFileOperations(t *testing.T) {
	perm := users.Permissions{Create: true, Modify: true, Rename: true, Delete: true, Download: true}
	e := newActivityEnv(t, perm)

	steps := []struct {
		name    string
		handler handleFunc
		method  string
		target  string
		body    string
		want    string
		path    string
		dest    string
	}{
		{"mkdir", audited(auditPost, resourcePostHandler(diskcache.NewNoOp())), http.MethodPost, "/api/resources/docs/", "", "mkdir", "/docs/", ""},
		{"upload", audited(auditPost, resourcePostHandler(diskcache.NewNoOp())), http.MethodPost, "/api/resources/docs/a.txt", "hello", "upload", "/docs/a.txt", ""},
		{"save", audited(auditFixed("save"), resourcePutHandler), http.MethodPut, "/api/resources/docs/a.txt", "changed", "save", "/docs/a.txt", ""},
		{"copy", audited(auditPatch, resourcePatchHandler(diskcache.NewNoOp())), http.MethodPatch, "/api/resources/docs/a.txt?action=copy&destination=/docs/b.txt", "", "copy", "/docs/a.txt", "/docs/b.txt"},
		{"rename", audited(auditPatch, resourcePatchHandler(diskcache.NewNoOp())), http.MethodPatch, "/api/resources/docs/b.txt?action=rename&destination=/docs/c.txt", "", "rename", "/docs/b.txt", "/docs/c.txt"},
		{"move", audited(auditPatch, resourcePatchHandler(diskcache.NewNoOp())), http.MethodPatch, "/api/resources/docs/c.txt?action=rename&destination=/c.txt", "", "move", "/docs/c.txt", "/c.txt"},
	}
	for _, s := range steps {
		rec := e.call(t, s.handler, "/api/resources", s.method, s.target, s.body, nil)
		if rec.Code >= 300 {
			t.Fatalf("%s: status=%d body=%q", s.name, rec.Code, rec.Body.String())
		}
	}

	items := e.entries(t)
	if len(items) != len(steps) {
		t.Fatalf("got %d entries, want %d: %+v", len(items), len(steps), items)
	}
	// Newest first.
	for i, s := range steps {
		got := items[len(items)-1-i]
		if got.Action != s.want || got.Path != s.path || got.Dest != s.dest || got.Username != "u" || got.UserID != 1 {
			t.Errorf("%s: got %+v", s.name, got)
		}
	}
}

func TestActivityIgnoresFailedOperations(t *testing.T) {
	perm := users.Permissions{Create: true, Modify: true, Delete: true}
	e := newActivityEnv(t, perm)
	if err := os.WriteFile(filepath.Join(e.scope, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Conflict: the file exists and override is not set.
	rec := e.call(t, audited(auditPost, resourcePostHandler(diskcache.NewNoOp())), "/api/resources", http.MethodPost, "/api/resources/a.txt", "x", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", rec.Code)
	}
	if n := len(e.entries(t)); n != 0 {
		t.Fatalf("a failed operation was logged: %d entries", n)
	}
}

func TestActivityOffWhenRetentionIsZero(t *testing.T) {
	perm := users.Permissions{Create: true}
	e := newActivityEnv(t, perm)
	e.server.ActivityDays = 0

	e.call(t, audited(auditPost, resourcePostHandler(diskcache.NewNoOp())), "/api/resources", http.MethodPost, "/api/resources/d/", "", nil)
	if n := len(e.entries(t)); n != 0 {
		t.Fatalf("logging is off but %d entries were written", n)
	}
}

func TestActivityListIsAdminOnlyAndFilters(t *testing.T) {
	admin := users.Permissions{Admin: true, Create: true}
	e := newActivityEnv(t, admin)
	for _, a := range []activity.Entry{
		{Username: "u", Action: "delete", Path: "/docs/report.txt"},
		{Username: "u", Action: "upload", Path: "/docs/photo.jpg", Detail: "3.0 MiB"},
		{Username: "bob", Action: "delete", Path: "/other/Report.pdf"},
	} {
		entry := a
		if err := e.st.Activity.Add(&entry); err != nil {
			t.Fatal(err)
		}
	}

	query := func(q string) activityResponse {
		rec := e.call(t, activityGetHandler, "", http.MethodGet, "/api/activity?"+q, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%q", q, rec.Code, rec.Body.String())
		}
		var out activityResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}

	if got := query(""); got.Total != 3 || got.Items[0].Path != "/other/Report.pdf" {
		t.Fatalf("all: %+v", got)
	}
	if got := query("action=delete"); got.Total != 2 {
		t.Fatalf("action filter: total=%d", got.Total)
	}
	if got := query("user=bob"); got.Total != 1 || got.Items[0].Username != "bob" {
		t.Fatalf("user filter: %+v", got)
	}
	if got := query("q=report"); got.Total != 2 {
		t.Fatalf("case-insensitive search: total=%d, want 2", got.Total)
	}
	if got := query("q=3.0+MiB"); got.Total != 1 || got.Items[0].Action != "upload" {
		t.Fatalf("detail search: %+v", got)
	}
	if got := query("limit=1&offset=1"); len(got.Items) != 1 || got.Total != 3 || got.Items[0].Path != "/docs/photo.jpg" {
		t.Fatalf("paging: %+v", got)
	}

	nonAdmin := newActivityEnv(t, users.Permissions{Create: true})
	if rec := nonAdmin.call(t, activityGetHandler, "", http.MethodGet, "/api/activity", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: status=%d, want 403", rec.Code)
	}
}

func TestActivityPrune(t *testing.T) {
	e := newActivityEnv(t, users.Permissions{})
	old := time.Now().Add(-100 * 24 * time.Hour)
	for i, when := range []time.Time{old, old, time.Now()} {
		if err := e.st.Activity.Add(&activity.Entry{Time: when, Username: "u", Action: "save", Path: "/p" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}

	e.st.Activity.Retention = 30 * 24 * time.Hour
	if err := e.st.Activity.Prune(); err != nil {
		t.Fatal(err)
	}
	items := e.entries(t)
	if len(items) != 1 || items[0].Path != "/pc" {
		t.Fatalf("after prune: %+v", items)
	}
}

func TestFilesizeLabel(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 3 << 20: "3.0 MiB", 5 << 30: "5.0 GiB"} {
		if got := filesizeLabel(n); got != want {
			t.Errorf("%d: got %q, want %q", n, got, want)
		}
	}
}
