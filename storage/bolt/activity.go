package bolt

import (
	"errors"
	"regexp"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/filebrowser/filebrowser/v2/activity"
)

type activityBackend struct {
	db *storm.DB
}

func (b activityBackend) Save(e *activity.Entry) error {
	return b.db.Save(e)
}

func (b activityBackend) matchers(f activity.Filter) []q.Matcher {
	var matchers []q.Matcher
	if f.Username != "" {
		matchers = append(matchers, q.Eq("Username", f.Username))
	}
	if f.Action != "" {
		matchers = append(matchers, q.Eq("Action", f.Action))
	}
	if f.Query != "" {
		re := "(?i)" + regexp.QuoteMeta(f.Query)
		matchers = append(matchers, q.Or(q.Re("Path", re), q.Re("Dest", re), q.Re("Detail", re)))
	}

	return matchers
}

func (b activityBackend) Find(f activity.Filter) ([]*activity.Entry, int, error) {
	matchers := b.matchers(f)

	total, err := b.db.Select(matchers...).Count(&activity.Entry{})
	if err != nil {
		return nil, 0, err
	}

	var entries []*activity.Entry
	err = b.db.Select(matchers...).OrderBy("ID").Reverse().Skip(f.Offset).Limit(f.Limit).Find(&entries)
	if errors.Is(err, storm.ErrNotFound) {
		return []*activity.Entry{}, total, nil
	}
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (b activityBackend) Prune(before time.Time, keep int) error {
	if !before.IsZero() {
		err := b.db.Select(q.Lt("Time", before)).Delete(&activity.Entry{})
		if err != nil && !errors.Is(err, storm.ErrNotFound) {
			return err
		}
	}

	total, err := b.db.Count(&activity.Entry{})
	if err != nil || total <= keep {
		return err
	}

	var oldest []*activity.Entry
	err = b.db.Select().OrderBy("ID").Limit(total - keep).Find(&oldest)
	if err != nil {
		return err
	}
	for _, e := range oldest {
		if err := b.db.DeleteStruct(e); err != nil {
			return err
		}
	}

	return nil
}
