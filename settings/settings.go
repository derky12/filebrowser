package settings

import (
	"crypto/rand"
	"io/fs"
	"log"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/rules"
)

const DefaultUsersHomeBasePath = "/users"
const DefaultLogoutPage = "/login"
const DefaultMinimumPasswordLength = 12
const DefaultFileMode = 0640
const DefaultDirMode = 0750

// AuthMethod describes an authentication method.
type AuthMethod string

// Settings contain the main settings of the application.
type Settings struct {
	Key                   []byte              `json:"key"`
	Signup                bool                `json:"signup"`
	HideLoginButton       bool                `json:"hideLoginButton"`
	CreateUserDir         bool                `json:"createUserDir"`
	UserHomeBasePath      string              `json:"userHomeBasePath"`
	Defaults              UserDefaults        `json:"defaults"`
	AuthMethod            AuthMethod          `json:"authMethod"`
	LogoutPage            string              `json:"logoutPage"`
	Branding              Branding            `json:"branding"`
	Tus                   Tus                 `json:"tus"`
	Commands              map[string][]string `json:"commands"`
	Shell                 []string            `json:"shell"`
	Rules                 []rules.Rule        `json:"rules"`
	MinimumPasswordLength uint                `json:"minimumPasswordLength"`
	FileMode              fs.FileMode         `json:"fileMode"`
	DirMode               fs.FileMode         `json:"dirMode"`
	HideDotfiles          bool                `json:"hideDotfiles"`
	// Trash is nil until it is configured; TrashConfig then gives the defaults.
	Trash *Trash `json:"trash,omitempty"`
	// WebDAVShares are the folders shared over WebDAV, see the webdav server.
	WebDAVShares []WebDAVShare `json:"webdavShares,omitempty"`
}

// WebDAVShare is a folder served over WebDAV under its name.
type WebDAVShare struct {
	Name     string    `json:"name"`     // the first path element of its URL
	Dir      string    `json:"dir"`      // folder, relative to the server root, "/"-separated
	Path     string    `json:"path"`     // where it was shared from in the sharer's scope (display only)
	ReadOnly bool      `json:"readOnly"` // no changes through WebDAV
	By       string    `json:"by"`
	Created  time.Time `json:"created"`
}

// Trash configures the recycle bin that deleted files are moved to.
type Trash struct {
	Enabled  bool  `json:"enabled"`
	MaxBytes int64 `json:"maxBytes"` // total size kept, 0 is unlimited
	Days     int   `json:"days"`     // how long items are kept, 0 until space is needed
}

// DefaultTrash is used until the trash is configured.
var DefaultTrash = Trash{Enabled: true, MaxBytes: 10 << 30, Days: 30}

// TrashConfig returns the configured trash, or the defaults.
func (s *Settings) TrashConfig() Trash {
	if s.Trash == nil {
		return DefaultTrash
	}

	return *s.Trash
}

// GetRules implements rules.Provider.
func (s *Settings) GetRules() []rules.Rule {
	return s.Rules
}

// Server specific settings.
type Server struct {
	Root                   string `json:"root"`
	BaseURL                string `json:"baseURL"`
	Socket                 string `json:"socket"`
	TLSKey                 string `json:"tlsKey"`
	TLSCert                string `json:"tlsCert"`
	Port                   string `json:"port"`
	Address                string `json:"address"`
	Log                    string `json:"log"`
	EnableThumbnails       bool   `json:"enableThumbnails"`
	ResizePreview          bool   `json:"resizePreview"`
	EnableExec             bool   `json:"enableExec"`
	TypeDetectionByHeader  bool   `json:"typeDetectionByHeader"`
	ImageResolutionCal     bool   `json:"imageResolutionCalculation"`
	AuthHook               string `json:"authHook"`
	TokenExpirationTime    string `json:"tokenExpirationTime"`
	FollowExternalSymlinks bool   `json:"followExternalSymlinks"`

	// Syncthing integration. Runtime-only: set from flags/environment, never
	// persisted nor sent to clients.
	SyncthingURL     string `json:"-"`
	SyncthingAPIKey  string `json:"-"`
	SyncthingPrivate bool   `json:"-"`
	// SyncthingPort is the sync port other devices listen on, used when a
	// device address is entered without one.
	SyncthingPort string `json:"-"`

	// WebDAV server, see http/webdav.go. Runtime-only, like the Syncthing settings.
	WebDAV            bool   `json:"-"`
	WebDAVPort        string `json:"-"` // port the server listens on inside the container
	WebDAVHostPort    string `json:"-"` // port it is published on, only to show URLs
	WebDAVUser        string `json:"-"`
	WebDAVPassword    string `json:"-"`
	WebDAVPrefix      string `json:"-"` // URL path prefix when it is served under a sub-path of a proxy
	WebDAVPublicURL   string `json:"-"` // where clients reach it, when that is not host:port
	WebDAVBehindProxy bool   `json:"-"` // trust X-Forwarded-For from a reverse proxy

	// ActivityDays is how many days the activity log is kept; 0 turns it off.
	// Runtime-only, like the Syncthing settings.
	ActivityDays int `json:"-"`

	// CaseInsensitiveFs is detected from Root at startup rather than
	// configured, and tells the rule checker to match paths case-insensitively.
	// It is never persisted.
	CaseInsensitiveFs bool `json:"-"`
}

// Clean cleans any variables that might need cleaning.
func (s *Server) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

func (s *Server) GetTokenExpirationTime(fallback time.Duration) time.Duration {
	if s.TokenExpirationTime == "" {
		return fallback
	}

	duration, err := time.ParseDuration(s.TokenExpirationTime)
	if err != nil {
		log.Printf("[WARN] Failed to parse tokenExpirationTime: %v", err)
		return fallback
	}
	return duration
}

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
