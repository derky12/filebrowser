package users

import (
	"reflect"
	"testing"
)

func TestCleanFavorites(t *testing.T) {
	t.Run("normalizes and dedupes", func(t *testing.T) {
		got, err := cleanFavorites([]string{"/a/b/", "/a/b", "/a/../c", "/"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"/a/b", "/c", "/"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("nil becomes empty list", func(t *testing.T) {
		got, err := cleanFavorites(nil)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("rejects relative paths", func(t *testing.T) {
		if _, err := cleanFavorites([]string{"a/b"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("cannot escape the root", func(t *testing.T) {
		got, err := cleanFavorites([]string{"/../../etc"})
		if err != nil || !reflect.DeepEqual(got, []string{"/etc"}) {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("caps the list size", func(t *testing.T) {
		in := make([]string, 0, MaxFavorites+1)
		for i := 0; i <= MaxFavorites; i++ {
			in = append(in, "/d"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		}
		if _, err := cleanFavorites(in); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCleanFavoritesViaClean(t *testing.T) {
	u := &User{Favorites: []string{"/x/", "/x"}}
	if err := u.Clean("/tmp", false, "Favorites"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(u.Favorites, []string{"/x"}) {
		t.Fatalf("got %v", u.Favorites)
	}
}
