package users

import (
	"reflect"
	"testing"
)

func TestDotfileHidden(t *testing.T) {
	u := &User{HideDotfiles: true}
	cases := map[string]bool{
		"/a/.hidden":       true,
		"/.trash":          true,
		"/a/.env":          false,
		"/a/.env.sample":   false,
		"/a/.git":          false,
		"/a/.gitignore":    false,
		"/a/visible.txt":   false,
		"/a/.dockerignore": false,
	}
	for p, want := range cases {
		if got := u.DotfileHidden(p); got != want {
			t.Errorf("DotfileHidden(%q) = %v, want %v", p, got, want)
		}
	}

	u.ShowDotfiles = []string{}
	if !u.DotfileHidden("/a/.env") {
		t.Error("an emptied list must hide .env again")
	}

	u.ShowDotfiles = []string{".trash"}
	if u.DotfileHidden("/.trash") || !u.DotfileHidden("/.env") {
		t.Error("a custom list replaces the default one")
	}

	u.HideDotfiles = false
	if u.DotfileHidden("/a/.hidden") {
		t.Error("nothing is hidden when HideDotfiles is off")
	}
}

func TestCleanShowDotfiles(t *testing.T) {
	got, err := cleanShowDotfiles([]string{" .env* ", "", ".git", ".env*"})
	if err != nil || !reflect.DeepEqual(got, []string{".env*", ".git"}) {
		t.Fatalf("got %v, %v", got, err)
	}

	for _, bad := range []string{"a/b", `a\b`, "[abc"} {
		if _, err := cleanShowDotfiles([]string{bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
