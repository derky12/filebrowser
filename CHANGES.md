# Changes of this fork

Releases of the private fork of File Browser. The version is `<upstream
version>-fork.<n>`; it is what the sidebar shows, the tag of the Docker images
(`q25459768/filebrowser:<version>-amd64` / `-arm64`, plus the moving `:amd64` /
`:arm64`) and the git tag `v<version>`. The bullet list of a release is stored
in the image label `com.filebrowser.fork.changes`
(`docker inspect --format '{{ index .Config.Labels "com.filebrowser.fork.changes" }}' <image>`).

Add a section here before every release (`scripts/release.sh` refuses to run
without one) and bump `VERSION`.

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
