package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

func callFormat(t *testing.T, lang, body string) *httptest.ResponseRecorder {
	t.Helper()
	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)

	req, _ := http.NewRequest(http.MethodPost, "/api/format/"+lang, strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"lang": lang})
	req.Header.Set("X-Auth", signToken(t, perm, key))
	rec := httptest.NewRecorder()
	handle(formatHandler, "", st, &settings.Server{}).ServeHTTP(rec, req)
	return rec
}

func TestFormatGo(t *testing.T) {
	rec := callFormat(t, "go", "package main\nfunc main(){\nx:=1\n_ = x}\n")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	want := "package main\n\nfunc main() {\n\tx := 1\n\t_ = x\n}\n"
	if rec.Body.String() != want {
		t.Fatalf("got %q, want %q", rec.Body.String(), want)
	}
}

func TestFormatGoSyntaxError(t *testing.T) {
	rec := callFormat(t, "go", "package main\nfunc main( {")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "expected") {
		t.Fatalf("the parse error should be reported: %q", rec.Body.String())
	}
}

func TestFormatUnknownLanguage(t *testing.T) {
	if rec := callFormat(t, "cobol", "x"); rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestFormatRequiresLogin(t *testing.T) {
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, t.TempDir(), users.Permissions{}, key)
	req, _ := http.NewRequest(http.MethodPost, "/api/format/go", strings.NewReader("package main"))
	req = mux.SetURLVars(req, map[string]string{"lang": "go"})
	rec := httptest.NewRecorder()
	handle(formatHandler, "", st, &settings.Server{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
}
