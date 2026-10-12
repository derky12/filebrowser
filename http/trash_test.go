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

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/users"
)

type trashEnv struct {
	t     *testing.T
	scope string
	st    *storage.Storage
	key   []byte
	perm  users.Permissions
}

func newTrashEnv(t *testing.T, cfg *settings.Trash, perm users.Permissions) *trashEnv {
	t.Helper()
	scope, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, scope, perm, key)
	if cfg != nil {
		if err := st.Settings.Save(&settings.Settings{Key: key, Trash: cfg}); err != nil {
			t.Fatal(err)
		}
	}
	return &trashEnv{t: t, scope: scope, st: st, key: key, perm: perm}
}

var fullPerm = users.Permissions{Create: true, Modify: true, Rename: true, Delete: true, Download: true}

func (e *trashEnv) call(h handleFunc, prefix, method, target string, vars map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	req, _ := http.NewRequest(method, target, http.NoBody)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	req.Header.Set("X-Auth", signToken(e.t, e.perm, e.key))
	rec := httptest.NewRecorder()
	handle(h, prefix, e.st, &settings.Server{}).ServeHTTP(rec, req)
	return rec
}

func (e *trashEnv) del(p string, query string) *httptest.ResponseRecorder {
	return e.call(resourceDeleteHandler(diskcache.NewNoOp()), "/api/resources", http.MethodDelete, "/api/resources"+p+query, nil)
}

func (e *trashEnv) write(rel string, size int) {
	e.t.Helper()
	path := filepath.Join(e.scope, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *trashEnv) exists(rel string) bool {
	_, err := os.Lstat(filepath.Join(e.scope, rel))
	return err == nil
}

func (e *trashEnv) list() trashListing {
	e.t.Helper()
	rec := e.call(trashListHandler, "", http.MethodGet, "/api/trash", nil)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("list: status=%d body=%q", rec.Code, rec.Body.String())
	}
	var out trashListing
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func TestTrashIsOnByDefaultAndMovesFiles(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm) // no settings saved: defaults
	e.write("docs/a.txt", 5)

	rec := e.del("/docs/a.txt", "")
	if rec.Code != http.StatusNoContent || rec.Header().Get("X-Trash") != "trashed" {
		t.Fatalf("status=%d X-Trash=%q", rec.Code, rec.Header().Get("X-Trash"))
	}
	if e.exists("docs/a.txt") {
		t.Fatal("the file is still in place")
	}

	list := e.list()
	if len(list.Items) != 1 || list.TotalBytes != 5 || !list.Enabled {
		t.Fatalf("listing: %+v", list)
	}
	item := list.Items[0]
	if item.Path != "/docs/a.txt" || item.Name != "a.txt" || item.IsDir || item.Size != 5 || item.By != "u" {
		t.Fatalf("item: %+v", item)
	}
	if !e.exists(".trash/" + item.ID + "/data") {
		t.Fatal("the data is not in the trash")
	}
}

func TestTrashFolder(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	e.write("tree/a.txt", 3)
	e.write("tree/sub/b.txt", 4)

	if rec := e.del("/tree", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	list := e.list()
	if len(list.Items) != 1 || !list.Items[0].IsDir || list.Items[0].Size != 7 {
		t.Fatalf("listing: %+v", list)
	}
	if !e.exists(".trash/" + list.Items[0].ID + "/data/sub/b.txt") {
		t.Fatal("the folder was not moved whole")
	}
}

func TestTrashDisabledOrPermanentDeletesForGood(t *testing.T) {
	off := newTrashEnv(t, &settings.Trash{Enabled: false}, fullPerm)
	off.write("a.txt", 1)
	if rec := off.del("/a.txt", ""); rec.Code != http.StatusNoContent || rec.Header().Get("X-Trash") != "deleted" {
		t.Fatalf("disabled: status=%d X-Trash=%q", rec.Code, rec.Header().Get("X-Trash"))
	}
	if off.exists("a.txt") || off.exists(".trash") {
		t.Fatal("a disabled trash must not keep anything")
	}

	on := newTrashEnv(t, nil, fullPerm)
	on.write("b.txt", 1)
	if rec := on.del("/b.txt", "?permanent=true"); rec.Header().Get("X-Trash") != "deleted" {
		t.Fatalf("permanent: X-Trash=%q", rec.Header().Get("X-Trash"))
	}
	if on.exists("b.txt") || len(on.list().Items) != 0 {
		t.Fatal("a permanent delete must not go to the trash")
	}
}

func TestTrashTooLargeIsDeletedPermanently(t *testing.T) {
	e := newTrashEnv(t, &settings.Trash{Enabled: true, MaxBytes: 10}, fullPerm)
	e.write("big.bin", 50)

	rec := e.del("/big.bin", "")
	if rec.Code != http.StatusNoContent || rec.Header().Get("X-Trash") != "deleted" {
		t.Fatalf("status=%d X-Trash=%q", rec.Code, rec.Header().Get("X-Trash"))
	}
	if e.exists("big.bin") || len(e.list().Items) != 0 {
		t.Fatal("an item larger than the whole trash is deleted, not kept")
	}
}

func TestTrashQuotaEvictsTheOldest(t *testing.T) {
	e := newTrashEnv(t, &settings.Trash{Enabled: true, MaxBytes: 10}, fullPerm)
	for _, name := range []string{"one.bin", "two.bin", "three.bin"} {
		e.write(name, 6)
		if rec := e.del("/"+name, ""); rec.Code != http.StatusNoContent {
			t.Fatalf("%s: status=%d", name, rec.Code)
		}
		time.Sleep(2 * time.Millisecond) // distinct ids and times
	}

	list := e.list()
	if len(list.Items) != 1 || list.Items[0].Name != "three.bin" || list.TotalBytes != 6 {
		t.Fatalf("only the newest fits in 10 bytes: %+v", list)
	}
}

func TestTrashExpiry(t *testing.T) {
	e := newTrashEnv(t, &settings.Trash{Enabled: true, Days: 1}, fullPerm)
	e.write("old.txt", 1)
	e.write("new.txt", 1)
	e.del("/old.txt", "")
	e.del("/new.txt", "")

	// Age the first item by rewriting its metadata.
	items := e.list().Items
	var oldID string
	for _, it := range items {
		if it.Name == "old.txt" {
			oldID = it.ID
			it.DeletedAt = time.Now().Add(-48 * time.Hour)
			raw, _ := json.Marshal(it)
			if err := os.WriteFile(filepath.Join(e.scope, ".trash", it.ID, "meta.json"), raw, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}

	left := e.list().Items
	if len(left) != 1 || left[0].Name != "new.txt" {
		t.Fatalf("after expiry: %+v", left)
	}
	if e.exists(".trash/" + oldID) {
		t.Fatal("the expired item is still on disk")
	}
}

func TestTrashRestore(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	e.write("docs/deep/a.txt", 2)
	e.del("/docs/deep/a.txt", "")
	id := e.list().Items[0].ID
	restore := func(query string) *httptest.ResponseRecorder {
		return e.call(trashRestoreHandler, "", http.MethodPost, "/api/trash/"+id+"/restore"+query, map[string]string{"id": id})
	}

	// The folder it came from was removed too: it is recreated.
	if err := os.RemoveAll(filepath.Join(e.scope, "docs")); err != nil {
		t.Fatal(err)
	}
	// A different file now sits at the original path.
	e.write("docs/deep/a.txt", 9)

	if rec := restore(""); rec.Code != http.StatusConflict {
		t.Fatalf("conflict: status=%d, want 409", rec.Code)
	}
	rec := restore("?rename=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("rename restore: status=%d body=%q", rec.Code, rec.Body.String())
	}
	var out map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["path"] != "/docs/deep/a(1).txt" || !e.exists("docs/deep/a(1).txt") {
		t.Fatalf("restored to %q", out["path"])
	}
	if len(e.list().Items) != 0 || e.exists(".trash/"+id) {
		t.Fatal("a restored item must leave the trash")
	}
	if raw, _ := os.ReadFile(filepath.Join(e.scope, "docs/deep/a.txt")); len(raw) != 9 {
		t.Fatal("the file that was in the way was touched")
	}
}

func TestTrashRestoreCreatesMissingFolders(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	e.write("gone/dir/a.txt", 2)
	e.del("/gone/dir/a.txt", "")
	if err := os.RemoveAll(filepath.Join(e.scope, "gone")); err != nil {
		t.Fatal(err)
	}
	id := e.list().Items[0].ID

	rec := e.call(trashRestoreHandler, "", http.MethodPost, "/api/trash/"+id+"/restore", map[string]string{"id": id})
	if rec.Code != http.StatusOK || !e.exists("gone/dir/a.txt") {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTrashDeleteForeverAndEmpty(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		e.write(name, 1)
		e.del("/"+name, "")
		time.Sleep(2 * time.Millisecond)
	}
	items := e.list().Items

	rec := e.call(trashDeleteHandler, "", http.MethodDelete, "/api/trash/"+items[0].ID, map[string]string{"id": items[0].ID})
	if rec.Code != http.StatusNoContent || len(e.list().Items) != 2 {
		t.Fatalf("delete one: status=%d left=%d", rec.Code, len(e.list().Items))
	}

	rec = e.call(trashEmptyHandler, "", http.MethodDelete, "/api/trash", nil)
	if rec.Code != http.StatusNoContent || len(e.list().Items) != 0 {
		t.Fatalf("empty: status=%d left=%d", rec.Code, len(e.list().Items))
	}
}

func TestTrashDeletingInsideTheTrashIsPermanent(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	e.write("a.txt", 1)
	e.del("/a.txt", "")
	id := e.list().Items[0].ID

	rec := e.del("/.trash/"+id+"/data", "")
	if rec.Code != http.StatusNoContent || rec.Header().Get("X-Trash") != "deleted" {
		t.Fatalf("status=%d X-Trash=%q", rec.Code, rec.Header().Get("X-Trash"))
	}
	if e.exists(".trash/" + id + "/data") {
		t.Fatal("deleting inside the trash must remove the data for good")
	}
}

func TestTrashRejectsBadIDsAndNeedsDeletePermission(t *testing.T) {
	e := newTrashEnv(t, nil, fullPerm)
	for _, id := range []string{"../etc", "..", "abc", "1234567890-zzzzzzzz"} {
		rec := e.call(trashDeleteHandler, "", http.MethodDelete, "/api/trash/x", map[string]string{"id": id})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id %q: status=%d, want 400", id, rec.Code)
		}
	}

	noDelete := newTrashEnv(t, nil, users.Permissions{Create: true, Download: true})
	if rec := noDelete.call(trashListHandler, "", http.MethodGet, "/api/trash", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("list without the delete permission: status=%d, want 403", rec.Code)
	}
}

func TestTrashSettingsRoundTrip(t *testing.T) {
	admin := users.Permissions{Admin: true}
	e := newTrashEnv(t, nil, admin)

	get := func() settingsData {
		rec := e.call(settingsGetHandler, "", http.MethodGet, "/api/settings", nil)
		var out settingsData
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	if got := get(); got.Trash == nil || !got.Trash.Enabled || got.Trash.Days != 30 || got.Trash.MaxBytes != 10<<30 {
		t.Fatalf("defaults: %+v", got.Trash)
	}

	put := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
		req.Header.Set("X-Auth", signToken(t, admin, e.key))
		rec := httptest.NewRecorder()
		handle(settingsPutHandler, "", e.st, &settings.Server{}).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := put(`{"trash":{"enabled":false,"maxBytes":5368709120,"days":7}}`); code != http.StatusOK {
		t.Fatalf("put: status=%d", code)
	}
	if got := get(); got.Trash.Enabled || got.Trash.MaxBytes != 5368709120 || got.Trash.Days != 7 {
		t.Fatalf("after put: %+v", got.Trash)
	}
	if code := put(`{"trash":{"enabled":true,"maxBytes":-1,"days":7}}`); code != http.StatusBadRequest {
		t.Fatalf("negative size: status=%d, want 400", code)
	}
	// A request without the trash section leaves it alone.
	if code := put(`{}`); code != http.StatusOK {
		t.Fatalf("put without trash: status=%d", code)
	}
	if got := get(); got.Trash.Enabled || got.Trash.Days != 7 {
		t.Fatalf("an absent section changed the trash: %+v", got.Trash)
	}
}
