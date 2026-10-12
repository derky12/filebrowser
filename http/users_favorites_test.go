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

func putFavorites(t *testing.T, favorites string) (*httptest.ResponseRecorder, func() []string) {
	t.Helper()
	key := []byte("test-signing-key")
	st := scopedUserStorage(t, t.TempDir(), users.Permissions{}, key)

	body := `{"what":"user","which":["favorites"],"data":{"id":1,"favorites":` + favorites + `}}`
	req, _ := http.NewRequest(http.MethodPut, "/api/users/1", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	req.Header.Set("X-Auth", signToken(t, users.Permissions{}, key))

	rec := httptest.NewRecorder()
	handle(userPutHandler, "", st, &settings.Server{}).ServeHTTP(rec, req)

	return rec, func() []string {
		u, err := st.Users.Get("", false, uint(1))
		if err != nil {
			t.Fatal(err)
		}
		return u.Favorites
	}
}

func TestUserPutFavoritesAllowedForNonAdmin(t *testing.T) {
	rec, saved := putFavorites(t, `["/A/","/B","/A"]`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	got := saved()
	if len(got) != 2 || got[0] != "/A" || got[1] != "/B" {
		t.Fatalf("favorites=%v, want [/A /B]", got)
	}
}

func TestUserPutFavoritesRejectsRelativePath(t *testing.T) {
	rec, saved := putFavorites(t, `["not/absolute"]`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
	if got := saved(); len(got) != 0 {
		t.Fatalf("favorites=%v, want none saved", got)
	}
}
