package fbhttp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
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

type archiveEnv struct {
	t     *testing.T
	scope string
	call  func(body map[string]interface{}) (*httptest.ResponseRecorder, archiveResult)
}

func newArchiveEnv(t *testing.T, perm users.Permissions, denied ...string) *archiveEnv {
	t.Helper()
	scope, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, scope, perm, key)
	if len(denied) > 0 {
		set := &settings.Settings{Key: key, DirMode: 0o755, FileMode: 0o644}
		for _, p := range denied {
			set.Rules = append(set.Rules, rules.Rule{Path: p, Allow: false})
		}
		if err := st.Settings.Save(set); err != nil {
			t.Fatal(err)
		}
	}

	e := &archiveEnv{t: t, scope: scope}
	e.call = func(body map[string]interface{}) (*httptest.ResponseRecorder, archiveResult) {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, "/api/archive", bytes.NewReader(raw))
		req.Header.Set("X-Auth", signToken(t, perm, key))
		rec := httptest.NewRecorder()
		handle(archivePostHandler, "", st, &settings.Server{}).ServeHTTP(rec, req)
		var res archiveResult
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		return rec, res
	}
	return e
}

func (e *archiveEnv) write(rel, content string) {
	e.t.Helper()
	p := filepath.Join(e.scope, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *archiveEnv) read(rel string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(e.scope, rel))
	return string(raw), err == nil
}

var archivePerm = users.Permissions{Create: true, Download: true}

func TestArchiveRoundTrip(t *testing.T) {
	for _, format := range []string{"zip", "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst"} {
		t.Run(format, func(t *testing.T) {
			e := newArchiveEnv(t, archivePerm)
			e.write("proj/a.txt", "alpha")
			e.write("proj/sub/b.txt", "beta")
			e.write("other.txt", "gamma")

			rec, res := e.call(map[string]interface{}{
				"action": "compress", "paths": []string{"/proj", "/other.txt"}, "destination": "/out/pack", "format": format,
			})
			if rec.Code != http.StatusOK || !strings.HasSuffix(res.Path, "."+format) || res.Files != 3 || res.Size == 0 {
				t.Fatalf("compress: status=%d body=%q", rec.Code, rec.Body.String())
			}

			rec, ex := e.call(map[string]interface{}{"action": "extract", "path": res.Path, "destination": "/restored"})
			if rec.Code != http.StatusOK || ex.Files != 3 || ex.Skipped != 0 {
				t.Fatalf("extract: status=%d body=%q", rec.Code, rec.Body.String())
			}
			for rel, want := range map[string]string{"restored/proj/a.txt": "alpha", "restored/proj/sub/b.txt": "beta", "restored/other.txt": "gamma"} {
				if got, ok := e.read(rel); !ok || got != want {
					t.Errorf("%s: got %q ok=%v, want %q", rel, got, ok, want)
				}
			}
		})
	}
}

func TestArchiveCompressDetails(t *testing.T) {
	e := newArchiveEnv(t, archivePerm)
	e.write("d/a.txt", "a")
	e.write("d/inner/b.txt", "b")

	// An item inside another selected folder is not packed twice; the archive
	// never contains itself; an existing archive is not overwritten.
	rec, res := e.call(map[string]interface{}{"action": "compress", "paths": []string{"/d", "/d/inner/b.txt"}, "destination": "/d/x.zip", "format": "zip"})
	if rec.Code != http.StatusOK || res.Files != 2 || res.Path != "/d/x.zip" {
		t.Fatalf("first: status=%d %+v", rec.Code, res)
	}
	_, again := e.call(map[string]interface{}{"action": "compress", "paths": []string{"/d"}, "destination": "/d/x.zip", "format": "zip"})
	if again.Path != "/d/x(1).zip" || again.Files != 3 { // a.txt, b.txt and the first x.zip
		t.Fatalf("second: %+v", again)
	}
}

// buildZip makes an archive with the given entries, as a hostile one would.
func buildZip(t *testing.T, build func(w *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	build(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func addZipFile(t *testing.T, w *zip.Writer, name, content string) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(content))
}

func TestArchiveExtractRefusesHostileEntries(t *testing.T) {
	e := newArchiveEnv(t, archivePerm)
	data := buildZip(t, func(w *zip.Writer) {
		addZipFile(t, w, "ok.txt", "fine")
		addZipFile(t, w, "../escape.txt", "evil")
		addZipFile(t, w, "a/../../escape2.txt", "evil")
		addZipFile(t, w, `..\winescape.txt`, "evil")
		addZipFile(t, w, "/abs.txt", "absolute")
		link, _ := w.CreateHeader(&zip.FileHeader{Name: "link", Method: zip.Store})
		link.(interface{ Write([]byte) (int, error) }).Write([]byte("/etc/passwd"))
		hdr := &zip.FileHeader{Name: "symlink", Method: zip.Store}
		hdr.SetMode(os.ModeSymlink | 0o777)
		sl, _ := w.CreateHeader(hdr)
		_, _ = sl.Write([]byte("/etc"))
	})
	if err := os.WriteFile(filepath.Join(e.scope, "evil.zip"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	rec, res := e.call(map[string]interface{}{"action": "extract", "path": "/evil.zip", "destination": "/out"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}

	if got, ok := e.read("out/ok.txt"); !ok || got != "fine" {
		t.Fatalf("the harmless file was not extracted: %q %v", got, ok)
	}
	if got, ok := e.read("out/abs.txt"); !ok || got != "absolute" {
		t.Fatalf("an absolute name must land inside the destination: %q %v", got, ok)
	}
	for _, escaped := range []string{"escape.txt", "escape2.txt", "winescape.txt", "out/escape.txt", "out/winescape.txt"} {
		if _, ok := e.read(escaped); ok {
			t.Errorf("%s was written", escaped)
		}
	}
	if _, err := os.Lstat(filepath.Join(e.scope, "out", "symlink")); err == nil {
		t.Error("a symlink entry was created")
	}
	if res.Skipped < 4 {
		t.Errorf("skipped=%d, want at least the 3 traversal names and the symlink: %+v", res.Skipped, res.Errors)
	}
}

func TestArchiveExtractDoesNotOverwriteUnlessAsked(t *testing.T) {
	e := newArchiveEnv(t, archivePerm)
	data := buildZip(t, func(w *zip.Writer) { addZipFile(t, w, "a.txt", "from the archive") })
	if err := os.WriteFile(filepath.Join(e.scope, "p.zip"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	e.write("out/a.txt", "mine")

	_, res := e.call(map[string]interface{}{"action": "extract", "path": "/p.zip", "destination": "/out"})
	if got, _ := e.read("out/a.txt"); got != "mine" || res.Skipped != 1 || res.Files != 0 {
		t.Fatalf("overwritten without being asked: %q %+v", got, res)
	}

	_, res = e.call(map[string]interface{}{"action": "extract", "path": "/p.zip", "destination": "/out", "overwrite": true})
	if got, _ := e.read("out/a.txt"); got != "from the archive" || res.Files != 1 {
		t.Fatalf("not overwritten when asked: %q %+v", got, res)
	}
}

func TestArchiveExtractLimits(t *testing.T) {
	e := newArchiveEnv(t, archivePerm)
	data := buildZip(t, func(w *zip.Writer) {
		addZipFile(t, w, "one.bin", strings.Repeat("x", 600))
		addZipFile(t, w, "two.bin", strings.Repeat("x", 600))
	})
	if err := os.WriteFile(filepath.Join(e.scope, "bomb.zip"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	old := maxExtractBytes
	maxExtractBytes = 1000
	defer func() { maxExtractBytes = old }()

	rec, _ := e.call(map[string]interface{}{"action": "extract", "path": "/bomb.zip", "destination": "/out"})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(e.scope, "out")); err == nil {
		t.Fatal("a failed extraction left its new folder behind")
	}

	oldEntries := maxExtractEntries
	maxExtractEntries = 1
	defer func() { maxExtractEntries = oldEntries }()
	maxExtractBytes = old
	if rec, _ := e.call(map[string]interface{}{"action": "extract", "path": "/bomb.zip", "destination": "/out2"}); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("entry limit: status=%d, want 413", rec.Code)
	}
}

func TestArchiveExtractTarGzWithTraversal(t *testing.T) {
	e := newArchiveEnv(t, archivePerm)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	add := func(name, content string, typeflag byte) {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: typeflag}
		if typeflag == tar.TypeSymlink {
			hdr.Size, hdr.Linkname = 0, content
		}
		_ = tw.WriteHeader(hdr)
		if typeflag == tar.TypeReg {
			_, _ = tw.Write([]byte(content))
		}
	}
	add("good.txt", "good", tar.TypeReg)
	add("../../escape.txt", "evil", tar.TypeReg)
	add("lnk", "/etc/passwd", tar.TypeSymlink)
	_ = tw.Close()
	_ = gz.Close()
	if err := os.WriteFile(filepath.Join(e.scope, "t.tar.gz"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, res := e.call(map[string]interface{}{"action": "extract", "path": "/t.tar.gz", "destination": "/out"})
	if rec.Code != http.StatusOK || res.Files != 1 || res.Skipped != 2 {
		t.Fatalf("status=%d %+v", rec.Code, res)
	}
	if _, ok := e.read("escape.txt"); ok {
		t.Fatal("a traversal entry was written")
	}
}

func TestArchiveRejections(t *testing.T) {
	e := newArchiveEnv(t, archivePerm, "/secret")
	e.write("a.txt", "a")
	e.write("secret/s.txt", "s")
	e.write("notanarchive.zip", "plain text")

	cases := map[string]struct {
		body map[string]interface{}
		want int
	}{
		"unknown action":     {map[string]interface{}{"action": "explode"}, http.StatusBadRequest},
		"unknown format":     {map[string]interface{}{"action": "compress", "paths": []string{"/a.txt"}, "destination": "/x", "format": "rar"}, http.StatusBadRequest},
		"scope root":         {map[string]interface{}{"action": "compress", "paths": []string{"/"}, "destination": "/x", "format": "zip"}, http.StatusBadRequest},
		"nothing selected":   {map[string]interface{}{"action": "compress", "paths": []string{}, "destination": "/x", "format": "zip"}, http.StatusBadRequest},
		"denied source":      {map[string]interface{}{"action": "compress", "paths": []string{"/secret"}, "destination": "/x", "format": "zip"}, http.StatusForbidden},
		"denied destination": {map[string]interface{}{"action": "compress", "paths": []string{"/a.txt"}, "destination": "/secret/x", "format": "zip"}, http.StatusForbidden},
		"into the trash":     {map[string]interface{}{"action": "compress", "paths": []string{"/a.txt"}, "destination": "/.trash/x", "format": "zip"}, http.StatusForbidden},
		"not an archive":     {map[string]interface{}{"action": "extract", "path": "/notanarchive.zip", "destination": "/out"}, http.StatusBadRequest},
		"missing archive":    {map[string]interface{}{"action": "extract", "path": "/nope.zip", "destination": "/out"}, http.StatusNotFound},
		"extract to root":    {map[string]interface{}{"action": "extract", "path": "/notanarchive.zip", "destination": "/"}, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rec, _ := e.call(tc.body); rec.Code != tc.want {
				t.Fatalf("status=%d, want %d (%q)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	noCreate := newArchiveEnv(t, users.Permissions{Download: true})
	noCreate.write("a.txt", "a")
	if rec, _ := noCreate.call(map[string]interface{}{"action": "compress", "paths": []string{"/a.txt"}, "destination": "/x", "format": "zip"}); rec.Code != http.StatusForbidden {
		t.Fatalf("without the create permission: status=%d, want 403", rec.Code)
	}
}
