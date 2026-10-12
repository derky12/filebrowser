package fbhttp

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/syncthing"
)

func TestConflictOriginal(t *testing.T) {
	cases := map[string]string{
		"report.sync-conflict-20240131-235959-ABCDEFG.txt":     "report.txt",
		"report.sync-conflict-20240131-235959.txt":             "report.txt",
		"archive.tar.sync-conflict-20240131-010203-AB12CDE.gz": "archive.tar.gz",
		"noext.sync-conflict-20240131-010203-AB12CDE":          "noext",
		"My File.sync-conflict-20240131-010203-AB12CDE.docx":   "My File.docx",
	}
	for name, want := range cases {
		got, at, ok := conflictOriginal(name)
		if !ok || got != want || at.IsZero() {
			t.Errorf("%q: got %q ok=%v at=%v, want %q", name, got, ok, at, want)
		}
	}
	for _, name := range []string{"report.txt", "sync-conflict.txt", "a.sync-conflict-2024.txt", ".sync-conflict-20240131-010203-AB12CDE.txt"} {
		if _, _, ok := conflictOriginal(name); ok {
			t.Errorf("%q must not be taken for a conflict file", name)
		}
	}
}

func syncedFolder(e *syncEnv, rel string) {
	e.fake.folders["fb-1"] = syncthing.Folder{ID: "fb-1", Label: "docs", Path: filepath.Join(e.scope, rel), Type: syncthing.TypeSendReceive}
}

func TestSyncConflictsListAndResolve(t *testing.T) {
	e := newSyncEnv(t)
	e.mkdir("docs/sub")
	e.mkdir("docs/.stversions")
	e.mkdir("other")
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(e.scope, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/a.txt", "mine")
	write("docs/a.sync-conflict-20240131-235959-ABCDEFG.txt", "theirs")
	write("docs/sub/b.txt", "b-mine")
	write("docs/sub/b.sync-conflict-20240201-000000-ABCDEFG.txt", "b-theirs")
	write("docs/.stversions/old.sync-conflict-20240101-000000-ABCDEFG.txt", "ignored")
	write("other/c.sync-conflict-20240101-000000-ABCDEFG.txt", "not in a synced folder")
	syncedFolder(e, "docs")

	list := func() []syncConflict {
		rec := e.call(syncConflictsGetHandler, e.adminPerm, http.MethodGet, "/api/sync/conflicts", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("list: status=%d body=%q", rec.Code, rec.Body.String())
		}
		var out []syncConflict
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}

	got := list()
	if len(got) != 2 {
		t.Fatalf("conflicts: %+v", got)
	}
	// Newest first.
	if got[0].Path != "/docs/sub/b.sync-conflict-20240201-000000-ABCDEFG.txt" || got[0].Original != "/docs/sub/b.txt" || !got[0].OriginalExists || got[0].OriginalSize != 6 || got[0].Size != 8 {
		t.Fatalf("first conflict: %+v", got[0])
	}

	resolve := func(path, keep string) int {
		body, _ := json.Marshal(map[string]string{"path": path, "keep": keep})
		return e.call(syncConflictResolveHandler, e.adminPerm, http.MethodPost, "/api/sync/conflicts/resolve", string(body), nil).Code
	}

	// Keep the other side's version: it takes the original's place, the
	// original goes to the trash.
	if code := resolve("/docs/a.sync-conflict-20240131-235959-ABCDEFG.txt", "conflict"); code != http.StatusOK {
		t.Fatalf("keep conflict: status=%d", code)
	}
	if raw, _ := os.ReadFile(filepath.Join(e.scope, "docs/a.txt")); string(raw) != "theirs" {
		t.Fatalf("a.txt is %q, want the conflict's content", raw)
	}
	if _, err := os.Stat(filepath.Join(e.scope, "docs/a.sync-conflict-20240131-235959-ABCDEFG.txt")); err == nil {
		t.Fatal("the conflict file is still there")
	}

	// Keep the original: the conflict file is discarded.
	if code := resolve("/docs/sub/b.sync-conflict-20240201-000000-ABCDEFG.txt", "original"); code != http.StatusOK {
		t.Fatalf("keep original: status=%d", code)
	}
	if raw, _ := os.ReadFile(filepath.Join(e.scope, "docs/sub/b.txt")); string(raw) != "b-mine" {
		t.Fatalf("b.txt is %q", raw)
	}
	if n := len(list()); n != 0 {
		t.Fatalf("%d conflicts left", n)
	}

	// Both discarded versions went to the trash (default settings keep it on).
	trashed := 0
	entries, _ := os.ReadDir(filepath.Join(e.scope, ".trash"))
	for range entries {
		trashed++
	}
	if trashed != 2 {
		t.Fatalf("%d items in the trash, want 2", trashed)
	}
}

func TestSyncConflictResolveRejections(t *testing.T) {
	e := newSyncEnv(t)
	e.mkdir("docs")
	e.mkdir("other")
	for _, rel := range []string{"docs/plain.txt", "other/c.sync-conflict-20240101-000000-ABCDEFG.txt"} {
		if err := os.WriteFile(filepath.Join(e.scope, rel), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	syncedFolder(e, "docs")

	for name, body := range map[string]string{
		"not a conflict file":       `{"path":"/docs/plain.txt","keep":"original"}`,
		"outside the synced folder": `{"path":"/other/c.sync-conflict-20240101-000000-ABCDEFG.txt","keep":"original"}`,
		"bad keep":                  `{"path":"/docs/plain.txt","keep":"both"}`,
	} {
		rec := e.call(syncConflictResolveHandler, e.adminPerm, http.MethodPost, "/api/sync/conflicts/resolve", body, nil)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("%s: status=%d, want 400 or 404", name, rec.Code)
		}
	}
	if _, err := os.Stat(filepath.Join(e.scope, "other/c.sync-conflict-20240101-000000-ABCDEFG.txt")); err != nil {
		t.Fatal("a file outside the synced folders was touched")
	}
}

func TestSyncIgnores(t *testing.T) {
	e := newSyncEnv(t)
	vars := map[string]string{"id": "fb-1"}

	put := func(body string) int {
		return e.call(syncIgnoresPutHandler, e.adminPerm, http.MethodPut, "/api/sync/folders/fb-1/ignores", body, vars).Code
	}
	if code := put(`{"lines":["*.tmp","node_modules","(?d).DS_Store"]}`); code != http.StatusNoContent {
		t.Fatalf("put: status=%d", code)
	}
	rec := e.call(syncIgnoresGetHandler, e.adminPerm, http.MethodGet, "/api/sync/folders/fb-1/ignores", "", vars)
	var got syncIgnores
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Lines) != 3 || got.Lines[0] != "*.tmp" {
		t.Fatalf("get: %+v", got)
	}

	for name, body := range map[string]string{
		"include":      `{"lines":["#include /etc/passwd"]}`,
		"include pad":  `{"lines":["  #include other"]}`,
		"newline":      `{"lines":["a\nb"]}`,
		"too long":     `{"lines":["` + strings.Repeat("a", maxIgnoreLength+1) + `"]}`,
		"invalid json": `{`,
	} {
		if code := put(body); code != http.StatusBadRequest {
			t.Errorf("%s: status=%d, want 400", name, code)
		}
	}
	if len(e.fake.ignores["fb-1"]) != 3 {
		t.Fatal("a rejected request changed the patterns")
	}
}

func TestSyncLimits(t *testing.T) {
	e := newSyncEnv(t)

	put := func(body string) int {
		return e.call(syncLimitsPutHandler, e.adminPerm, http.MethodPut, "/api/sync/limits", body, nil).Code
	}
	if code := put(`{"maxRecvKbps":2048,"maxSendKbps":512,"limitBandwidthInLan":true}`); code != http.StatusNoContent {
		t.Fatalf("put: status=%d", code)
	}
	rec := e.call(syncLimitsGetHandler, e.adminPerm, http.MethodGet, "/api/sync/limits", "", nil)
	var got syncthing.Limits
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.MaxRecvKbps != 2048 || got.MaxSendKbps != 512 || !got.LimitBandwidthInLan {
		t.Fatalf("limits: %+v", got)
	}

	for _, body := range []string{`{"maxRecvKbps":-1}`, `{"maxSendKbps":99999999}`} {
		if code := put(body); code != http.StatusBadRequest {
			t.Errorf("%s: status=%d, want 400", body, code)
		}
	}
}

func TestSyncExtrasNeedAdmin(t *testing.T) {
	e := newSyncEnv(t)
	e.server = &settings.Server{} // Syncthing not configured
	for _, h := range []handleFunc{syncConflictsGetHandler, syncLimitsGetHandler, syncIgnoresGetHandler} {
		if rec := e.call(h, e.adminPerm, http.MethodGet, "/x", "", map[string]string{"id": "fb-1"}); rec.Code != http.StatusNotFound {
			t.Errorf("unconfigured: status=%d, want 404", rec.Code)
		}
	}
}
