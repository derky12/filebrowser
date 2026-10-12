# Changes of this fork

Releases of the private fork of File Browser. The version is `<upstream
version>-fork.<n>`; it is what the sidebar shows, the tag of the Docker images
(`q25459768/filebrowser:<version>-amd64` / `-arm64`, plus the moving `:amd64` /
`:arm64`) and the git tag `v<version>`. The bullet list of a release is stored
in the image label `com.filebrowser.fork.changes`
(`docker inspect --format '{{ index .Config.Labels "com.filebrowser.fork.changes" }}' <image>`).

Add a section here before every release (`scripts/release.sh` refuses to run
without one) and bump `VERSION`.

## 2.63.23-fork.4 (2026-10-13)

- Sync tab: a button opens the Syncthing web interface (needs FB_SYNCTHING_GUI_PORT, which docker-compose.yml now sets from SYNCTHING_GUI_PORT; no button without it).
- Settings > Profile: dotfiles that stay visible while "Hide dotfiles" is on, a list of names with * wildcards (default .env*, .git*, .dockerignore; .trash stays hidden).
- New installs default to hiding dotfiles, single click and redirect after copy/move; existing users keep their settings.
- The "File Browser is being archived" notice is removed from the Settings pages.

## 2.63.23-fork.3 (2026-10-12)

- WebDAV: right click a folder > Share as WebDAV (read-only by default), manage the shares in Settings > WebDAV. One WebDAV server with a user name and password from .env (WEBDAV_ENABLED, WEBDAV_USER, WEBDAV_PASSWORD, WEBDAV_BIND_IP, WEBDAV_PORT), shares appear as top level folders, each confined to its folder; changes are logged and wrong passwords are limited per client address.
- WebDAV behind an HTTPS reverse proxy: WEBDAV_PUBLIC_URL, WEBDAV_PREFIX and WEBDAV_BEHIND_PROXY, with tested Caddy and nginx examples (examples/https) and a README section on the pitfalls (the Host header for MOVE/COPY, body size limits, sub-paths, client addresses).
- Folder path autocomplete when typing a path in the Sync tab (accept a shared folder) and in the extract dialog.
- Fix: environment variables of a flag that is the prefix of another one did not work (FB_SOCKET_PERM); every flag is now tested for its environment variable.
- docker-compose: new optional WebDAV variables with safe defaults (off, bound to 127.0.0.1); scripts/init-env.sh makes a WebDAV password.

## 2.63.23-fork.2 (2026-10-12)

- Trash: deleting moves files to a recycle bin that can be restored; it can be switched off, and limited by total size (GB) and days, in Settings > Global Settings. Items larger than the whole limit are deleted for good; deleting can also skip the trash.
- Activity log (Settings > Activity, admins): who created, uploaded, saved, deleted, renamed, moved, copied, shared, logged in, changed permissions, restored, compressed or synced what, when and from where; kept for --activityDays (default 90).
- Sync additions: a status badge on synced folders in the file list, conflict resolution (keep either version, the other goes to the trash), editable ignore rules per folder and bandwidth limits.
- Disk usage page: what each item in a folder takes, as bars, drilling down folder by folder.
- Compress to zip or tar (gz, bz2, xz, zst) and extract zip, tar, 7z and rar, refusing path traversal, links and archive bombs.
- Rename several items at once (find and replace with regex, prefix, suffix, case, numbering, live preview) and duplicate items.
- Two panes view: two folders side by side with drag and drop (Ctrl/Cmd copies), F5/F6/F7 and keyboard selection.
- docker-compose: the Tailscale address, all ports, the served folder, the Syncthing device name and the user now come from .env; Syncthing's web interface is published on SYNCTHING_GUI_PORT (default 38003); HOST_IP is required so nothing falls back to all interfaces.

## 2.63.23-fork.1 (2026-10-12)

- Cut and Paste buttons in the header bar, next to Copy and Move.
- Favorite folders: star a folder, it appears in the sidebar and accepts dropped files (move, or copy with Ctrl/Cmd).
- Folder colors: pick one of 10 preset colors per folder, shown in the listing and the sidebar.
- Folder sync between instances, backed by Syncthing: share a folder one-way or two-way, the other side accepts it, manage and pause it under Settings > Sync, with speed, remaining size and totals.
- Syncthing runs next to File Browser in docker-compose, with configurable sync and discovery ports (default 23000 / 23001) and discovery, relays and NAT turned off for VPN-only use.
- Right click > Permissions: change the mode (rwx grid or octal) of files and folders, recursively with separate file and folder modes, and the owner and group for administrators; refuses the scope root, symlinks and setuid/setgid/sticky bits.
- Text editor: 16px JetBrains Mono by default, per-language indentation, a Format action (Alt+Shift+F) with optional format-on-save for YAML, JSON, JS/TS, CSS, HTML, Vue, Markdown, Python, SQL and Go; syntax highlighting and checking now work offline.
- Multi-arch Docker build from source (Dockerfile.multiarch) and docker-compose.yml for amd64 and arm64, with ~/.ssh hidden from the containers.
- Fix: favorites and folder colors disappeared when entering a folder (the token renewal replaced the user).
- Fix: scripts without a precompressed copy are served instead of answering 404.
- Fix: a mistyped Syncthing device ID is reported as 400 with the reason instead of a generic 500.
