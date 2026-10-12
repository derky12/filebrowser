package fbhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/syncthing"
	"github.com/filebrowser/filebrowser/v2/users"
)

const (
	testSelfID          = "AAAAAAA-BBBBBBB-CCCCCCC-DDDDDDD-EEEEEEE-FFFFFFF-GGGGGGG-HHHHHHH"
	testBadCheckDigitID = "ZZZZZZZ-ZZZZZZZ-ZZZZZZZ-ZZZZZZZ-ZZZZZZZ-ZZZZZZZ-ZZZZZZZ-ZZZZZZZ"
	testRemoteID        = "2222222-3333333-4444444-5555555-6666666-7777777-AAAAAAA-BBBBBBB"
)

// fakeSyncthing is an in-memory stand-in for the Syncthing REST API.
type fakeSyncthing struct {
	mu             sync.Mutex
	devices        map[string]syncthing.Device
	folders        map[string]syncthing.Folder
	pendingDevices map[string]syncthing.PendingDevice
	pendingFolders map[string]syncthing.PendingFolder
	dismissed      []string
	srv            *httptest.Server
}

func newFakeSyncthing(t *testing.T) *fakeSyncthing {
	t.Helper()
	f := &fakeSyncthing{
		devices:        map[string]syncthing.Device{testSelfID: {DeviceID: testSelfID, Name: "self"}},
		folders:        map[string]syncthing.Folder{},
		pendingDevices: map[string]syncthing.PendingDevice{},
		pendingFolders: map[string]syncthing.PendingFolder{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSyncthing) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.Header.Get("X-API-Key") != "test-key" {
		http.Error(w, "bad key", http.StatusForbidden)
		return
	}

	write := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }
	body, _ := io.ReadAll(r.Body)
	id := strings.TrimPrefix(r.URL.Path[strings.LastIndex(r.URL.Path, "/"):], "/")

	switch {
	case r.URL.Path == "/rest/system/status":
		write(map[string]string{"myID": testSelfID})
	case r.URL.Path == "/rest/system/connections":
		write(map[string]interface{}{"connections": map[string]syncthing.Connection{testRemoteID: {Connected: true}}})
	case r.URL.Path == "/rest/config/devices" && r.Method == http.MethodGet:
		list := []syncthing.Device{}
		for _, d := range f.devices {
			list = append(list, d)
		}
		write(list)
	case strings.HasPrefix(r.URL.Path, "/rest/config/devices/") && r.Method == http.MethodPut && id == testBadCheckDigitID:
		http.Error(w, `"bad id": check digit incorrect`, http.StatusBadRequest)
	case strings.HasPrefix(r.URL.Path, "/rest/config/devices/") && r.Method == http.MethodPut:
		var d syncthing.Device
		_ = json.Unmarshal(body, &d)
		f.devices[id] = d
	case strings.HasPrefix(r.URL.Path, "/rest/config/devices/") && r.Method == http.MethodDelete:
		delete(f.devices, id)
	case r.URL.Path == "/rest/config/folders" && r.Method == http.MethodGet:
		list := []syncthing.Folder{}
		for _, fo := range f.folders {
			list = append(list, fo)
		}
		write(list)
	case strings.HasPrefix(r.URL.Path, "/rest/config/folders/") && r.Method == http.MethodPut:
		var fo syncthing.Folder
		_ = json.Unmarshal(body, &fo)
		f.folders[id] = fo
	case strings.HasPrefix(r.URL.Path, "/rest/config/folders/") && r.Method == http.MethodPatch:
		fo := f.folders[id]
		var patch struct{ Paused bool }
		_ = json.Unmarshal(body, &patch)
		fo.Paused = patch.Paused
		f.folders[id] = fo
	case strings.HasPrefix(r.URL.Path, "/rest/config/folders/") && r.Method == http.MethodDelete:
		delete(f.folders, id)
	case r.URL.Path == "/rest/db/status":
		write(syncthing.FolderStatus{State: "idle", GlobalBytes: 10, InSyncBytes: 10})
	case r.URL.Path == "/rest/db/completion":
		write(syncthing.Completion{Completion: 100})
	case r.URL.Path == "/rest/cluster/pending/devices" && r.Method == http.MethodGet:
		write(f.pendingDevices)
	case r.URL.Path == "/rest/cluster/pending/folders" && r.Method == http.MethodGet:
		write(f.pendingFolders)
	case strings.HasPrefix(r.URL.Path, "/rest/cluster/pending/") && r.Method == http.MethodDelete:
		f.dismissed = append(f.dismissed, r.URL.Path+"?"+r.URL.RawQuery)
	default:
		http.NotFound(w, r)
	}
}

type syncEnv struct {
	t         *testing.T
	scope     string // real, symlink-free scope directory
	st        *storage.Storage
	key       []byte
	server    *settings.Server
	fake      *fakeSyncthing
	adminPerm users.Permissions
}

func newSyncEnv(t *testing.T) *syncEnv {
	t.Helper()
	scope, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	perm := users.Permissions{Admin: true, Create: true, Modify: true, Rename: true, Download: true}
	fake := newFakeSyncthing(t)
	return &syncEnv{
		t:         t,
		scope:     scope,
		st:        scopedUserStorage(t, scope, perm, key),
		key:       key,
		server:    &settings.Server{SyncthingURL: fake.srv.URL, SyncthingAPIKey: "test-key"},
		fake:      fake,
		adminPerm: perm,
	}
}

func (e *syncEnv) call(h handleFunc, perm users.Permissions, method, target, body string, vars map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	req, _ := http.NewRequest(method, target, strings.NewReader(body))
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	req.Header.Set("X-Auth", signToken(e.t, perm, e.key))
	rec := httptest.NewRecorder()
	handle(h, "", e.st, e.server).ServeHTTP(rec, req)
	return rec
}

func (e *syncEnv) mkdir(rel string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Join(e.scope, rel), 0o755); err != nil {
		e.t.Fatal(err)
	}
}

func (e *syncEnv) addRemote() {
	e.fake.devices[testRemoteID] = syncthing.Device{DeviceID: testRemoteID, Name: "remote"}
}

func TestSyncRequiresAdminAndConfiguration(t *testing.T) {
	e := newSyncEnv(t)
	nonAdmin := users.Permissions{Create: true}
	// The stored user is an admin; a non-admin store user is needed for 403.
	st := scopedUserStorage(t, e.scope, nonAdmin, e.key)
	e.st = st
	if rec := e.call(syncStatusHandler, nonAdmin, http.MethodGet, "/api/sync", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: status=%d, want 403", rec.Code)
	}

	e = newSyncEnv(t)
	e.server.SyncthingURL = ""
	if rec := e.call(syncStatusHandler, e.adminPerm, http.MethodGet, "/api/sync", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("disabled: status=%d, want 404", rec.Code)
	}
}

func TestSyncCreateFolder(t *testing.T) {
	e := newSyncEnv(t)
	e.mkdir("docs")
	e.addRemote()

	body := `{"path":"/docs","deviceIDs":["` + testRemoteID + `"],"mode":"oneway"}`
	rec := e.call(syncFolderPostHandler, e.adminPerm, http.MethodPost, "/api/sync/folders", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}

	var resp struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	folder, ok := e.fake.folders[resp.ID]
	if !ok || !strings.HasPrefix(resp.ID, "fb-") {
		t.Fatalf("folder %q not created: %v", resp.ID, e.fake.folders)
	}
	if folder.Path != filepath.Join(e.scope, "docs") || folder.Type != syncthing.TypeSendOnly || folder.Label != "docs" {
		t.Fatalf("unexpected folder %+v", folder)
	}
	if len(folder.Devices) != 2 || folder.Devices[0].DeviceID != testSelfID || folder.Devices[1].DeviceID != testRemoteID {
		t.Fatalf("unexpected devices %+v", folder.Devices)
	}
	if folder.Versioning == nil || folder.Versioning.Type != "trashcan" {
		t.Fatalf("expected trashcan versioning, got %+v", folder.Versioning)
	}
}

func TestSyncCreateFolderTwoWayWithoutVersioning(t *testing.T) {
	e := newSyncEnv(t)
	e.mkdir("docs")
	e.addRemote()

	body := `{"path":"/docs","deviceIDs":["` + testRemoteID + `"],"mode":"twoway","versioning":false}`
	rec := e.call(syncFolderPostHandler, e.adminPerm, http.MethodPost, "/api/sync/folders", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	for _, f := range e.fake.folders {
		if f.Type != syncthing.TypeSendReceive || f.Versioning != nil {
			t.Fatalf("unexpected folder %+v", f)
		}
	}
}

func TestSyncCreateFolderRejections(t *testing.T) {
	e := newSyncEnv(t)
	e.mkdir("docs/inner")
	e.mkdir("elsewhere")
	e.addRemote()
	if err := os.WriteFile(filepath.Join(e.scope, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(e.scope, "elsewhere"), filepath.Join(e.scope, "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	remote := `"deviceIDs":["` + testRemoteID + `"]`

	// Existing synced folder, to test overlap detection.
	e.fake.folders["x"] = syncthing.Folder{ID: "x", Path: filepath.Join(e.scope, "docs")}

	cases := map[string]struct {
		body string
		want int
	}{
		"scope root":      {`{"path":"/",` + remote + `,"mode":"oneway"}`, http.StatusBadRequest},
		"missing":         {`{"path":"/nope",` + remote + `,"mode":"oneway"}`, http.StatusNotFound},
		"file not folder": {`{"path":"/file.txt",` + remote + `,"mode":"oneway"}`, http.StatusBadRequest},
		"symlink":         {`{"path":"/link",` + remote + `,"mode":"oneway"}`, http.StatusBadRequest},
		"overlap inside":  {`{"path":"/docs/inner",` + remote + `,"mode":"oneway"}`, http.StatusBadRequest},
		"overlap same":    {`{"path":"/docs",` + remote + `,"mode":"oneway"}`, http.StatusBadRequest},
		"unknown device":  {`{"path":"/elsewhere","deviceIDs":["` + strings.Repeat("Z", 7) + `"],"mode":"oneway"}`, http.StatusBadRequest},
		"no devices":      {`{"path":"/elsewhere","deviceIDs":[],"mode":"oneway"}`, http.StatusBadRequest},
		"receive on send": {`{"path":"/elsewhere",` + remote + `,"mode":"receive"}`, http.StatusBadRequest},
		"bad mode":        {`{"path":"/elsewhere",` + remote + `,"mode":"x"}`, http.StatusBadRequest},
		"traversal":       {`{"path":"/../../etc",` + remote + `,"mode":"oneway"}`, http.StatusNotFound},
		"invalid json":    {`{`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			before := len(e.fake.folders)
			rec := e.call(syncFolderPostHandler, e.adminPerm, http.MethodPost, "/api/sync/folders", tc.body, nil)
			if rec.Code != tc.want {
				t.Fatalf("status=%d, want %d (%q)", rec.Code, tc.want, rec.Body.String())
			}
			if len(e.fake.folders) != before {
				t.Fatal("a folder was created despite the rejection")
			}
		})
	}
}

func TestSyncStatusMapsPathsAndPending(t *testing.T) {
	e := newSyncEnv(t)
	e.addRemote()
	e.fake.folders["fb-1"] = syncthing.Folder{
		ID: "fb-1", Label: "docs", Path: filepath.Join(e.scope, "docs"), Type: syncthing.TypeSendOnly,
		Devices: []syncthing.DeviceRef{{DeviceID: testSelfID}, {DeviceID: testRemoteID}},
	}
	e.fake.folders["ext"] = syncthing.Folder{ID: "ext", Label: "ext", Path: "/somewhere/else", Type: syncthing.TypeSendReceive}
	e.fake.pendingDevices["PEND123"] = syncthing.PendingDevice{Name: "laptop", Address: "1.2.3.4:22000"}
	e.fake.pendingFolders["fb-2"] = syncthing.PendingFolder{OfferedBy: map[string]syncthing.PendingOffer{testRemoteID: {Label: "photos"}}}

	rec := e.call(syncStatusHandler, e.adminPerm, http.MethodGet, "/api/sync", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var got syncStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.DefaultPort != "22000" {
		t.Fatalf("defaultPort=%q, want 22000", got.DefaultPort)
	}
	if got.DeviceID != testSelfID || len(got.Devices) != 1 || got.Devices[0].Name != "remote" || !got.Devices[0].Connected {
		t.Fatalf("devices: %+v", got)
	}
	byID := map[string]syncFolderInfo{}
	for _, f := range got.Folders {
		byID[f.ID] = f
	}
	if f := byID["fb-1"]; f.Path != "/docs" || f.External || f.Mode != syncModeOneWay || f.State != "idle" || len(f.Remotes) != 1 || f.Remotes[0].Completion != 100 {
		t.Fatalf("fb-1: %+v", f)
	}
	if f := byID["ext"]; !f.External || f.Path != "/somewhere/else" {
		t.Fatalf("ext: %+v", f)
	}
	if len(got.PendingDevices) != 1 || got.PendingDevices[0].Name != "laptop" {
		t.Fatalf("pending devices: %+v", got.PendingDevices)
	}
	if len(got.PendingFolders) != 1 || got.PendingFolders[0].Label != "photos" || got.PendingFolders[0].DeviceName != "remote" {
		t.Fatalf("pending folders: %+v", got.PendingFolders)
	}
}

func TestSyncAcceptPendingFolder(t *testing.T) {
	e := newSyncEnv(t)
	e.addRemote()
	e.fake.pendingFolders["fb-9"] = syncthing.PendingFolder{OfferedBy: map[string]syncthing.PendingOffer{testRemoteID: {Label: "photos"}}}

	body := `{"folderID":"fb-9","deviceID":"` + testRemoteID + `","path":"/incoming/photos","mode":"receive"}`
	rec := e.call(syncPendingAcceptHandler, e.adminPerm, http.MethodPost, "/api/sync/pending/folders/accept", body, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	folder := e.fake.folders["fb-9"]
	if folder.Path != filepath.Join(e.scope, "incoming", "photos") || folder.Type != syncthing.TypeReceiveOnly || folder.Label != "photos" {
		t.Fatalf("unexpected folder %+v", folder)
	}
	if info, err := os.Stat(folder.Path); err != nil || !info.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestSyncAcceptPendingFolderRejections(t *testing.T) {
	e := newSyncEnv(t)
	e.addRemote()
	e.mkdir("real")
	if err := os.Symlink(filepath.Join(e.scope, "real"), filepath.Join(e.scope, "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	e.fake.pendingFolders["fb-9"] = syncthing.PendingFolder{OfferedBy: map[string]syncthing.PendingOffer{testRemoteID: {Label: "photos"}}}

	cases := map[string]struct {
		body string
		want int
	}{
		"scope root":      {`{"folderID":"fb-9","deviceID":"` + testRemoteID + `","path":"/","mode":"receive"}`, http.StatusBadRequest},
		"through symlink": {`{"folderID":"fb-9","deviceID":"` + testRemoteID + `","path":"/link/new","mode":"receive"}`, http.StatusBadRequest},
		"not offered":     {`{"folderID":"nope","deviceID":"` + testRemoteID + `","path":"/x","mode":"receive"}`, http.StatusNotFound},
		"wrong device":    {`{"folderID":"fb-9","deviceID":"` + strings.Repeat("A", 7) + `","path":"/x","mode":"receive"}`, http.StatusNotFound},
		"bad mode":        {`{"folderID":"fb-9","deviceID":"` + testRemoteID + `","path":"/x","mode":"oneway"}`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := e.call(syncPendingAcceptHandler, e.adminPerm, http.MethodPost, "/api/sync/pending/folders/accept", tc.body, nil)
			if rec.Code != tc.want {
				t.Fatalf("status=%d, want %d (%q)", rec.Code, tc.want, rec.Body.String())
			}
			if len(e.fake.folders) != 0 {
				t.Fatal("a folder was created despite the rejection")
			}
		})
	}

	if entries, _ := os.ReadDir(filepath.Join(e.scope, "real")); len(entries) != 0 {
		t.Fatalf("something was created behind the symlink: %v", entries)
	}
}

func TestSyncDevices(t *testing.T) {
	e := newSyncEnv(t)

	add := func(body string) *httptest.ResponseRecorder {
		return e.call(syncDevicePostHandler, e.adminPerm, http.MethodPost, "/api/sync/devices", body, nil)
	}

	if rec := add(`{"deviceID":"` + strings.ToLower(testRemoteID) + `","name":"laptop","address":"100.64.0.30"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("add: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if d := e.fake.devices[testRemoteID]; d.Name != "laptop" || len(d.Addresses) != 1 || d.Addresses[0] != "tcp://100.64.0.30:22000" {
		t.Fatalf("unexpected device %+v", d)
	}

	for name, body := range map[string]string{
		"bad id":   `{"deviceID":"abc","name":"x"}`,
		"no name":  `{"deviceID":"` + testRemoteID + `","name":""}`,
		"bad addr": `{"deviceID":"` + testRemoteID + `","name":"x","address":"http://evil/x"}`,
		"own id":   `{"deviceID":"` + testSelfID + `","name":"me"}`,
	} {
		if rec := add(body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status=%d, want 400", name, rec.Code)
		}
	}

	// Syncthing rejecting the ID (e.g. wrong check digit) is the user's mistake,
	// reported as 400 with Syncthing's message.
	rec := add(`{"deviceID":"` + testBadCheckDigitID + `","name":"typo"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "check digit incorrect") {
		t.Fatalf("bad check digit: status=%d body=%q", rec.Code, rec.Body.String())
	}

	// An address without a port uses the configured sync port.
	e.server.SyncthingPort = "23000"
	if rec := add(`{"deviceID":"` + testRemoteID + `","name":"laptop","address":"100.64.0.30"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("add with custom default port: status=%d", rec.Code)
	}
	if d := e.fake.devices[testRemoteID]; d.Addresses[0] != "tcp://100.64.0.30:23000" {
		t.Fatalf("expected the configured default port, got %+v", d.Addresses)
	}
	// An explicit port always wins.
	if rec := add(`{"deviceID":"` + testRemoteID + `","name":"laptop","address":"100.64.0.30:24000"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("add with explicit port: status=%d", rec.Code)
	}
	if d := e.fake.devices[testRemoteID]; d.Addresses[0] != "tcp://100.64.0.30:24000" {
		t.Fatalf("expected the explicit port, got %+v", d.Addresses)
	}
	e.server.SyncthingPort = ""

	// A device used by a folder cannot be removed.
	e.fake.folders["f"] = syncthing.Folder{ID: "f", Label: "docs", Devices: []syncthing.DeviceRef{{DeviceID: testRemoteID}}}
	if rec := e.call(syncDeviceDeleteHandler, e.adminPerm, http.MethodDelete, "/api/sync/devices/x", "", map[string]string{"id": testRemoteID}); rec.Code != http.StatusBadRequest {
		t.Fatalf("in-use delete: status=%d, want 400", rec.Code)
	}
	delete(e.fake.folders, "f")
	if rec := e.call(syncDeviceDeleteHandler, e.adminPerm, http.MethodDelete, "/api/sync/devices/x", "", map[string]string{"id": testRemoteID}); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status=%d, want 204", rec.Code)
	}
	if _, ok := e.fake.devices[testRemoteID]; ok {
		t.Fatal("device not removed")
	}
}

func TestSyncFolderPauseAndDelete(t *testing.T) {
	e := newSyncEnv(t)
	e.fake.folders["fb-1"] = syncthing.Folder{ID: "fb-1"}
	vars := map[string]string{"id": "fb-1"}

	if rec := e.call(syncFolderPatchHandler, e.adminPerm, http.MethodPatch, "/api/sync/folders/fb-1", `{"paused":true}`, vars); rec.Code != http.StatusNoContent {
		t.Fatalf("pause: status=%d", rec.Code)
	}
	if !e.fake.folders["fb-1"].Paused {
		t.Fatal("folder not paused")
	}
	if rec := e.call(syncFolderPatchHandler, e.adminPerm, http.MethodPatch, "/api/sync/folders/fb-1", `{}`, vars); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty patch: status=%d, want 400", rec.Code)
	}
	if rec := e.call(syncFolderDeleteHandler, e.adminPerm, http.MethodDelete, "/api/sync/folders/fb-1", "", vars); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status=%d", rec.Code)
	}
	if len(e.fake.folders) != 0 {
		t.Fatal("folder not removed")
	}
}

func TestSyncPendingDismiss(t *testing.T) {
	e := newSyncEnv(t)

	if rec := e.call(syncPendingDeviceDeleteHandler, e.adminPerm, http.MethodDelete, "/x", "", map[string]string{"id": testRemoteID}); rec.Code != http.StatusNoContent {
		t.Fatalf("device dismiss: status=%d", rec.Code)
	}
	if rec := e.call(syncPendingFolderDeleteHandler, e.adminPerm, http.MethodDelete, "/x?folder=fb-1&device="+testRemoteID, "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("folder dismiss: status=%d", rec.Code)
	}
	if len(e.fake.dismissed) != 2 {
		t.Fatalf("dismissed=%v", e.fake.dismissed)
	}
}

func TestSyncUnavailableMapsToBadGateway(t *testing.T) {
	e := newSyncEnv(t)
	e.server.SyncthingAPIKey = "wrong"
	if rec := e.call(syncStatusHandler, e.adminPerm, http.MethodGet, "/api/sync", "", nil); rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502", rec.Code)
	}
}
