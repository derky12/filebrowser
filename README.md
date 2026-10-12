<p align="center">
  <img src="./branding/banner.png" width="550"/>
</p>

<p align="center"><b>English</b> · <a href="README.zh-CN.md">简体中文</a></p>

# File Browser (enhanced fork): self-hosted cloud drive, multi-machine sync and WebDAV

A **create-your-own-cloud** web app. One web page manages the files on your machine, and keeps folders in sync between several machines. Run it on your own server, NAS or home computer; your data never goes through a third party.

**What you can do with it**

- **Manage files**: upload, download, preview, edit, move, copy, rename and delete in the browser. Batch rename, compress / extract (zip, tar.gz, tar.xz, tar.zst), dual-pane drag and drop, and a disk usage view.
- **Sync between machines**: Syncthing is built in. Right-click a folder → "Sync folder" to share it with another machine, one-way or two-way, with conflict handling, ignore rules, bandwidth limits and live transfer speeds.
- **Share over WebDAV**: right-click a folder → "Share as WebDAV" and mount it from Finder, Windows Explorer, a phone app or rclone. It can sit behind Caddy, nginx or Tailscale Serve for HTTPS.
- **Undo mistakes**: deleted items go to a trash you can restore from; an activity log records who did what and when.
- **Everyday comforts**: favorites, folder colors, cut / paste buttons, a text editor with formatting and syntax highlighting, and chmod / chown from the context menu.
- **One-command deployment**: amd64 and arm64 share one `docker-compose.yml`, and `scripts/init-env.sh` writes the configuration for you.

> [!WARNING]
> **Security: this fork is for private use and has no authentication configured.**
> I run it on a private Tailscale network, so the example `docker-compose.yml` sets `FB_NOAUTH=true`: **there is no login**, and anyone who can reach the port can read and write the whole served directory. The Syncthing web UI is also unprotected until you set a password (WebDAV shares use one username / password pair from `.env`).
> **Do not expose it to the internet** (this includes Tailscale Funnel). If you want to open it up or share it with other people, remove `FB_NOAUTH` and put an HTTPS reverse proxy, authentication and a firewall in front of it. See also [Security](#security).

## Tribute to File Browser

This project is built on **[filebrowser/filebrowser](https://github.com/filebrowser/filebrowser)**. Many thanks to [Henrique Dias (hacdias)](https://github.com/hacdias) and all the contributors who spent years turning File Browser into a simple, reliable and friendly open-source file manager. This fork would not exist without their work.

- It is based on upstream **v2.63.23**. Upstream was archived on 2026-09-01 and will get no further releases or security fixes. Background: [Goodbye File Browser, for Real This Time](https://hacdias.com/2026/07/28/filebrowser/).
- This fork only adds the features I needed and keeps maintaining them. **It is not affiliated with the official project**; please report problems in this repository, not upstream.
- It keeps upstream's [Apache License 2.0](LICENSE) and all the original copyright and license notices. Every change is listed in [Features added in this fork](#features-added-in-this-fork) and in [`CHANGES.md`](CHANGES.md).
- Upstream's own documentation ([`docs/`](docs), [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md)) stays in the repository.

## Features added in this fork

On top of upstream v2.63.23. The code follows upstream's style and has tests. The per-feature design notes, limits and API details are in the [Chinese README](README.zh-CN.md) (the links below point to the matching section).

**Browsing and organizing**

| Feature | How to use it |
|---|---|
| [Cut / paste buttons](README.zh-CN.md#剪切--粘贴按钮) | Select files → ✂ in the top bar, then Paste in the target folder |
| [Favorites](README.zh-CN.md#收藏夹) | Star a folder; it appears in the sidebar and you can drop files on it (move, or copy with Ctrl/Cmd) |
| [Folder colors](README.zh-CN.md#文件夹颜色) | Palette button on a folder, 10 presets |
| [Dual-pane mode](README.zh-CN.md#双栏模式) | Sidebar "Dual pane": two folders side by side, drag and drop, F5 copy, F6 move |
| [Batch rename](README.zh-CN.md#批量重命名与复制一份) | Select several items → find / replace (regex), prefix / suffix, case, numbering, live preview |
| [Duplicate](README.zh-CN.md#批量重命名与复制一份) | Select items → "Duplicate" creates `name(1)` next to the original |
| [Compress / extract](README.zh-CN.md#压缩与解压) | zip, tar.gz, tar.xz, tar.zst and more |
| [WebDAV sharing](README.zh-CN.md#webdav-共享) | Right-click a folder → "Share as WebDAV"; [HTTPS behind a reverse proxy](README.zh-CN.md#用-https-反向代理访问-webdav) |
| [Path autocomplete](README.zh-CN.md#路径自动补全) | Existing folders are suggested when choosing a save location or extract target |
| [Disk usage](README.zh-CN.md#磁盘占用) | Top bar "Disk usage": size of every child, drill down level by level |

**Safety and recovery**

| Feature | How to use it |
|---|---|
| [Trash](README.zh-CN.md#回收站) | Deleting moves items to the trash, restore any time; size limit and retention days are configurable |
| [Activity log](README.zh-CN.md#活动日志) | Settings → Activity: who did what, when and from where |
| [Permissions](README.zh-CN.md#右键权限chmod--chown) | Right-click → Permissions: chmod / chown, optionally recursive |
| Hidden `~/.ssh` | The container overlays `/srv/.ssh` with an empty directory so keys are never served |

**Multi-machine sync (Syncthing)**

| Feature | How to use it |
|---|---|
| [Folder sync](README.zh-CN.md#文件夹同步) | Select a folder → "Sync folder", pick the peer and one-way / two-way; the peer accepts it under Settings → Sync |
| [Status and speed](README.zh-CN.md#状态与速度) | Settings → Sync: per-folder state and remaining data, per-device live speed and totals |
| [Conflicts, ignore rules, bandwidth limits, badges](README.zh-CN.md#冲突忽略规则限速与标记) | Pick which version to keep (the other goes to the trash), edit `.stignore`-style rules, limit up / down speed, see a sync icon on synced folders |

**Editing**

| Feature | How to use it |
|---|---|
| [Text editor](README.zh-CN.md#文本编辑器) | JetBrains Mono, per-language indentation, Alt+Shift+F formatting (YAML, JSON, JS/TS, CSS, HTML, Vue, Markdown, Python, SQL, Go), format on save, offline syntax highlighting and linting |

**Deployment and releases**

| Feature | How to use it |
|---|---|
| [Multi-arch Docker](README.zh-CN.md#多架构-docker-构建与发布) | One `Dockerfile.multiarch` builds amd64 and arm64 images |
| [docker-compose + `.env`](#quick-start) | One compose file for both architectures; architecture, IP, ports, data directory and device name live in `.env` |
| Versions and change log | Versions look like `<upstream version>-fork.<n>` (see `VERSION`), with notes in [`CHANGES.md`](CHANGES.md) |

## Quick start

Two services run together: `filebrowser` and `syncthing`. Everything that differs between machines is in a `.env` file (not committed); `docker-compose.yml` only references variables.

1. **Create `.env`**: run `scripts/init-env.sh` in the repository directory. It detects the architecture (`uname -m`) and the Tailscale address (`tailscale ip -4`), generates a Syncthing API key, uses the host name as the device name and writes `.env` (mode 600). It never overwrites an existing `.env`. If detection fails, pass the values: `HOST_IP=100.x.y.z DATA_DIR=/root SYNCTHING_NAME=my-server scripts/init-env.sh`. You can also `cp .env.example .env` and edit it.
2. **Start**: `docker compose -f docker-compose.yml up -d`, then open `http://<HOST_IP>:<WEB_PORT>` (default port 38002). The `-f` is required because upstream's `compose.yaml` (with redis) takes precedence otherwise.
3. **Update**: `docker compose -f docker-compose.yml pull && docker compose -f docker-compose.yml up -d`. With docker-compose 1.29 (v1), if recreating fails with `KeyError: 'ContainerConfig'`, run `docker-compose down` first, or use `docker compose` (v2).

| `.env` variable | Default | Meaning |
|---|---|---|
| `ARCH` | required | `amd64` or `arm64`; selects the image platform |
| `HOST_IP` | required | The machine's Tailscale address; every port is published only on it |
| `SYNCTHING_API_KEY` | required | Secret shared by File Browser and Syncthing (written by `init-env.sh`) |
| `DATA_DIR` | `/root` | Host directory that is served and synced; mounted at `/srv` in both containers |
| `WEB_PORT` | `38002` | File Browser web port |
| `SYNCTHING_GUI_PORT` | `38003` | Syncthing's own web UI |
| `SYNC_PORT` / `DISCOVERY_PORT` | `23000` / `23001` | Sync port (TCP + UDP, same on all machines) and local discovery port |
| `SYNCTHING_NAME` | none | Device name shown to peers; must differ per machine |
| `WEBDAV_*` | WebDAV off | `WEBDAV_ENABLED`, `WEBDAV_USER`, `WEBDAV_PASSWORD` (8+ characters), `WEBDAV_BIND_IP`, `WEBDAV_PORT` (38004), and the proxy-related `WEBDAV_PUBLIC_URL`, `WEBDAV_PREFIX`, `WEBDAV_BEHIND_PROXY` |
| `PUID` / `PGID` | `0` / `0` | User the containers run as |

Put comments in `.env` on their own lines, not after a value. Do not write the compose file or `.env` with an unquoted heredoc (`<< EOF`): the shell would expand every `${...}`; use `<< 'EOF'` or the script. Common errors (`exec format error`, `ports contains an invalid type`) and the notes about serving `/root` safely are in the [Chinese deployment section](README.zh-CN.md#部署).

**Images**: `q25459768/filebrowser:latest` is a multi-arch tag (amd64 and arm64); `:<version>-amd64` / `:<version>-arm64` and version tags are also published. You can also build from source with `Dockerfile.multiarch`.

**WebDAV**: set `WEBDAV_ENABLED=true` and a password in `.env`, recreate the containers, then right-click a folder → "Share as WebDAV". Clients connect to `http://<HOST_IP>:38004/<share-name>/`. Setup for HTTPS with Tailscale Serve, Caddy or nginx is in [README.zh-CN.md](README.zh-CN.md#用-https-反向代理访问-webdav) and [`examples/https/`](examples/https).

## Build and test

```bash
go build ./... && go vet ./... && go test ./...           # backend
cd frontend && pnpm install --frozen-lockfile
pnpm run typecheck && pnpm run lint && pnpm test           # frontend (vitest)
pnpm run build                                             # builds frontend/dist; then `go build` gives a binary with the UI
```

## Security

Published advisories are listed under [security advisories](https://github.com/filebrowser/filebrowser/security/advisories),
and reporting instructions are in [SECURITY.md](SECURITY.md). Two known issue classes
remain unaddressed and will not be fixed:

- **Command execution, runner, and hooks.** This feature is plagued with vulnerabilities across many published advisories, and would need a full rewrite to be made safe. It is disabled by default; if you re-enable it with `--disable-exec=false`, treat the ability to run commands as equivalent to shell access on the host. Background: [#5199](https://github.com/filebrowser/filebrowser/issues/5199).
- **Session and JWT handling.** Sessions are self-contained JWTs rather than server-side identifiers, so they cannot be revoked, which means that logout, password changes, and renewal leave previously issued tokens valid until they expire, and the same refresh token can be redeemed repeatedly. Assume a leaked token is valid until expiry. Background: [#5216](https://github.com/filebrowser/filebrowser/issues/5216).

If you keep running File Browser, treat it as unmaintained software:

- **Do not expose it directly to the internet.** Put it behind a reverse proxy that terminates TLS and performs its own authentication.
- **Keep the command runner disabled.** It is off by default, so leave it off. See [#5199](https://github.com/filebrowser/filebrowser/issues/5199) and [`docs/command-execution.md`](docs/command-execution.md).
- **Run it unprivileged, inside a container**, with only the directory you intend to serve mounted into it.

## Documentation

Upstream's documentation on how to install, configure, and build the project lives in [`docs`](docs). [CONTRIBUTING.md](CONTRIBUTING.md) documents how to build and develop it.

## License

[Apache License 2.0](LICENSE) © File Browser Contributors
