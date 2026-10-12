package fbhttp

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tomasen/realip"

	"github.com/filebrowser/filebrowser/v2/activity"
	"github.com/filebrowser/filebrowser/v2/users"
)

// auditNamer describes, from the request, the action to log.
type auditNamer func(r *http.Request) (action, path, dest, detail string)

// audit records an action of the current user. It never fails the request:
// problems are only logged.
func (d *data) audit(r *http.Request, action, path, dest, detail string) {
	d.auditAs(r, d.user, action, path, dest, detail)
}

func (d *data) auditAs(r *http.Request, user *users.User, action, path, dest, detail string) {
	if d.store == nil || d.store.Activity == nil || d.server == nil || d.server.ActivityDays <= 0 {
		return
	}

	entry := &activity.Entry{
		Time:   time.Now(),
		Action: action,
		Path:   path,
		Dest:   dest,
		Detail: detail,
		IP:     realip.FromRequest(r),
	}
	if user != nil {
		entry.UserID = user.ID
		entry.Username = user.Username
	}

	if err := d.store.Activity.Add(entry); err != nil {
		log.Printf("activity: could not record %s %s: %v", action, path, err)
	}
}

// audited wraps a handler so that its successful runs are logged.
func audited(name auditNamer, fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		status, err := fn(w, r, d)
		if err == nil && (status == 0 || (status >= 200 && status < 300)) {
			action, path, dest, detail := name(r)
			d.audit(r, action, path, dest, detail)
		}

		return status, err
	}
}

// auditFixed names a handler whose action and path never vary.
func auditFixed(action string) auditNamer {
	return func(r *http.Request) (string, string, string, string) {
		return action, r.URL.Path, "", ""
	}
}

// auditPatch names the rename, move and copy variants of the resource PATCH.
func auditPatch(r *http.Request) (string, string, string, string) {
	src := slashClean(r.URL.Path)
	dst, err := url.QueryUnescape(r.URL.Query().Get("destination"))
	if err != nil {
		dst = r.URL.Query().Get("destination")
	}
	dst = slashClean(dst)

	action := r.URL.Query().Get("action")
	if action == "rename" {
		action = "move"
		if parentDir(src) == parentDir(dst) {
			action = "rename"
		}
	}

	return action, src, dst, ""
}

func parentDir(p string) string {
	for i := len(p) - 1; i > 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}

	return "/"
}

// auditPost names the creation of a folder or the upload of a file.
func auditPost(r *http.Request) (string, string, string, string) {
	if len(r.URL.Path) > 1 && r.URL.Path[len(r.URL.Path)-1] == '/' {
		return "mkdir", r.URL.Path, "", ""
	}

	return "upload", r.URL.Path, "", ""
}

type activityResponse struct {
	Items []*activity.Entry `json:"items"`
	Total int               `json:"total"`
}

var activityGetHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if d.store.Activity == nil {
		return http.StatusNotFound, nil
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	items, total, err := d.store.Activity.List(activity.Filter{
		Username: q.Get("user"),
		Action:   q.Get("action"),
		Query:    q.Get("q"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, activityResponse{Items: items, Total: total})
})

// filesizeLabel formats a byte count for the log, e.g. "3.2 MiB".
func filesizeLabel(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}

	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}

	return strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64) + " " + string("KMGTPE"[exp]) + "iB"
}
