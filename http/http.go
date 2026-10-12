package fbhttp

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
)

type modifyRequest struct {
	What            string   `json:"what"`             // Answer to: what data type?
	Which           []string `json:"which"`            // Answer to: which fields?
	CurrentPassword string   `json:"current_password"` // Answer to: user logged password
}

func NewHandler(
	imgSvc ImgService,
	fileCache FileCache,
	uploadCache UploadCache,
	store *storage.Storage,
	server *settings.Server,
	assetsFs fs.FS,
) (http.Handler, error) {
	server.Clean()
	server.CaseInsensitiveFs = files.CaseInsensitive(afero.NewOsFs(), server.Root)

	startSyncthingSetup(server)
	if store.Activity != nil {
		store.Activity.Retention = time.Duration(server.ActivityDays) * 24 * time.Hour
	}

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
			next.ServeHTTP(w, r)
		})
	})
	index, static := getStaticHandlers(store, server, assetsFs)

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}

	r.HandleFunc("/health", healthHandler)
	r.PathPrefix("/static").Handler(static)
	r.NotFoundHandler = index

	api := r.PathPrefix("/api").Subrouter()

	tokenExpirationTime := server.GetTokenExpirationTime(DefaultTokenExpirationTime)
	api.Handle("/login", monkey(loginHandler(tokenExpirationTime), ""))
	api.Handle("/signup", monkey(signupHandler, ""))
	api.Handle("/renew", monkey(renewHandler(tokenExpirationTime), ""))

	users := api.PathPrefix("/users").Subrouter()
	users.Handle("", monkey(usersGetHandler, "")).Methods("GET")
	users.Handle("", monkey(userPostHandler, "")).Methods("POST")
	users.Handle("/{id:[0-9]+}", monkey(userPutHandler, "")).Methods("PUT")
	users.Handle("/{id:[0-9]+}", monkey(userGetHandler, "")).Methods("GET")
	users.Handle("/{id:[0-9]+}", monkey(userDeleteHandler, "")).Methods("DELETE")

	api.PathPrefix("/resources/recursive").Handler(monkey(resourceGetRecursiveHandler, "/api/resources/recursive")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(audited(auditPost, resourcePostHandler(fileCache)), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(audited(auditFixed("save"), resourcePutHandler), "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(audited(auditPatch, resourcePatchHandler(fileCache)), "/api/resources")).Methods("PATCH")

	api.PathPrefix("/tus").Handler(monkey(tusPostHandler(uploadCache), "/api/tus")).Methods("POST")
	api.PathPrefix("/tus").Handler(monkey(tusHeadHandler(uploadCache), "/api/tus")).Methods("HEAD", "GET")
	api.PathPrefix("/tus").Handler(monkey(tusPatchHandler(uploadCache), "/api/tus")).Methods("PATCH")
	api.PathPrefix("/tus").Handler(monkey(tusDeleteHandler(uploadCache), "/api/tus")).Methods("DELETE")

	api.Handle("/format/{lang}", monkey(formatHandler, "")).Methods("POST")

	api.PathPrefix("/permissions").Handler(monkey(permissionsGetHandler, "/api/permissions")).Methods("GET")
	api.PathPrefix("/permissions").Handler(monkey(permissionsPatchHandler, "/api/permissions")).Methods("PATCH")

	api.PathPrefix("/du").Handler(monkey(diskUsageHandler, "/api/du")).Methods("GET")
	api.PathPrefix("/usage").Handler(monkey(diskUsage, "/api/usage")).Methods("GET")

	api.Handle("/shares", monkey(shareListHandler, "")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(shareGetsHandler, "/api/share")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(audited(auditFixed("share"), sharePostHandler), "/api/share")).Methods("POST")
	api.PathPrefix("/share").Handler(monkey(shareDeleteHandler, "/api/share")).Methods("DELETE")

	api.Handle("/trash", monkey(trashListHandler, "")).Methods("GET")
	api.Handle("/trash", monkey(trashEmptyHandler, "")).Methods("DELETE")
	api.Handle("/trash/{id}/restore", monkey(trashRestoreHandler, "")).Methods("POST")
	api.Handle("/trash/{id}", monkey(trashDeleteHandler, "")).Methods("DELETE")

	api.Handle("/activity", monkey(activityGetHandler, "")).Methods("GET")

	sync := api.PathPrefix("/sync").Subrouter()
	sync.Handle("", monkey(syncStatusHandler, "")).Methods("GET")
	sync.Handle("/devices", monkey(syncDevicePostHandler, "")).Methods("POST")
	sync.Handle("/devices/{id}", monkey(syncDeviceDeleteHandler, "")).Methods("DELETE")
	sync.Handle("/folders", monkey(syncFolderPostHandler, "")).Methods("POST")
	sync.Handle("/folders/{id}", monkey(syncFolderPatchHandler, "")).Methods("PATCH")
	sync.Handle("/folders/{id}", monkey(syncFolderDeleteHandler, "")).Methods("DELETE")
	sync.Handle("/conflicts", monkey(syncConflictsGetHandler, "")).Methods("GET")
	sync.Handle("/conflicts/resolve", monkey(syncConflictResolveHandler, "")).Methods("POST")
	sync.Handle("/folders/{id}/ignores", monkey(syncIgnoresGetHandler, "")).Methods("GET")
	sync.Handle("/folders/{id}/ignores", monkey(syncIgnoresPutHandler, "")).Methods("PUT")
	sync.Handle("/limits", monkey(syncLimitsGetHandler, "")).Methods("GET")
	sync.Handle("/limits", monkey(syncLimitsPutHandler, "")).Methods("PUT")
	sync.Handle("/pending/devices/{id}", monkey(syncPendingDeviceDeleteHandler, "")).Methods("DELETE")
	sync.Handle("/pending/folders", monkey(syncPendingFolderDeleteHandler, "")).Methods("DELETE")
	sync.Handle("/pending/folders/accept", monkey(syncPendingAcceptHandler, "")).Methods("POST")

	api.Handle("/settings", monkey(settingsGetHandler, "")).Methods("GET")
	api.Handle("/settings", monkey(settingsPutHandler, "")).Methods("PUT")

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/command").Handler(monkey(commandsHandler, "/api/command")).Methods("GET")
	api.PathPrefix("/search").Handler(monkey(searchHandler, "/api/search")).Methods("GET")
	api.PathPrefix("/subtitle").Handler(monkey(subtitleHandler, "/api/subtitle")).Methods("GET")

	public := api.PathPrefix("/public").Subrouter()
	public.PathPrefix("/dl").Handler(monkey(publicDlHandler, "/api/public/dl/")).Methods("GET")
	public.PathPrefix("/share").Handler(monkey(publicShareHandler, "/api/public/share/")).Methods("GET")

	return stripPrefix(server.BaseURL, r), nil
}
