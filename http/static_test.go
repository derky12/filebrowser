package fbhttp

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

func TestStaticScripts(t *testing.T) {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	_, _ = zw.Write([]byte("console.log('zipped')"))
	_ = zw.Close()

	assets := fstest.MapFS{
		"assets/zipped.js.gz": {Data: zipped.Bytes()},
		"assets/tiny.js":      {Data: []byte("export default 1")}, // no .gz: the build skips tiny files
		"assets/font.woff2":   {Data: []byte("font")},
	}
	st := scopedUserStorage(t, t.TempDir(), users.Permissions{}, []byte("test-signing-key"))
	_, static := getStaticHandlers(st, &settings.Server{}, assets)

	get := func(path, acceptEncoding string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodGet, "/static/"+path, http.NoBody)
		if acceptEncoding != "" {
			req.Header.Set("Accept-Encoding", acceptEncoding)
		}
		rec := httptest.NewRecorder()
		static.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("assets/zipped.js", "gzip"); rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("precompressed script for a gzip client: status=%d encoding=%q", rec.Code, rec.Header().Get("Content-Encoding"))
	}

	rec := get("assets/zipped.js", "")
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || string(body) != "console.log('zipped')" {
		t.Fatalf("precompressed script for a plain client: status=%d body=%q", rec.Code, body)
	}

	rec = get("assets/tiny.js", "gzip")
	body, _ = io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || string(body) != "export default 1" {
		t.Fatalf("script without a .gz copy must still be served: status=%d body=%q", rec.Code, body)
	}

	if rec := get("assets/font.woff2", ""); rec.Code != http.StatusOK {
		t.Fatalf("non-script asset: status=%d", rec.Code)
	}
	if rec := get("assets/missing.js", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing script: status=%d, want 404", rec.Code)
	}
}
