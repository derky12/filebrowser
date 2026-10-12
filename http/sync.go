package fbhttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	gopath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/syncthing"
)

// Sync modes exposed to the UI. They map to Syncthing folder types.
const (
	syncModeOneWay  = "oneway"  // this side sends, the other side only receives
	syncModeTwoWay  = "twoway"  // both sides send and receive
	syncModeReceive = "receive" // this side only receives
)

const syncFallbackPort = "22000"

// syncDefaultPort is the port assumed for device addresses entered without one.
func syncDefaultPort(server *settings.Server) string {
	if server.SyncthingPort == "" {
		return syncFallbackPort
	}

	return server.SyncthingPort
}

var (
	syncDeviceIDPattern = regexp.MustCompile(`^[A-Z2-7]{7}(-[A-Z2-7]{7}){7}$`)
	syncAddressPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.\-]*(:[0-9]{1,5})?$`)
	errSyncBadRequest   = errors.New("bad sync request")
)

type syncDeviceInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Connected bool   `json:"connected"`
	Paused    bool   `json:"paused"`
	// Cumulative bytes received/sent since the connection started. Speeds are
	// derived by the client from two polls.
	InBytes  int64 `json:"inBytes"`
	OutBytes int64 `json:"outBytes"`
}

type syncRemoteInfo struct {
	DeviceID   string  `json:"deviceID"`
	Connected  bool    `json:"connected"`
	Completion float64 `json:"completion"`
	NeedBytes  int64   `json:"needBytes"`
	NeedItems  int     `json:"needItems"`
}

type syncFolderInfo struct {
	ID          string           `json:"id"`
	Label       string           `json:"label"`
	Path        string           `json:"path"`
	External    bool             `json:"external"` // path is outside the user's scope (raw path shown)
	Mode        string           `json:"mode"`
	Paused      bool             `json:"paused"`
	State       string           `json:"state"`
	GlobalBytes int64            `json:"globalBytes"`
	InSyncBytes int64            `json:"inSyncBytes"`
	NeedBytes   int64            `json:"needBytes"`
	NeedFiles   int              `json:"needFiles"`
	Error       string           `json:"error,omitempty"`
	Remotes     []syncRemoteInfo `json:"remotes"`
}

type syncPendingDeviceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Time    string `json:"time"`
}

type syncPendingFolderInfo struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DeviceID   string `json:"deviceID"`
	DeviceName string `json:"deviceName"`
	Time       string `json:"time"`
}

type syncStatus struct {
	DeviceID       string                  `json:"deviceID"`
	DefaultPort    string                  `json:"defaultPort"`
	GUIPort        string                  `json:"guiPort,omitempty"`
	Time           int64                   `json:"time"` // server clock in ms, for transfer rates
	Devices        []syncDeviceInfo        `json:"devices"`
	Folders        []syncFolderInfo        `json:"folders"`
	PendingDevices []syncPendingDeviceInfo `json:"pendingDevices"`
	PendingFolders []syncPendingFolderInfo `json:"pendingFolders"`
}

// startSyncthingSetup applies the private-network options in the background,
// retrying until Syncthing is reachable (it may start after File Browser).
func startSyncthingSetup(server *settings.Server) {
	if server.SyncthingURL == "" || !server.SyncthingPrivate {
		return
	}

	client := syncthing.New(server.SyncthingURL, server.SyncthingAPIKey)
	go func() {
		for attempt := 0; attempt < 60; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := client.UsePrivateNetwork(ctx)
			cancel()
			if err == nil {
				log.Println("syncthing: private network options applied")
				return
			}
			time.Sleep(5 * time.Second)
		}
		log.Println("syncthing: could not apply private network options")
	}()
}

// withSync restricts a handler to administrators and to servers where the
// Syncthing integration is configured.
func withSync(fn func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error)) handleFunc {
	return withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if d.server.SyncthingURL == "" {
			return http.StatusNotFound, nil
		}

		return fn(w, r, d, syncthing.New(d.server.SyncthingURL, d.server.SyncthingAPIKey))
	})
}

func syncErrToStatus(err error) (int, error) {
	var apiErr *syncthing.APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.Status == http.StatusBadRequest:
		// Syncthing rejected the input (e.g. a device ID with a wrong check
		// digit); its message is meant for the user.
		return http.StatusBadRequest, syncBadRequest("%s", strings.Trim(apiErr.Body, `" `))
	case errors.Is(err, errSyncBadRequest):
		return http.StatusBadRequest, err
	case errors.Is(err, syncthing.ErrUnavailable):
		return http.StatusBadGateway, err
	default:
		return http.StatusInternalServerError, err
	}
}

func syncBadRequest(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errSyncBadRequest, fmt.Sprintf(format, args...))
}

func syncMode(folderType string) string {
	switch folderType {
	case syncthing.TypeSendOnly:
		return syncModeOneWay
	case syncthing.TypeReceiveOnly:
		return syncModeReceive
	default:
		return syncModeTwoWay
	}
}

func syncFolderType(mode string) (string, error) {
	switch mode {
	case syncModeOneWay:
		return syncthing.TypeSendOnly, nil
	case syncModeTwoWay:
		return syncthing.TypeSendReceive, nil
	case syncModeReceive:
		return syncthing.TypeReceiveOnly, nil
	default:
		return "", syncBadRequest("unknown mode %q", mode)
	}
}

// realScopeRoot returns the symlink-free absolute directory of the user scope.
func realScopeRoot(d *data) (string, error) {
	return filepath.EvalSymlinks(d.user.FullPath("/"))
}

// resolveSyncDir validates a virtual directory path chosen in the UI and
// returns its absolute on-disk path (as Syncthing sees it) and its cleaned
// virtual form. With create set, a missing directory is created first.
func resolveSyncDir(d *data, p string, create bool) (string, string, error) {
	p = slashClean(p)
	if p == "/" {
		return "", "", syncBadRequest("choose a folder inside the scope, not the scope root")
	}

	if !d.Check(p) {
		return "", "", fmt.Errorf("%w: path not allowed", os.ErrPermission)
	}

	if create {
		if err := d.user.Fs.MkdirAll(p, d.settings.DirMode); err != nil {
			return "", "", err
		}
	}

	info, err := d.user.Fs.Stat(p)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", syncBadRequest("%s is not a folder", p)
	}

	root, err := realScopeRoot(d)
	if err != nil {
		return "", "", err
	}
	real, err := filepath.EvalSymlinks(d.user.FullPath(p))
	if err != nil {
		return "", "", err
	}

	// Syncthing must see exactly the folder the user picked: no symlink may
	// appear anywhere on the way, otherwise it could sync something else.
	if real != filepath.Join(root, filepath.FromSlash(p)) {
		return "", "", syncBadRequest("%s is, or is inside, a symbolic link", p)
	}

	return real, p, nil
}

func newSyncFolderID() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return "fb-" + hex.EncodeToString(b), nil
}

// syncPathsOverlap reports whether one path equals, contains or is inside the
// other, which Syncthing does not handle well.
func syncPathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	sep := string(filepath.Separator)

	return a == b || strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep)
}

func checkSyncOverlap(existing []syncthing.Folder, real, ignoreID string) error {
	for _, f := range existing {
		if f.ID != ignoreID && syncPathsOverlap(f.Path, real) {
			return syncBadRequest("%s overlaps the already synced folder %s", real, f.Path)
		}
	}

	return nil
}

func trashVersioning(enabled bool) *syncthing.Versioning {
	if !enabled {
		return nil
	}

	return &syncthing.Versioning{Type: "trashcan", Params: map[string]string{"cleanoutDays": "30"}}
}

func hasDevice(devices []syncthing.Device, id string) bool {
	for _, dev := range devices {
		if dev.DeviceID == id {
			return true
		}
	}

	return false
}

var syncStatusHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	ctx := r.Context()

	myID, err := c.MyID(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}

	devices, err := c.Devices(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	folders, err := c.Folders(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	connections, err := c.Connections(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	pendingDevices, err := c.PendingDevices(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	pendingFolders, err := c.PendingFolders(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}

	out := syncStatus{
		DeviceID:       myID,
		DefaultPort:    syncDefaultPort(d.server),
		GUIPort:        d.server.SyncthingGUIPort,
		Time:           time.Now().UnixMilli(),
		Devices:        []syncDeviceInfo{},
		Folders:        []syncFolderInfo{},
		PendingDevices: []syncPendingDeviceInfo{},
		PendingFolders: []syncPendingFolderInfo{},
	}

	names := map[string]string{}
	for _, dev := range devices {
		names[dev.DeviceID] = dev.Name
		if dev.DeviceID == myID {
			continue
		}

		address := "dynamic"
		if len(dev.Addresses) > 0 {
			address = strings.Join(dev.Addresses, ", ")
		}
		out.Devices = append(out.Devices, syncDeviceInfo{
			ID:        dev.DeviceID,
			Name:      dev.Name,
			Address:   address,
			Connected: connections[dev.DeviceID].Connected,
			Paused:    dev.Paused,
			InBytes:   connections[dev.DeviceID].InBytesTotal,
			OutBytes:  connections[dev.DeviceID].OutBytesTotal,
		})
	}

	root, rootErr := realScopeRoot(d)
	for _, f := range folders {
		info := syncFolderInfo{
			ID:      f.ID,
			Label:   f.Label,
			Path:    f.Path,
			Mode:    syncMode(f.Type),
			Paused:  f.Paused,
			Remotes: []syncRemoteInfo{},
		}

		if rootErr == nil {
			if rel, relErr := filepath.Rel(root, f.Path); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				info.Path = gopath.Join("/", filepath.ToSlash(rel))
			} else {
				info.External = true
			}
		} else {
			info.External = true
		}

		if status, statusErr := c.FolderStatus(ctx, f.ID); statusErr == nil {
			info.State = status.State
			info.GlobalBytes = status.GlobalBytes
			info.InSyncBytes = status.InSyncBytes
			info.NeedBytes = status.NeedBytes
			info.NeedFiles = status.NeedFiles
			info.Error = status.Error
			if info.Error == "" && status.PullErrors > 0 {
				info.Error = fmt.Sprintf("%d items failed to sync", status.PullErrors)
			}
		}
		if f.Paused {
			info.State = "paused"
		}

		for _, ref := range f.Devices {
			if ref.DeviceID == myID {
				continue
			}

			remote := syncRemoteInfo{DeviceID: ref.DeviceID, Connected: connections[ref.DeviceID].Connected}
			if remote.Connected && !f.Paused {
				if completion, completionErr := c.Completion(ctx, f.ID, ref.DeviceID); completionErr == nil {
					remote.Completion = completion.Completion
					remote.NeedBytes = completion.NeedBytes
					remote.NeedItems = completion.NeedItems
				}
			}
			info.Remotes = append(info.Remotes, remote)
		}

		out.Folders = append(out.Folders, info)
	}

	for id, pending := range pendingDevices {
		out.PendingDevices = append(out.PendingDevices, syncPendingDeviceInfo{ID: id, Name: pending.Name, Address: pending.Address, Time: pending.Time})
	}
	for id, pending := range pendingFolders {
		for deviceID, offer := range pending.OfferedBy {
			out.PendingFolders = append(out.PendingFolders, syncPendingFolderInfo{
				ID: id, Label: offer.Label, DeviceID: deviceID, DeviceName: names[deviceID], Time: offer.Time,
			})
		}
	}

	sort.Slice(out.Devices, func(i, j int) bool { return out.Devices[i].Name < out.Devices[j].Name })
	sort.Slice(out.Folders, func(i, j int) bool { return out.Folders[i].Path < out.Folders[j].Path })
	sort.Slice(out.PendingDevices, func(i, j int) bool { return out.PendingDevices[i].ID < out.PendingDevices[j].ID })
	sort.Slice(out.PendingFolders, func(i, j int) bool {
		return out.PendingFolders[i].ID+out.PendingFolders[i].DeviceID < out.PendingFolders[j].ID+out.PendingFolders[j].DeviceID
	})

	return renderJSON(w, r, out)
})

func decodeSyncBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return syncBadRequest("empty request")
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v); err != nil {
		return syncBadRequest("invalid JSON: %v", err)
	}

	return nil
}

var syncDevicePostHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req struct {
		DeviceID string `json:"deviceID"`
		Name     string `json:"name"`
		Address  string `json:"address"`
	}
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}

	id := strings.ToUpper(strings.TrimSpace(req.DeviceID))
	if !syncDeviceIDPattern.MatchString(id) {
		return syncErrToStatus(syncBadRequest("invalid device ID"))
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return syncErrToStatus(syncBadRequest("a name of up to 100 characters is required"))
	}

	addresses := []string{"dynamic"}
	if address := strings.TrimSpace(req.Address); address != "" {
		if !syncAddressPattern.MatchString(address) {
			return syncErrToStatus(syncBadRequest("invalid address, use host or host:port"))
		}
		if !strings.Contains(address, ":") {
			address += ":" + syncDefaultPort(d.server)
		}
		addresses = []string{"tcp://" + address}
	}

	myID, err := c.MyID(r.Context())
	if err != nil {
		return syncErrToStatus(err)
	}
	if id == myID {
		return syncErrToStatus(syncBadRequest("that is this device's own ID"))
	}

	if err := c.PutDevice(r.Context(), syncthing.Device{DeviceID: id, Name: name, Addresses: addresses}); err != nil {
		return syncErrToStatus(err)
	}
	d.audit(r, "sync_device_add", "", "", name)

	return http.StatusNoContent, nil
})

var syncDeviceDeleteHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	id := strings.ToUpper(mux.Vars(r)["id"])
	if !syncDeviceIDPattern.MatchString(id) {
		return syncErrToStatus(syncBadRequest("invalid device ID"))
	}

	folders, err := c.Folders(r.Context())
	if err != nil {
		return syncErrToStatus(err)
	}
	for _, f := range folders {
		for _, ref := range f.Devices {
			if ref.DeviceID == id {
				return syncErrToStatus(syncBadRequest("device is still used by the synced folder %q, remove that first", f.Label))
			}
		}
	}

	if err := c.DeleteDevice(r.Context(), id); err != nil {
		return syncErrToStatus(err)
	}
	d.audit(r, "sync_device_remove", "", "", id)

	return http.StatusNoContent, nil
})

var syncFolderPostHandler = withSync(func(w http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req struct {
		Path       string   `json:"path"`
		DeviceIDs  []string `json:"deviceIDs"`
		Mode       string   `json:"mode"`
		Versioning *bool    `json:"versioning"`
	}
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}

	folderType, err := syncFolderType(req.Mode)
	if err != nil || req.Mode == syncModeReceive {
		return syncErrToStatus(syncBadRequest("mode must be %q or %q", syncModeOneWay, syncModeTwoWay))
	}
	if len(req.DeviceIDs) == 0 {
		return syncErrToStatus(syncBadRequest("choose at least one device"))
	}

	real, virtual, err := resolveSyncDir(d, req.Path, false)
	if err != nil {
		return syncPathErr(err)
	}

	ctx := r.Context()
	myID, err := c.MyID(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	devices, err := c.Devices(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	existing, err := c.Folders(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	if err := checkSyncOverlap(existing, real, ""); err != nil {
		return syncErrToStatus(err)
	}

	refs := []syncthing.DeviceRef{{DeviceID: myID}}
	for _, id := range req.DeviceIDs {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == myID || !hasDevice(devices, id) {
			return syncErrToStatus(syncBadRequest("unknown device %s, add it first", id))
		}
		refs = append(refs, syncthing.DeviceRef{DeviceID: id})
	}

	id, err := newSyncFolderID()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	folder := syncthing.Folder{
		ID:               id,
		Label:            gopath.Base(virtual),
		Path:             real,
		Type:             folderType,
		Devices:          refs,
		FSWatcherEnabled: true,
		FSWatcherDelayS:  3,
		RescanIntervalS:  3600,
		Versioning:       trashVersioning(req.Versioning == nil || *req.Versioning),
	}
	if err := c.PutFolder(ctx, folder); err != nil {
		return syncErrToStatus(err)
	}
	d.audit(r, "sync_share", virtual, "", req.Mode+" with "+strings.Join(req.DeviceIDs, ", "))

	return renderJSON(w, r, map[string]string{"id": id})
})

// syncPathErr maps path validation failures to HTTP statuses.
func syncPathErr(err error) (int, error) {
	switch {
	case errors.Is(err, errSyncBadRequest):
		return http.StatusBadRequest, err
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden, nil
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound, nil
	default:
		return http.StatusInternalServerError, err
	}
}

var syncFolderPatchHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req struct {
		Paused *bool `json:"paused"`
	}
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}
	if req.Paused == nil {
		return syncErrToStatus(syncBadRequest("nothing to change"))
	}

	if err := c.SetFolderPaused(r.Context(), mux.Vars(r)["id"], *req.Paused); err != nil {
		return syncErrToStatus(err)
	}
	if *req.Paused {
		d.audit(r, "sync_pause", "", "", mux.Vars(r)["id"])
	} else {
		d.audit(r, "sync_resume", "", "", mux.Vars(r)["id"])
	}

	return http.StatusNoContent, nil
})

var syncFolderDeleteHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	if err := c.DeleteFolder(r.Context(), mux.Vars(r)["id"]); err != nil {
		return syncErrToStatus(err)
	}
	d.audit(r, "sync_stop", "", "", mux.Vars(r)["id"])

	return http.StatusNoContent, nil
})

var syncPendingDeviceDeleteHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	id := strings.ToUpper(mux.Vars(r)["id"])
	if !syncDeviceIDPattern.MatchString(id) {
		return syncErrToStatus(syncBadRequest("invalid device ID"))
	}

	if err := c.DismissPendingDevice(r.Context(), id); err != nil {
		return syncErrToStatus(err)
	}

	return http.StatusNoContent, nil
})

var syncPendingFolderDeleteHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	folder, device := r.URL.Query().Get("folder"), strings.ToUpper(r.URL.Query().Get("device"))
	if folder == "" || !syncDeviceIDPattern.MatchString(device) {
		return syncErrToStatus(syncBadRequest("folder and device are required"))
	}

	if err := c.DismissPendingFolder(r.Context(), folder, device); err != nil {
		return syncErrToStatus(err)
	}

	return http.StatusNoContent, nil
})

// syncPendingAcceptHandler accepts a folder offered by another device, storing
// it in the chosen local directory (created when missing).
var syncPendingAcceptHandler = withSync(func(_ http.ResponseWriter, r *http.Request, d *data, c *syncthing.Client) (int, error) {
	var req struct {
		FolderID string `json:"folderID"`
		DeviceID string `json:"deviceID"`
		Path     string `json:"path"`
		Mode     string `json:"mode"`
	}
	if err := decodeSyncBody(r, &req); err != nil {
		return syncErrToStatus(err)
	}

	req.DeviceID = strings.ToUpper(strings.TrimSpace(req.DeviceID))
	if req.Mode != syncModeReceive && req.Mode != syncModeTwoWay {
		return syncErrToStatus(syncBadRequest("mode must be %q or %q", syncModeReceive, syncModeTwoWay))
	}
	folderType, _ := syncFolderType(req.Mode)

	ctx := r.Context()
	pending, err := c.PendingFolders(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	offer, ok := pending[req.FolderID].OfferedBy[req.DeviceID]
	if !ok {
		return http.StatusNotFound, nil
	}

	existing, err := c.Folders(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}
	for _, f := range existing {
		if f.ID == req.FolderID {
			return syncErrToStatus(syncBadRequest("this folder is already synced"))
		}
	}

	// Validate before creating anything on disk.
	if _, _, err := resolveSyncDirPlan(d, req.Path); err != nil {
		return syncPathErr(err)
	}
	real, virtual, err := resolveSyncDir(d, req.Path, true)
	if err != nil {
		return syncPathErr(err)
	}
	if err := checkSyncOverlap(existing, real, ""); err != nil {
		return syncErrToStatus(err)
	}

	myID, err := c.MyID(ctx)
	if err != nil {
		return syncErrToStatus(err)
	}

	label := offer.Label
	if label == "" {
		label = gopath.Base(virtual)
	}

	folder := syncthing.Folder{
		ID:               req.FolderID,
		Label:            label,
		Path:             real,
		Type:             folderType,
		Devices:          []syncthing.DeviceRef{{DeviceID: myID}, {DeviceID: req.DeviceID}},
		FSWatcherEnabled: true,
		FSWatcherDelayS:  3,
		RescanIntervalS:  3600,
		Versioning:       trashVersioning(true),
	}
	if err := c.PutFolder(ctx, folder); err != nil {
		return syncErrToStatus(err)
	}
	d.audit(r, "sync_accept", virtual, "", req.Mode+" from "+req.DeviceID)

	return http.StatusNoContent, nil
})

// resolveSyncDirPlan performs the checks of resolveSyncDir that do not need
// the directory to exist yet, so that a rejected request creates nothing.
func resolveSyncDirPlan(d *data, p string) (string, string, error) {
	p = slashClean(p)
	if p == "/" {
		return "", "", syncBadRequest("choose a folder inside the scope, not the scope root")
	}
	if !d.Check(p) {
		return "", "", fmt.Errorf("%w: path not allowed", os.ErrPermission)
	}

	// The deepest existing ancestor must not be, or sit behind, a symlink.
	root, err := realScopeRoot(d)
	if err != nil {
		return "", "", err
	}
	existing := p
	for existing != "/" {
		if _, statErr := d.user.Fs.Stat(existing); statErr == nil {
			break
		}
		existing = gopath.Dir(existing)
	}
	real, err := filepath.EvalSymlinks(d.user.FullPath(existing))
	if err != nil {
		return "", "", err
	}
	if real != filepath.Join(root, filepath.FromSlash(existing)) {
		return "", "", syncBadRequest("%s is, or is inside, a symbolic link", existing)
	}

	return real, p, nil
}
