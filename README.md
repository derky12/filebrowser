> [!WARNING]
> 
> **File Browser is archived on 2026-09-01**. The last planned release has already shipped. There will be no further releases, bug fixes, or security fixes.   

<p align="center">
  <img src="./branding/banner.png" width="550"/>
</p>

File Browser provides a file managing interface within a specified directory and it can be used to upload, delete, preview and edit your files. It is a **create-your-own-cloud**-kind of software where you can just install it on your server, direct it to a path and access your files through a nice web interface.

**Background:** [Goodbye File Browser, for Real This Time](https://hacdias.com/2026/07/28/filebrowser/), July 2026.

## 本分支的改动

这是 `filebrowser/filebrowser` 的私人维护分支（上游已于 2026-09-01 归档）。在上游 v2.63.23 的基础上增加了下面这些功能。所有改动都遵循上游的代码风格，并带有测试；下面的内容以代码为准。

| 功能 | 用途 | 位置 |
|---|---|---|
| [剪切 / 粘贴按钮](#剪切--粘贴按钮) | 不用键盘也能剪切、粘贴 | 前端 |
| [收藏夹](#收藏夹) | 常用文件夹点星号，出现在侧栏，可以直接把文件拖进去 | 后端 + 前端 |
| [文件夹颜色](#文件夹颜色) | 用预设色板给文件夹换颜色 | 后端 + 前端 |
| [文件夹同步](#文件夹同步) | 在 File Browser 里把一个文件夹同步到另一台机器（单向/双向，对方同意后生效） | 后端 + 前端 + Syncthing |
| [多架构 Docker 构建](#多架构-docker-构建与发布) | 从源码构建 amd64 / arm64 镜像 | Docker |

### 分支、镜像与机器

| 分支 | 机器 | 镜像 | Tailscale 地址 | Syncthing 设备名 |
|---|---|---|---|---|
| `main` | amd64 | `q25459768/filebrowser:amd64` | `100.64.0.7` | `fb-amd64` |
| `arm64` | arm64 | `q25459768/filebrowser:arm64` | `100.64.0.30` | `fb-arm64` |

- 两个分支的代码完全相同，**只有 `docker-compose.yml` 里每台机器自己的 7 行不同**：头部注释、镜像 tag、Web 端口的 IP、Syncthing 主机名、同步端口和发现端口的 IP。
- 修改 `main` 之后，把 `main` 合并进 `arm64`，再把 `docker-compose.yml` 里这 7 行换成 arm64 的值。
- 仓库里还保留上游自带的 `compose.yaml`（带 redis）。因为它优先于 `docker-compose.yml`，所以启动时必须写 `-f docker-compose.yml`。
- `compose.multiarch.yaml` 是一份不含 Syncthing、从源码构建的简单示例，日常不用。

### 剪切 / 粘贴按钮

- **用法**：选中文件后，顶栏（桌面）和选择条（手机）里有 ✂ 剪切按钮。剪切或复制之后，剪贴板非空时会出现粘贴按钮，点它就粘贴到当前目录。冲突处理沿用原来的“替换 / 跳过 / 重命名”弹窗。
- **设计**：没有新增后端接口。按钮只是调用原来 Ctrl+X / Ctrl+V 用的 `copyCut` 和 `paste`（`frontend/src/views/files/FileListing.vue`），所以剪切就是“移动”，复制仍然用原来的复制弹窗。
- **权限**：剪切需要 `rename` 权限，粘贴需要 `create` 权限，和原来的移动、复制一致。

### 收藏夹

- **用法**：鼠标移到文件夹上，右上角出现星号，点一下收藏，再点取消。侧栏的“收藏夹”区域列出所有收藏，点击进入，悬停时出现 ✕ 可移除。
- **拖放**：把选中的文件拖到侧栏的某个收藏夹上，默认是**移动**；按住 **Ctrl / Cmd** 是**复制**。拖到文件当前所在的文件夹上不会有反应。有同名文件时弹出和原来一样的冲突处理框。
- **存储**：存在服务器端的用户记录里（`users.User.Favorites []string`，JSON 字段 `favorites`），所以换浏览器、换机器登录同一个实例看到的是同一份。收藏夹只属于所在实例，不会跨实例同步。
- **路径格式**：解码后的、相对于用户 scope 的路径，不带结尾斜杠，根目录是 `/`，例如 `/Photos/2024`。
- **校验**（`users/users.go` 的 `cleanFavorites`）：必须以 `/` 开头，经 `path.Clean` 规范化（`/A/` 和 `/A` 合并、`/x/../C` 变成 `/C`），去重，最多 100 个（`MaxFavorites`）；不合法返回 400。
- **接口**：`PUT /api/users/{id}`，`which: ["favorites"]`。普通用户可以改自己的收藏夹。
- **共享拖放逻辑**：文件项和侧栏共用 `frontend/src/utils/drop.ts` 的 `dropSelectedInto`。

### 文件夹颜色

- **用法**：鼠标移到文件夹上，星号左边有一个调色板按钮，点开后选 10 种预设色之一，“默认”恢复原色。颜色显示在文件列表（列表和马赛克视图）和侧栏收藏夹的图标上。被选中的文件夹仍用原来的选中色，保证选中状态清晰。
- **色板**（`frontend/src/utils/folderColors.ts`）：red `#e53935`、pink `#d81b60`、purple `#8e24aa`、blue `#1e88e5`、teal `#00897b`、green `#43a047`、yellow `#f9a825`、orange `#fb8c00`、brown `#6d4c41`、grey `#757575`。
- **存储**：`users.User.FolderColors map[string]string`（JSON 字段 `folderColors`），键是和收藏夹相同格式的路径，值是 `#rrggbb`。
- **校验**（`cleanFolderColors`）：路径规则同收藏夹；颜色必须匹配 `^#[0-9a-f]{6}$`（大写会被转成小写，所以不能塞入任意样式字符串）；最多 500 个（`MaxFolderColors`）。
- **接口**：`PUT /api/users/{id}`，`which: ["folderColors"]`。

### 收藏夹与颜色共用的设计要点

- **为什么不放进 JWT**：前端的用户信息来自登录令牌（JWT）解码的 `userInfo`，里面没有这两个字段。把它们塞进令牌会超出 cookie 的 4 KB 上限（100 个路径）。
- **曾经出过的 bug**：每次进入子目录都会刷新令牌并用令牌里的用户整个覆盖当前用户，导致收藏夹消失。现在的做法（`frontend/src/stores/auth.ts`）：登录后调用一次 `loadUserData()` 读取这两个字段；`setUser()` 在令牌刷新时保留已加载的值，并且只在同一个用户 ID 下保留，避免串号；修改时先更新本地状态，保存失败则回滚。
- **已知限制**：它们都是按路径记录的。在 File Browser 里重命名或移动文件夹之后，收藏夹和颜色不会跟着走。

### 文件夹同步

在浏览目录时选中一个文件夹，配置同步到另一台机器，选择单向还是双向，**对方同意后才开始同步**；速度、进度和设置都可以在「设置 → 同步」里管理。

#### 架构

不自己写同步引擎，而是让 File Browser 充当 [Syncthing](https://syncthing.net) 的前端。同步由和 File Browser 并排运行的 Syncthing 容器完成，File Browser 负责界面、权限和路径校验。

```
 浏览器 ──► File Browser ──(REST, X-API-Key)──► Syncthing ◄══ 同步端口 ══► 对方的 Syncthing
            /api/sync/*       仅在 compose 内部网络         23000 (tcp+udp，仅 Tailscale 地址)
            (仅管理员)        Syncthing 的网页界面不对外发布
```

- `syncthing/client.go`：Syncthing REST API 的小客户端（设备、目录、状态、连接、待处理项、私有网络选项）。
- `http/sync.go`：`/api/sync/*` 处理器，负责校验和转发，不保存任何同步状态；状态全部在 Syncthing 里。
- `frontend/src/views/settings/Sync.vue`：设置页；`frontend/src/components/prompts/SyncFolder.vue`：在文件列表里发起同步的弹窗；`frontend/src/components/settings/SyncDeviceForm.vue`：添加设备表单。

#### 使用流程

1. **配对设备（一次）**：A 机器打开「设置 → 同步」，复制“本机设备 ID”。B 机器在“添加设备”里填这个 ID、名称，以及 A 的地址（例如 Tailscale IP，不写端口则用默认同步端口）。A 那边会在“等待你处理”里出现 B 的请求，点“接受”。
2. **共享文件夹**：在文件列表里选中一个文件夹，点顶栏的同步图标（“同步文件夹”）。选对方设备（可多选），选模式，决定是否保留回收站。
3. **对方接受**：对方的「设置 → 同步」里出现“某某共享了文件夹”，选保存位置（不存在会自动创建）和“仅接收”或“双向”，点“接受”。在此之前不会同步任何东西。
4. **管理**：“同步中的文件夹”里能看状态、暂停/继续、停止同步（只是停止，两边文件都保留）。

#### 模式与 Syncthing 目录类型

| 界面 | Syncthing `type` | 含义 |
|---|---|---|
| 单向 | `sendonly` | 本机为源，对方只接收，对方的改动不会回流 |
| 双向 | `sendreceive` | 两边的改动互相同步 |
| 仅接收 | `receiveonly` | 本机只接收，本地改动不发送（接受对方共享时的选项） |

发起共享时只能选单向或双向；接受共享时可以选“仅接收”或“双向”。发送方选了单向而接收方选了双向，接收方的改动仍不会回流，因为 `sendonly` 一侧不接受远端改动。

#### 目录配置

File Browser 创建的目录使用：ID 为 `fb-` 加 10 位随机十六进制；标签为文件夹名；路径为文件夹在磁盘上的真实路径；设备为本机加所选对方；开启文件监听（`fsWatcherEnabled`，延迟 3 秒）；每小时全量扫描一次；默认启用回收站版本管理（`trashcan`，保留 30 天，在目录内的 `.stversions`）。每个同步目录里会出现 `.stfolder` 标记目录，不要删。

#### 路径与安全校验

所有接口只有管理员能用（`withAdmin`）；没有配置 Syncthing 时返回 404。路径检查在 `http/sync.go` 的 `resolveSyncDir` / `resolveSyncDirPlan`：

- 目录必须在用户 scope 内，**不能是 scope 根**。
- 必须通过 File Browser 的访问规则（`d.Check`）。
- 必须存在且是目录（接受共享时目录不存在会先创建，创建前先做检查，被拒绝时不会在磁盘上留下任何东西）。
- **不能是符号链接，路径中任何一层也不能是符号链接**（用 `EvalSymlinks` 比较真实路径）。
- 不能和已有的同步目录相同、包含或被包含（Syncthing 处理不了嵌套）。
- 所选设备必须已经添加；设备 ID 必须匹配 `^[A-Z2-7]{7}(-[A-Z2-7]{7}){7}$`；名称 1–100 个字符；地址必须是 `host` 或 `host:port`（不接受 URL），会转成 `tcp://host:port`。
- 设备被某个同步目录使用时不能删除。
- 接受共享时，该共享必须真的在 Syncthing 的“待处理”列表里，并且来自指定设备。

#### 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/sync` | 汇总：本机设备 ID、默认端口、服务器时间、设备、目录、待处理的设备和目录 |
| POST | `/api/sync/devices` | 添加或更新设备 `{deviceID, name, address?}`，成功 204 |
| DELETE | `/api/sync/devices/{id}` | 删除设备 |
| POST | `/api/sync/folders` | 发起共享 `{path, deviceIDs[], mode: "oneway"\|"twoway", versioning?}`，返回 `{id}` |
| PATCH | `/api/sync/folders/{id}` | 暂停或恢复 `{paused}` |
| DELETE | `/api/sync/folders/{id}` | 停止同步（文件保留） |
| POST | `/api/sync/pending/folders/accept` | 接受对方共享 `{folderID, deviceID, path, mode: "receive"\|"twoway"}` |
| DELETE | `/api/sync/pending/folders?folder=&device=` | 忽略对方的目录共享 |
| DELETE | `/api/sync/pending/devices/{id}` | 忽略对方的设备请求 |

错误：输入问题返回 400，并带原因（Syncthing 自己拒绝的输入，例如设备 ID 校验位错误，也会作为 400 转发）；非管理员 403；未配置 Syncthing 404；Syncthing 连不上或 API key 不对 502。

#### 状态与速度

- 页面每 4 秒轮询 `/api/sync`。每个目录显示：已同步 / 同步中（剩余大小和文件数）/ 对方同步中（百分比和剩余）/ 扫描中 / 已暂停 / 离线 / 错误。
- **速度**：Syncthing 只给出每个连接自建立以来的累计收发字节数（`inBytesTotal` / `outBytesTotal`），所以速度由前端用相邻两次轮询的差值除以服务器时间差算出（`frontend/src/utils/syncRate.ts`）；计数器变小（重新连接）按 0 处理。设备表显示每台设备的 ↓/↑ 速度和累计传输量，目录卡片顶部显示总速度。刚打开页面的头几秒会是 0，需要两次采样。
- Syncthing 没有按目录统计的速度，所以速度只能按设备显示。

#### 配置项

只有设置了 `--syncthingUrl` 才会启用整个功能（设置页的“同步”标签才会出现）。这些参数只在运行时生效，不会写入数据库，API key 也不会发给浏览器。

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `--syncthingUrl` | `FB_SYNCTHING_URL` | 空（关闭） | Syncthing 的地址，例如 `http://syncthing:8384` |
| `--syncthingApiKey` | `FB_SYNCTHING_API_KEY` | 无 | 设置了 URL 时必填 |
| `--syncthingPort` | `FB_SYNCTHING_PORT` | `22000` | 添加设备时地址没写端口所用的同步端口；必须是 1–65535 |
| `--syncthingPrivateNetwork` | `FB_SYNCTHING_PRIVATE_NETWORK` | `false` | 启动时自动关闭 Syncthing 的全球发现、本地发现、中继、NAT 穿透和使用统计（`urAccepted=-1`）；Syncthing 比 File Browser 晚启动时会每 5 秒重试，最多 60 次 |

#### 端口

Syncthing 容器内部固定使用 22000（同步，TCP + QUIC/UDP）和 21027（本地发现，UDP）。compose 里只改**宿主机端口**，所以不需要改 Syncthing 的配置：

| `.env` 变量 | 默认 | 映射 |
|---|---|---|
| `SYNC_PORT` | `23000` | `<Tailscale IP>:SYNC_PORT` → 容器 22000（tcp 和 udp） |
| `DISCOVERY_PORT` | `23001` | `<Tailscale IP>:DISCOVERY_PORT` → 容器 21027（udp） |

- 所有机器要用同一个 `SYNC_PORT`，因为它也是添加设备时默认补上的端口（compose 把 `FB_SYNCTHING_PORT` 设成 `SYNC_PORT`）。
- 本地发现靠局域网广播，在 Tailscale 上不起作用，而且已被关闭，所以 `DISCOVERY_PORT` 只是按要求映射，实际没有流量。机器之间的连接只靠你填的地址和同步端口。
- Syncthing 的网页界面（8384）不对外发布，只在 compose 内部网络供 File Browser 访问。

#### 限制

- 目录路径必须在 File Browser 和 Syncthing 中完全相同，所以两个容器都把同一个宿主机目录挂到 `/srv`。
- 收藏夹和文件夹颜色不会随同步传到对方。不要同步 File Browser 自己的 `/database` 和 `/config`（数据库是 bolt 文件，被覆盖会损坏）。
- 接受对方共享时，保存路径要手动输入，没有目录选择器。
- 只有英文和简体中文翻译，其他语言回退到英文。
- 双向同步时，一边的删除会同步到另一边，所以建议保持回收站开启。
- 已在本机用两套 Syncthing + File Browser 容器完整测试过，包括配对、两种模式、暂停恢复、限速下的速度显示；没有在两台真实机器之间长期运行验证过。

### 多架构 Docker 构建与发布

- **`Dockerfile.multiarch`**：从源码构建，三个阶段。① 在构建机上用 `node:24-alpine` 构建前端；② 在构建机上用 `golang:1.25-alpine` 交叉编译 Go 二进制（`CGO_ENABLED=0`，`-trimpath`，`-ldflags "-s -w"`，带 `VERSION` 和 `COMMIT`）；③ `alpine:3.23` 最终镜像，带 `tini`、`su-exec`、`jq`、`ca-certificates`、`mailcap`。前端和 Go 编译都在构建机上原生运行，不需要 QEMU。
- **运行用户**：`docker/multiarch/entrypoint.sh` 按 `PUID` / `PGID` 创建用户后降权运行；`PUID=0` 时直接以 root 运行。它只对 `/config` 和 `/database` 做 `chown`，不碰 `/srv`。
- **QEMU**：最终阶段有一步 `apk add` 要在目标架构下执行，所以在 amd64 机器上构建 arm64 镜像之前，要先执行一次 `docker run --privileged --rm tonistiigi/binfmt --install all`；在 arm64 机器上原生构建则不需要。
- **发布流程**（每次改动后，`main` 为 amd64，`arm64` 分支为 arm64）：
  1. 在 `main` 上提交，合并进 `arm64` 并确认 `docker-compose.yml` 只差那 7 行。
  2. 本机 `docker login` 后构建并推送 amd64：`docker buildx build -f Dockerfile.multiarch --platform linux/amd64 --build-arg VERSION=2.63.23 --build-arg COMMIT=$(git rev-parse --short HEAD) -t q25459768/filebrowser:amd64 --push .`
  3. 把 `arm64` 分支的源码传到 arm 机器（`git archive arm64 | ssh -p 10086 100.64.0.30 'tar -x -C <dir>'`），在那里原生构建并推送 `:arm64`。
  4. `git push origin main arm64`。
  5. 在两台机器上 `docker compose -f docker-compose.yml pull && docker compose -f docker-compose.yml up -d`。
  - Docker Hub 的登录凭据不要写进仓库；用完 `docker logout`。

### 部署

两个服务：`filebrowser` 和 `syncthing`。

1. **首次**：创建 `.env`（见 `.env.example`）：`echo "SYNCTHING_API_KEY=$(openssl rand -hex 24)" > .env`。这个密钥由两个容器共用，缺少时 compose 会直接报错。`.env` 和 `syncthing-config/` 已在 `.gitignore` 里。
2. **启动**：`docker compose -f docker-compose.yml up -d`，然后打开 `http://<Tailscale IP>:38002`。
3. **`.env` 变量**：`SYNCTHING_API_KEY`（必填）、`SYNC_PORT`（默认 23000）、`DISCOVERY_PORT`（默认 23001）。
4. **数据**：`/root` 挂到 `/srv`（两个容器必须一致）；`./config`、`./database` 是 File Browser 的配置和数据库；`./syncthing-config` 是 Syncthing 的配置和设备密钥。
5. **镜像**：`syncthing/syncthing:2`（固定主版本，支持 amd64 和 arm64）。

#### 挂载 `/root` 的安全说明

- 实测结果：两个容器都**不会**改变挂载目录的属主或权限（`PUID=0` 和 `PUID=1000` 都试了，目录、`.ssh`、`authorized_keys` 的属主和权限全程不变）。Syncthing 的启动脚本只 `chown` 它自己的 `/var/syncthing`，不递归；File Browser 只 `chown` `/config` 和 `/database`。所以挂载 `/root` 本身不会破坏 ssh。
- `PUID=1000` 去访问属主为 root 的目录会得到 403。解决办法是保持 `PUID=0` / `PGID=0`，**不要**为此去 `chown` / `chmod` 家目录：sshd 对家目录和 `.ssh` 的属主、权限要求很严，一改就会让 ssh 异常。
- 因为 `/root` 被整体服务，里面的 `~/.ssh` 私钥原本会被 File Browser 读到，同步目录也可能被指向 `.ssh` 并覆盖 `authorized_keys`。所以 compose 在两个服务里都给 `/srv/.ssh` 盖了一个空的只读 `tmpfs`（`/srv/.ssh:ro,size=4k,mode=0555`）：容器里看到的是空目录，宿主机上的真实目录完全不动。实测读取私钥返回 404，写入失败。其他机密目录（例如 `.gnupg`）照样多加一行 `tmpfs` 即可。
- 上面的 compose 同时设置了 `FB_NOAUTH=true`：能连到这个端口的人不用登录就能读写被服务的整个目录。只走 Tailscale 时，风险取决于 tailnet 里有谁和 ACL 怎么设；不要用 Tailscale Funnel 把它暴露到公网。

### 数据与代码改动清单

- 后端新增：`syncthing/`（客户端）、`http/sync.go`、`http/users_favorites_test.go`、`http/sync_test.go`、`users/favorites_test.go`；修改 `users/users.go`（`Favorites`、`FolderColors` 及校验）、`http/users.go`（校验失败返回 400）、`http/http.go`（注册 `/api/sync`）、`http/static.go`（向前端传 `EnableSync`）、`settings/settings.go` 和 `cmd/root.go`（运行时的 Syncthing 参数）。
- 前端新增：`api/sync.ts`、`types/sync.d.ts`、`utils/{drop,favorites,folderColors,syncRate}.ts`、`components/prompts/{FolderColor,SyncFolder}.vue`、`components/settings/SyncDeviceForm.vue`、`views/settings/Sync.vue`；修改 `stores/auth.ts`、`Sidebar.vue`、`ListingItem.vue`、`FileListing.vue` 等。
- 部署新增：`Dockerfile.multiarch`、`docker/multiarch/entrypoint.sh`、`docker-compose.yml`、`.env.example`、`compose.multiarch.yaml`。

### 开发与测试

```bash
go build ./... && go vet ./... && go test ./...          # 后端
cd frontend && pnpm install --frozen-lockfile
pnpm run typecheck && pnpm run lint && pnpm test          # 前端（vitest）
pnpm run build                                            # 构建前端到 frontend/dist，再 go build 即可得到带界面的二进制
```

- **后端测试**：`users/favorites_test.go`（收藏夹和颜色的规范化与校验）、`http/users_favorites_test.go`（普通用户可以改、非法输入返回 400）、`http/sync_test.go`（用内存里的假 Syncthing 服务器，覆盖权限、路径校验、符号链接、重叠、设备管理、接受共享、状态和速度字段、错误映射）。对符号链接防护做过变异测试：去掉防护后对应用例会失败。
- **前端测试**：`utils/__tests__/favorites.test.ts`、`stores/__tests__/auth-favorites.test.ts`（含“令牌刷新后收藏夹仍在”的回归测试）、`utils/__tests__/sync-rate.test.ts`。
- **端到端**：用 Docker 起两套真实的 Syncthing + File Browser，再用 Playwright 驱动浏览器，把配对、共享、接受、同步、暂停恢复、拖放到收藏夹、收藏夹和颜色在子目录跳转和刷新后是否保留、传输速度显示都走过一遍。这部分是手工脚本，没有放进仓库。
- 测试时要注意：Syncthing 默认不对局域网地址限速，在 Docker 内网里测速度要同时设置 `limitBandwidthInLan: true`。

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

Documentation on how to install, configure, and build this project lives in [`docs`](docs) in this repository.

[CONTRIBUTING.md](CONTRIBUTING.md) documents how to build and develop the project, which remains useful to anyone forking it.

## License

[Apache License 2.0](LICENSE) © File Browser Contributors
