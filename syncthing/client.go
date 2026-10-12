// Package syncthing is a small client for the Syncthing REST API, covering
// just what File Browser needs to share folders between instances.
package syncthing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrUnavailable is returned when Syncthing cannot be reached or rejects the
// API key.
var ErrUnavailable = errors.New("syncthing is unavailable")

// APIError is an error response from the Syncthing API.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("syncthing: %s %s: %d %s: %s", e.Method, e.Path, e.Status, http.StatusText(e.Status), e.Body)
}

// Folder types understood by Syncthing.
const (
	TypeSendOnly    = "sendonly"
	TypeSendReceive = "sendreceive"
	TypeReceiveOnly = "receiveonly"
)

// Client talks to a Syncthing instance.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New returns a client for the Syncthing instance at baseURL.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// DeviceRef is a device listed in a folder.
type DeviceRef struct {
	DeviceID string `json:"deviceID"`
}

// Versioning is the file versioning configuration of a folder.
type Versioning struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params,omitempty"`
}

// Folder is a Syncthing folder configuration (the subset File Browser uses).
type Folder struct {
	ID               string      `json:"id"`
	Label            string      `json:"label"`
	Path             string      `json:"path"`
	Type             string      `json:"type"`
	Devices          []DeviceRef `json:"devices"`
	Paused           bool        `json:"paused"`
	FSWatcherEnabled bool        `json:"fsWatcherEnabled"`
	FSWatcherDelayS  int         `json:"fsWatcherDelayS,omitempty"`
	RescanIntervalS  int         `json:"rescanIntervalS"`
	Versioning       *Versioning `json:"versioning,omitempty"`
}

// Device is a Syncthing device configuration (the subset File Browser uses).
type Device struct {
	DeviceID  string   `json:"deviceID"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Paused    bool     `json:"paused"`
}

// FolderStatus is the local state of a folder.
type FolderStatus struct {
	State       string `json:"state"`
	GlobalBytes int64  `json:"globalBytes"`
	InSyncBytes int64  `json:"inSyncBytes"`
	NeedBytes   int64  `json:"needBytes"`
	NeedFiles   int    `json:"needFiles"`
	Errors      int    `json:"errors"`
	PullErrors  int    `json:"pullErrors"`
	Error       string `json:"error"`
}

// Completion is how far a remote device is in syncing a folder.
type Completion struct {
	Completion float64 `json:"completion"`
	NeedBytes  int64   `json:"needBytes"`
	NeedItems  int     `json:"needItems"`
}

// Connection is the connection state of a remote device.
type Connection struct {
	Connected bool   `json:"connected"`
	Paused    bool   `json:"paused"`
	Address   string `json:"address"`
	// Bytes transferred since the connection was established.
	InBytesTotal  int64 `json:"inBytesTotal"`
	OutBytesTotal int64 `json:"outBytesTotal"`
}

// PendingDevice is a device that tried to connect but is not configured yet.
type PendingDevice struct {
	Time    string `json:"time"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// PendingFolder is a folder offered by remote devices and not accepted yet.
type PendingFolder struct {
	OfferedBy map[string]PendingOffer `json:"offeredBy"`
}

// PendingOffer is one device's offer of a folder.
type PendingOffer struct {
	Time  string `json:"time"`
	Label string `json:"label"`
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out interface{}) error {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: invalid API key", ErrUnavailable)
	case resp.StatusCode >= 300:
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}

	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}

	return nil
}

// MyID returns the device ID of the Syncthing instance.
func (c *Client) MyID(ctx context.Context) (string, error) {
	var status struct {
		MyID string `json:"myID"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/system/status", nil, nil, &status); err != nil {
		return "", err
	}

	return status.MyID, nil
}

// Devices lists the configured devices, including the local one.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var devices []Device
	err := c.do(ctx, http.MethodGet, "/rest/config/devices", nil, nil, &devices)
	return devices, err
}

// PutDevice adds or replaces a device.
func (c *Client) PutDevice(ctx context.Context, device Device) error {
	return c.do(ctx, http.MethodPut, "/rest/config/devices/"+url.PathEscape(device.DeviceID), nil, device, nil)
}

// DeleteDevice removes a device.
func (c *Client) DeleteDevice(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/rest/config/devices/"+url.PathEscape(id), nil, nil, nil)
}

// Folders lists the configured folders.
func (c *Client) Folders(ctx context.Context) ([]Folder, error) {
	var folders []Folder
	err := c.do(ctx, http.MethodGet, "/rest/config/folders", nil, nil, &folders)
	return folders, err
}

// PutFolder adds or replaces a folder.
func (c *Client) PutFolder(ctx context.Context, folder Folder) error {
	return c.do(ctx, http.MethodPut, "/rest/config/folders/"+url.PathEscape(folder.ID), nil, folder, nil)
}

// DeleteFolder removes a folder from Syncthing. Files on disk are untouched.
func (c *Client) DeleteFolder(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/rest/config/folders/"+url.PathEscape(id), nil, nil, nil)
}

// SetFolderPaused pauses or resumes a folder.
func (c *Client) SetFolderPaused(ctx context.Context, id string, paused bool) error {
	return c.do(ctx, http.MethodPatch, "/rest/config/folders/"+url.PathEscape(id), nil, map[string]bool{"paused": paused}, nil)
}

// FolderStatus returns the local state of a folder.
func (c *Client) FolderStatus(ctx context.Context, id string) (FolderStatus, error) {
	var status FolderStatus
	err := c.do(ctx, http.MethodGet, "/rest/db/status", url.Values{"folder": {id}}, nil, &status)
	return status, err
}

// Completion returns how far a remote device is in syncing a folder.
func (c *Client) Completion(ctx context.Context, folder, device string) (Completion, error) {
	var completion Completion
	err := c.do(ctx, http.MethodGet, "/rest/db/completion", url.Values{"folder": {folder}, "device": {device}}, nil, &completion)
	return completion, err
}

// Connections returns the connection state of every remote device.
func (c *Client) Connections(ctx context.Context) (map[string]Connection, error) {
	var connections struct {
		Connections map[string]Connection `json:"connections"`
	}
	err := c.do(ctx, http.MethodGet, "/rest/system/connections", nil, nil, &connections)
	return connections.Connections, err
}

// PendingDevices lists devices waiting to be accepted.
func (c *Client) PendingDevices(ctx context.Context) (map[string]PendingDevice, error) {
	pending := map[string]PendingDevice{}
	err := c.do(ctx, http.MethodGet, "/rest/cluster/pending/devices", nil, nil, &pending)
	return pending, err
}

// PendingFolders lists folders offered by remote devices.
func (c *Client) PendingFolders(ctx context.Context) (map[string]PendingFolder, error) {
	pending := map[string]PendingFolder{}
	err := c.do(ctx, http.MethodGet, "/rest/cluster/pending/folders", nil, nil, &pending)
	return pending, err
}

// DismissPendingDevice rejects a pending device.
func (c *Client) DismissPendingDevice(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/rest/cluster/pending/devices", url.Values{"device": {id}}, nil, nil)
}

// DismissPendingFolder rejects a folder offered by a device.
func (c *Client) DismissPendingFolder(ctx context.Context, folder, device string) error {
	return c.do(ctx, http.MethodDelete, "/rest/cluster/pending/folders", url.Values{"folder": {folder}, "device": {device}}, nil, nil)
}

// UsePrivateNetwork turns off everything that talks to the public internet
// (global discovery, relays, NAT traversal, local discovery, usage reports),
// for instances that only sync over a private network such as a VPN.
func (c *Client) UsePrivateNetwork(ctx context.Context) error {
	return c.do(ctx, http.MethodPatch, "/rest/config/options", nil, map[string]interface{}{
		"globalAnnounceEnabled": false,
		"localAnnounceEnabled":  false,
		"relaysEnabled":         false,
		"natEnabled":            false,
		"urAccepted":            -1,
	}, nil)
}

// Ignores returns the ignore patterns of a folder (its .stignore).
func (c *Client) Ignores(ctx context.Context, folder string) ([]string, error) {
	var out struct {
		Ignore []string `json:"ignore"`
	}
	err := c.do(ctx, http.MethodGet, "/rest/db/ignores", url.Values{"folder": {folder}}, nil, &out)
	if out.Ignore == nil {
		out.Ignore = []string{}
	}

	return out.Ignore, err
}

// SetIgnores replaces the ignore patterns of a folder.
func (c *Client) SetIgnores(ctx context.Context, folder string, lines []string) error {
	return c.do(ctx, http.MethodPost, "/rest/db/ignores", url.Values{"folder": {folder}}, map[string][]string{"ignore": lines}, nil)
}

// Limits are the bandwidth limits of the instance, in KiB/s; 0 is unlimited.
type Limits struct {
	MaxRecvKbps         int  `json:"maxRecvKbps"`
	MaxSendKbps         int  `json:"maxSendKbps"`
	LimitBandwidthInLan bool `json:"limitBandwidthInLan"`
}

// Limits returns the current bandwidth limits.
func (c *Client) Limits(ctx context.Context) (Limits, error) {
	var limits Limits
	err := c.do(ctx, http.MethodGet, "/rest/config/options", nil, nil, &limits)
	return limits, err
}

// SetLimits changes the bandwidth limits.
func (c *Client) SetLimits(ctx context.Context, limits Limits) error {
	return c.do(ctx, http.MethodPatch, "/rest/config/options", nil, limits, nil)
}
