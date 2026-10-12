// Package activity keeps a log of what users did: who changed which file, when
// and from where.
package activity

import (
	"sync"
	"time"
)

// Entry is one logged action.
type Entry struct {
	ID       uint      `storm:"id,increment" json:"id"`
	Time     time.Time `storm:"index" json:"time"`
	UserID   uint      `json:"userID"`
	Username string    `storm:"index" json:"username"`
	Action   string    `storm:"index" json:"action"`
	Path     string    `json:"path"`
	Dest     string    `json:"dest,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	IP       string    `json:"ip"`
}

// Filter selects entries; empty fields match everything.
type Filter struct {
	Username string
	Action   string
	Query    string // case-insensitive substring of the path, destination or detail
	Limit    int
	Offset   int
}

// Backend is the persistence of the log.
type Backend interface {
	Save(*Entry) error
	// Find returns the newest entries first and the total number of matches.
	Find(Filter) ([]*Entry, int, error)
	// Prune deletes entries older than before and, beyond keep entries, the oldest.
	Prune(before time.Time, keep int) error
}

// MaxEntries bounds the size of the log whatever the retention is.
const MaxEntries = 50000

// pruneEvery is how many additions pass between two prunes.
const pruneEvery = 200

// Storage logs and queries activity.
type Storage struct {
	back Backend

	// Retention is how long entries are kept; zero keeps them until MaxEntries.
	Retention time.Duration

	mux  sync.Mutex
	adds int
}

// NewStorage returns a Storage on top of back.
func NewStorage(back Backend) *Storage {
	return &Storage{back: back}
}

// Add stores an entry, stamping it with the current time when it has none.
func (s *Storage) Add(e *Entry) error {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if err := s.back.Save(e); err != nil {
		return err
	}

	s.mux.Lock()
	s.adds++
	due := s.adds%pruneEvery == 0
	s.mux.Unlock()

	if due {
		return s.Prune()
	}

	return nil
}

// Prune applies the retention.
func (s *Storage) Prune() error {
	var before time.Time
	if s.Retention > 0 {
		before = time.Now().Add(-s.Retention)
	}

	return s.back.Prune(before, MaxEntries)
}

// List returns a page of entries, newest first, and the total number of matches.
func (s *Storage) List(f Filter) ([]*Entry, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	return s.back.Find(f)
}
