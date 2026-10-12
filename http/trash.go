package fbhttp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"
)

// The trash lives in the user's scope: /.trash/<id>/data holds the deleted
// file or folder and /.trash/<id>/meta.json where it came from.
const trashDir = "/.trash"

var trashIDPattern = regexp.MustCompile(`^[0-9]{10,}-[0-9a-f]{8}$`)

type trashItem struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"` // where it was deleted from
	Name      string    `json:"name"`
	IsDir     bool      `json:"isDir"`
	Size      int64     `json:"size"`
	DeletedAt time.Time `json:"deletedAt"`
	By        string    `json:"by"`
}

func isInTrash(p string) bool {
	p = slashClean(p)
	return p == trashDir || strings.HasPrefix(p, trashDir+"/")
}

func newTrashID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b)), nil
}

// treeSizeBudget bounds how long measuring a folder may take; a huge tree is
// then only measured in part (its size is an estimate that is too small).
const treeSizeBudget = 10 * time.Second

// treeSize returns the size of a file, or of everything inside a folder.
func treeSize(fs afero.Fs, p string, info os.FileInfo) int64 {
	if !info.IsDir() {
		return info.Size()
	}

	var total int64
	deadline := time.Now().Add(treeSizeBudget)
	_ = afero.Walk(fs, p, func(_ string, i os.FileInfo, err error) error {
		if time.Now().After(deadline) {
			return errTreeSizeBudget
		}
		if err == nil && !i.IsDir() {
			total += i.Size()
		}
		return nil
	})

	return total
}

var errTreeSizeBudget = errors.New("size measurement took too long")

// readTrash lists the items in the trash, oldest first.
func readTrash(d *data) []trashItem {
	entries, err := afero.ReadDir(d.user.Fs, trashDir)
	if err != nil {
		return nil
	}

	items := make([]trashItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !trashIDPattern.MatchString(entry.Name()) {
			continue
		}

		raw, err := afero.ReadFile(d.user.Fs, trashDir+"/"+entry.Name()+"/meta.json")
		if err != nil {
			continue
		}
		var item trashItem
		if json.Unmarshal(raw, &item) != nil || item.ID != entry.Name() {
			continue
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].DeletedAt.Before(items[j].DeletedAt) })
	return items
}

func removeTrashItem(d *data, id string) error {
	return d.user.Fs.RemoveAll(trashDir + "/" + id)
}

// enforceTrashLimits removes the items that are older than the configured
// number of days and then the oldest ones until the trash fits its size limit.
func enforceTrashLimits(d *data) []trashItem {
	cfg := d.settings.TrashConfig()
	items := readTrash(d)

	var keep []trashItem
	var total int64
	for _, item := range items {
		if cfg.Days > 0 && time.Since(item.DeletedAt) > time.Duration(cfg.Days)*24*time.Hour {
			if err := removeTrashItem(d, item.ID); err != nil {
				log.Printf("trash: could not remove expired %s: %v", item.ID, err)
				keep = append(keep, item)
				total += item.Size
			}
			continue
		}
		keep = append(keep, item)
		total += item.Size
	}

	for cfg.MaxBytes > 0 && total > cfg.MaxBytes && len(keep) > 0 {
		oldest := keep[0]
		if err := removeTrashItem(d, oldest.ID); err != nil {
			log.Printf("trash: could not make room by removing %s: %v", oldest.ID, err)
			break
		}
		total -= oldest.Size
		keep = keep[1:]
	}

	return keep
}

// moveToTrash moves p into the trash. It reports false, without touching
// anything, when the item is larger than the whole trash may be.
func moveToTrash(d *data, p string) (bool, error) {
	p = slashClean(p)
	cfg := d.settings.TrashConfig()

	var info os.FileInfo
	var err error
	if l, ok := d.user.Fs.(afero.Lstater); ok {
		info, _, err = l.LstatIfPossible(p)
	} else {
		info, err = d.user.Fs.Stat(p)
	}
	if err != nil {
		return false, err
	}

	size := treeSize(d.user.Fs, p, info)
	if cfg.MaxBytes > 0 && size > cfg.MaxBytes {
		return false, nil
	}

	id, err := newTrashID()
	if err != nil {
		return false, err
	}
	dir := trashDir + "/" + id

	if err := d.user.Fs.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}

	meta, err := json.Marshal(trashItem{
		ID: id, Path: p, Name: info.Name(), IsDir: info.IsDir(), Size: size,
		DeletedAt: time.Now(), By: d.user.Username,
	})
	if err == nil {
		err = afero.WriteFile(d.user.Fs, dir+"/meta.json", meta, 0o600)
	}
	if err == nil {
		err = d.user.Fs.Rename(p, dir+"/data")
	}
	if err != nil {
		_ = d.user.Fs.RemoveAll(dir)
		return false, err
	}

	enforceTrashLimits(d)
	return true, nil
}

// withTrash restricts a handler to users who may delete and where the trash
// is enabled.
func withTrash(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		return fn(w, r, d)
	})
}

type trashListing struct {
	Enabled    bool        `json:"enabled"`
	MaxBytes   int64       `json:"maxBytes"`
	Days       int         `json:"days"`
	TotalBytes int64       `json:"totalBytes"`
	Items      []trashItem `json:"items"`
}

var trashListHandler = withTrash(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	cfg := d.settings.TrashConfig()
	items := enforceTrashLimits(d)

	out := trashListing{Enabled: cfg.Enabled, MaxBytes: cfg.MaxBytes, Days: cfg.Days, Items: []trashItem{}}
	for i := len(items) - 1; i >= 0; i-- { // newest first
		out.Items = append(out.Items, items[i])
		out.TotalBytes += items[i].Size
	}

	return renderJSON(w, r, out)
})

var trashRestoreHandler = withTrash(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Create {
		return http.StatusForbidden, nil
	}

	id := mux.Vars(r)["id"]
	if !trashIDPattern.MatchString(id) {
		return http.StatusBadRequest, errors.New("invalid trash id")
	}

	raw, err := afero.ReadFile(d.user.Fs, trashDir+"/"+id+"/meta.json")
	if err != nil {
		return http.StatusNotFound, nil
	}
	var item trashItem
	if json.Unmarshal(raw, &item) != nil || isInTrash(item.Path) {
		return http.StatusInternalServerError, errors.New("damaged trash entry")
	}

	dst := slashClean(item.Path)
	if !d.Check(dst) {
		return http.StatusForbidden, nil
	}

	if _, err := d.user.Fs.Stat(dst); err == nil {
		if r.URL.Query().Get("rename") != "true" {
			return http.StatusConflict, nil
		}
		dst = addVersionSuffix(dst, d.user.Fs)
	}

	if err := d.user.Fs.MkdirAll(parentDir(dst), d.settings.DirMode); err != nil {
		return errToStatus(err), err
	}
	if err := d.user.Fs.Rename(trashDir+"/"+id+"/data", dst); err != nil {
		return errToStatus(err), err
	}
	_ = removeTrashItem(d, id)

	d.audit(r, "restore", dst, "", item.Name)
	return renderJSON(w, r, map[string]string{"path": dst})
})

var trashDeleteHandler = withTrash(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	id := mux.Vars(r)["id"]
	if !trashIDPattern.MatchString(id) {
		return http.StatusBadRequest, errors.New("invalid trash id")
	}

	raw, _ := afero.ReadFile(d.user.Fs, trashDir+"/"+id+"/meta.json")
	var item trashItem
	_ = json.Unmarshal(raw, &item)

	if err := removeTrashItem(d, id); err != nil {
		return errToStatus(err), err
	}

	d.audit(r, "purge", item.Path, "", item.Name)
	return http.StatusNoContent, nil
})

var trashEmptyHandler = withTrash(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	items := readTrash(d)
	for _, item := range items {
		if err := removeTrashItem(d, item.ID); err != nil {
			return errToStatus(err), err
		}
	}

	d.audit(r, "purge", "", "", fmt.Sprintf("emptied the trash (%d items)", len(items)))
	return http.StatusNoContent, nil
})
