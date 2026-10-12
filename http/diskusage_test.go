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

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

func duCall(t *testing.T, scope string, p string, denied ...string) (*httptest.ResponseRecorder, duResponse) {
	t.Helper()
	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, scope, perm, key)
	if len(denied) > 0 {
		set := &settings.Settings{Key: key}
		for _, d := range denied {
			set.Rules = append(set.Rules, ruleDeny(d))
		}
		if err := st.Settings.Save(set); err != nil {
			t.Fatal(err)
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/du"+p, http.NoBody)
	req.Header.Set("X-Auth", signToken(t, perm, key))
	rec := httptest.NewRecorder()
	handle(diskUsageHandler, "/api/du", st, &settings.Server{}).ServeHTTP(rec, req)

	var out duResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func fill(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiskUsage(t *testing.T) {
	scope, _ := filepath.EvalSymlinks(t.TempDir())
	fill(t, filepath.Join(scope, "big", "a.bin"), 1000)
	fill(t, filepath.Join(scope, "big", "deep", "b.bin"), 500)
	fill(t, filepath.Join(scope, "small", "c.bin"), 10)
	fill(t, filepath.Join(scope, "top.txt"), 100)
	fill(t, filepath.Join(scope, ".hidden", "h.bin"), 7)

	rec, out := duCall(t, scope, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if out.Total != 1617 || out.Files != 5 || out.Partial {
		t.Fatalf("total=%d files=%d partial=%v", out.Total, out.Files, out.Partial)
	}
	// Biggest first, folders measured recursively, dotfiles included.
	want := []struct {
		name string
		size int64
		dir  bool
	}{{"big", 1500, true}, {"top.txt", 100, false}, {"small", 10, true}, {".hidden", 7, true}}
	if len(out.Entries) != len(want) {
		t.Fatalf("entries: %+v", out.Entries)
	}
	for i, w := range want {
		e := out.Entries[i]
		if e.Name != w.name || e.Size != w.size || e.IsDir != w.dir || !e.Complete || e.Path != "/"+w.name {
			t.Errorf("entry %d: %+v, want %+v", i, e, w)
		}
	}

	_, sub := duCall(t, scope, "/big")
	if sub.Total != 1500 || len(sub.Entries) != 2 || sub.Entries[0].Name != "a.bin" {
		t.Fatalf("sub: %+v", sub)
	}
}

func TestDiskUsageSkipsSymlinksAndRejectsFiles(t *testing.T) {
	scope, _ := filepath.EvalSymlinks(t.TempDir())
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	fill(t, filepath.Join(outside, "secret.bin"), 9999)
	fill(t, filepath.Join(scope, "real", "a.bin"), 10)
	if err := os.Symlink(outside, filepath.Join(scope, "real", "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	_, out := duCall(t, scope, "/real")
	if out.Total >= 9999 {
		t.Fatalf("a symlink was followed: total=%d", out.Total)
	}

	if rec, _ := duCall(t, scope, "/real/a.bin"); rec.Code != http.StatusBadRequest {
		t.Fatalf("a file: status=%d, want 400", rec.Code)
	}
	if rec, _ := duCall(t, scope, "/missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: status=%d, want 404", rec.Code)
	}
}

func TestDiskUsageHonoursRules(t *testing.T) {
	scope, _ := filepath.EvalSymlinks(t.TempDir())
	fill(t, filepath.Join(scope, "open", "a.bin"), 10)
	fill(t, filepath.Join(scope, "secret", "b.bin"), 99)

	rec, out := duCall(t, scope, "/", "/secret")
	if rec.Code != http.StatusOK || out.Total != 10 || len(out.Entries) != 1 {
		t.Fatalf("a denied folder leaked: %+v", out)
	}
	if rec, _ := duCall(t, scope, "/secret", "/secret"); rec.Code != http.StatusForbidden {
		t.Fatalf("a denied path: status=%d, want 403", rec.Code)
	}
}

func TestDiskUsageReportsPartialResults(t *testing.T) {
	scope, _ := filepath.EvalSymlinks(t.TempDir())
	for i := 0; i < 50; i++ {
		fill(t, filepath.Join(scope, "many", "d"+string(rune('a'+i%20)), "f"+string(rune('a'+i))+".bin"), 1)
	}

	old := duBudget
	duBudget = time.Nanosecond
	defer func() { duBudget = old }()
	duCache.Lock()
	duCache.m = map[string]duResult{}
	duCache.Unlock()

	_, out := duCall(t, scope, "/")
	if !out.Partial {
		t.Fatalf("a request out of time must say it is partial: %+v", out)
	}
}

func ruleDeny(path string) rules.Rule { return rules.Rule{Path: path, Allow: false} }
