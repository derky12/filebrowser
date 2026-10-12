package fbhttp

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	gopath "path"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// duBudget bounds one disk usage request: a folder not measured in this time
// is reported as partial instead of making the request hang.
var duBudget = 30 * time.Second

// duWorkers is how many folders are measured at the same time.
const duWorkers = 4

const (
	duCacheTTL     = 2 * time.Minute
	duCacheEntries = 5000
)

type duEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Size     int64  `json:"size"`
	Files    int    `json:"files"`
	Complete bool   `json:"complete"`
}

type duResponse struct {
	Path    string    `json:"path"`
	Total   int64     `json:"total"`
	Files   int       `json:"files"`
	Entries []duEntry `json:"entries"`
	Partial bool      `json:"partial"`
}

type duResult struct {
	size  int64
	files int
	at    time.Time
}

// duCache remembers complete measurements for a short while, so that going
// down into a folder just measured, or back up, is instant.
var duCache = struct {
	sync.Mutex
	m map[string]duResult
}{m: map[string]duResult{}}

func duCacheGet(real string) (duResult, bool) {
	duCache.Lock()
	defer duCache.Unlock()

	r, ok := duCache.m[real]
	if !ok || time.Since(r.at) > duCacheTTL {
		delete(duCache.m, real)
		return duResult{}, false
	}

	return r, true
}

func duCachePut(real string, r duResult) {
	duCache.Lock()
	defer duCache.Unlock()

	if len(duCache.m) >= duCacheEntries {
		for k, v := range duCache.m {
			if time.Since(v.at) > duCacheTTL {
				delete(duCache.m, k)
			}
		}
		if len(duCache.m) >= duCacheEntries {
			duCache.m = map[string]duResult{}
		}
	}
	r.at = time.Now()
	duCache.m[real] = r
}

// measure adds up the files under real. Symbolic links are counted for
// themselves and never followed. It reports false when the context ended first.
func measure(ctx context.Context, real string) (duResult, bool) {
	if r, ok := duCacheGet(real); ok {
		return r, true
	}

	var res duResult
	complete := true
	_ = filepath.WalkDir(real, func(_ string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			complete = false
			return ctx.Err()
		}
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			res.size += info.Size()
			res.files++
		}
		return nil
	})

	if complete {
		duCachePut(real, res)
	}

	return res, complete
}

var diskUsageHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	p := slashClean(r.URL.Path)
	if !d.Check(p) {
		return http.StatusForbidden, nil
	}

	info, err := d.user.Fs.Stat(p)
	if err != nil {
		return errToStatus(err), err
	}
	if !info.IsDir() {
		return http.StatusBadRequest, os.ErrInvalid
	}

	root, err := realScopeRoot(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	base, err := filepath.EvalSymlinks(d.user.FullPath(p))
	if err != nil {
		return errToStatus(err), err
	}
	// Measure only what is inside the scope.
	if rel, relErr := filepath.Rel(root, base); relErr != nil || rel == ".." || (len(rel) > 2 && rel[:3] == "../") {
		return http.StatusForbidden, nil
	}

	children, err := afero.ReadDir(d.user.Fs, p)
	if err != nil {
		return errToStatus(err), err
	}

	ctx, cancel := context.WithTimeout(r.Context(), duBudget)
	defer cancel()

	// Build the list first; the workers then each fill in their own element.
	entries := make([]duEntry, 0, len(children))
	reals := make([]string, 0, len(children))
	for _, child := range children {
		childPath := gopath.Join(p, child.Name())
		// Hiding is a display choice; the rules still apply.
		if !d.CheckRules(childPath) {
			continue
		}

		entry := duEntry{Name: child.Name(), Path: childPath, IsDir: child.IsDir(), Complete: true}
		if !child.IsDir() {
			entry.Size, entry.Files = child.Size(), 1
		}
		entries = append(entries, entry)
		reals = append(reals, filepath.Join(base, child.Name()))
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, duWorkers)
	for i := range entries {
		if !entries[i].IsDir {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(entry *duEntry, real string) {
			defer wg.Done()
			defer func() { <-sem }()

			res, complete := measure(ctx, real)
			entry.Size, entry.Files, entry.Complete = res.size, res.files, complete
		}(&entries[i], reals[i])
	}
	wg.Wait()

	out := duResponse{Path: p, Entries: entries}
	for _, e := range entries {
		out.Total += e.Size
		out.Files += e.Files
		if !e.Complete {
			out.Partial = true
		}
	}
	sort.SliceStable(out.Entries, func(i, j int) bool { return out.Entries[i].Size > out.Entries[j].Size })

	return renderJSON(w, r, out)
})
