package users

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/spf13/afero"
)

// ViewMode describes a view mode.
type ViewMode string

const (
	ListViewMode   ViewMode = "list"
	MosaicViewMode ViewMode = "mosaic"
)

// User describes a user.
type User struct {
	ID                    uint          `storm:"id,increment" json:"id"`
	Username              string        `storm:"unique" json:"username"`
	Password              string        `json:"password"`
	Scope                 string        `json:"scope"`
	Locale                string        `json:"locale"`
	LockPassword          bool          `json:"lockPassword"`
	ViewMode              ViewMode      `json:"viewMode"`
	SingleClick           bool          `json:"singleClick"`
	RedirectAfterCopyMove bool          `json:"redirectAfterCopyMove"`
	Perm                  Permissions   `json:"perm"`
	Commands              []string      `json:"commands"`
	Sorting               files.Sorting `json:"sorting"`
	Fs                    afero.Fs      `json:"-" yaml:"-"`
	Rules                 []rules.Rule  `json:"rules"`
	HideDotfiles          bool          `json:"hideDotfiles"`
	// ShowDotfiles lists name patterns (path.Match, tested against the base
	// name) of dotfiles that stay visible while HideDotfiles is on. nil means
	// "never set" and falls back to DefaultShowDotfiles; an empty list means
	// the user cleared it.
	ShowDotfiles   []string `json:"showDotfiles"`
	DateFormat     bool     `json:"dateFormat"`
	AceEditorTheme string   `json:"aceEditorTheme"`
	Favorites      []string `json:"favorites"`
	// FolderColors maps a folder path to a "#rrggbb" display color.
	FolderColors map[string]string `json:"folderColors"`
}

// DefaultShowDotfiles is the ShowDotfiles list of users that never set one.
// The trash (.trash) is deliberately not in it.
var DefaultShowDotfiles = []string{".env*", ".git*", ".dockerignore"}

// MaxShowDotfiles is the maximum number of dotfile patterns a user can keep.
const MaxShowDotfiles = 100

// EffectiveShowDotfiles returns the dotfile patterns that stay visible.
func (u *User) EffectiveShowDotfiles() []string {
	if u.ShowDotfiles == nil {
		return DefaultShowDotfiles
	}

	return u.ShowDotfiles
}

// DotfileHidden reports whether HideDotfiles hides the item at p: its base name
// starts with a dot and is not covered by a ShowDotfiles pattern.
func (u *User) DotfileHidden(p string) bool {
	if !u.HideDotfiles || !rules.MatchHidden(p) {
		return false
	}

	base := path.Base(filepath.ToSlash(p))
	for _, pattern := range u.EffectiveShowDotfiles() {
		if ok, _ := path.Match(pattern, base); ok {
			return false
		}
	}

	return true
}

// MaxFavorites is the maximum number of favorite folders a user can keep.
const MaxFavorites = 100

// MaxFolderColors is the maximum number of colored folders a user can keep.
const MaxFolderColors = 500

var folderColorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// GetRules implements rules.Provider.
func (u *User) GetRules() []rules.Rule {
	return u.Rules
}

var checkableFields = []string{
	"Username",
	"Password",
	"Scope",
	"ViewMode",
	"Commands",
	"Sorting",
	"Rules",
	"Favorites",
	"FolderColors",
	"ShowDotfiles",
}

// Clean cleans up a user and verifies if all its fields
// are alright to be saved.
func (u *User) Clean(baseScope string, followExternalSymlinks bool, fields ...string) error {
	if len(fields) == 0 {
		fields = checkableFields
	}

	for _, field := range fields {
		switch field {
		case "Username":
			if u.Username == "" {
				return fberrors.ErrEmptyUsername
			}
		case "Password":
			if u.Password == "" {
				return fberrors.ErrEmptyPassword
			}
		case "ViewMode":
			if u.ViewMode == "" {
				u.ViewMode = ListViewMode
			}
		case "Commands":
			if u.Commands == nil {
				u.Commands = []string{}
			}
		case "Sorting":
			if u.Sorting.By == "" {
				u.Sorting.By = "name"
			}
		case "Rules":
			if u.Rules == nil {
				u.Rules = []rules.Rule{}
			}
		case "Favorites":
			favorites, err := cleanFavorites(u.Favorites)
			if err != nil {
				return err
			}
			u.Favorites = favorites
		case "ShowDotfiles":
			if u.ShowDotfiles != nil {
				patterns, err := cleanShowDotfiles(u.ShowDotfiles)
				if err != nil {
					return err
				}
				u.ShowDotfiles = patterns
			}
		case "FolderColors":
			colors, err := cleanFolderColors(u.FolderColors)
			if err != nil {
				return err
			}
			u.FolderColors = colors
		}
	}

	if u.Fs == nil {
		scope := u.Scope
		scope = filepath.Join(baseScope, filepath.Join("/", scope))
		u.Fs = files.NewFs(afero.NewOsFs(), scope, followExternalSymlinks)
	}

	return nil
}

// cleanShowDotfiles trims the patterns, drops blanks and duplicates, and
// rejects malformed patterns and patterns that name a path.
func cleanShowDotfiles(patterns []string) ([]string, error) {
	cleaned := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		if strings.ContainsAny(pattern, "/\\") {
			return nil, fberrors.ErrInvalidRequestParams
		}

		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fberrors.ErrInvalidRequestParams
		}

		if _, ok := seen[pattern]; ok {
			continue
		}

		seen[pattern] = struct{}{}
		cleaned = append(cleaned, pattern)
	}

	if len(cleaned) > MaxShowDotfiles {
		return nil, fberrors.ErrInvalidRequestParams
	}

	return cleaned, nil
}

// cleanFavorites normalizes the favorite folders of a user: every entry becomes
// a cleaned absolute path, duplicates are dropped and the list is capped.
func cleanFavorites(favorites []string) ([]string, error) {
	cleaned := make([]string, 0, len(favorites))
	seen := make(map[string]struct{}, len(favorites))

	for _, favorite := range favorites {
		if !strings.HasPrefix(favorite, "/") {
			return nil, fberrors.ErrInvalidRequestParams
		}

		favorite = path.Clean(favorite)
		if _, ok := seen[favorite]; ok {
			continue
		}

		seen[favorite] = struct{}{}
		cleaned = append(cleaned, favorite)
	}

	if len(cleaned) > MaxFavorites {
		return nil, fberrors.ErrInvalidRequestParams
	}

	return cleaned, nil
}

// cleanFolderColors normalizes the folder colors of a user: paths become
// cleaned absolute paths and every color must be a "#rrggbb" hex value.
func cleanFolderColors(colors map[string]string) (map[string]string, error) {
	if len(colors) > MaxFolderColors {
		return nil, fberrors.ErrInvalidRequestParams
	}

	cleaned := make(map[string]string, len(colors))
	for folder, color := range colors {
		if !strings.HasPrefix(folder, "/") {
			return nil, fberrors.ErrInvalidRequestParams
		}

		color = strings.ToLower(color)
		if !folderColorPattern.MatchString(color) {
			return nil, fberrors.ErrInvalidRequestParams
		}

		cleaned[path.Clean(folder)] = color
	}

	return cleaned, nil
}

// FullPath gets the full path for a user's relative path.
func (u *User) FullPath(path string) string {
	return afero.FullBaseFsPath(files.BasePath(u.Fs), path)
}
