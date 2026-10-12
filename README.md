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

### 功能总览

**浏览与整理**

| 功能 | 怎么用 | 详情 |
|---|---|---|
| 剪切 / 粘贴按钮 | 选中文件后点顶栏的 ✂ 剪切，到目标文件夹点粘贴 | [详情](#剪切--粘贴按钮) |
| 收藏夹 | 文件夹上点星号，出现在侧栏，可以直接把文件拖进去（移动，Ctrl/Cmd 为复制） | [详情](#收藏夹) |
| 文件夹颜色 | 文件夹上的调色板按钮，10 种预设色 | [详情](#文件夹颜色) |
| 双栏模式 | 侧栏“双栏”：左右两个文件夹并排，拖放、F5 复制、F6 移动 | [详情](#双栏模式) |
| 批量重命名 | 选中多个项目 → “批量重命名”：查找替换（支持正则）、前后缀、大小写、编号，带实时预览 | [详情](#批量重命名与复制一份) |
| 复制一份 | 选中项目 → “复制一份”，在原位置生成 `名称(1)` | [详情](#批量重命名与复制一份) |
| 压缩 / 解压 | 选中项目 → “压缩”（zip、tar.gz、tar.xz、tar.zst 等）；选中压缩包 → “解压” | [详情](#压缩与解压) |
| 磁盘占用 | 顶栏“磁盘占用”：按大小显示每个子项占多少，逐层钻取 | [详情](#磁盘占用) |

**安全与找回**

| 功能 | 怎么用 | 详情 |
|---|---|---|
| 回收站 | 删除先进回收站，可还原；设置里可开关，并限制占用空间和保留天数 | [详情](#回收站) |
| 活动日志 | 设置 → 活动：谁在什么时候从哪里做了什么 | [详情](#活动日志) |
| 右键权限 | 右键 → 权限：修改 chmod / chown，可递归 | [详情](#右键权限chmod--chown) |
| 隐藏 `~/.ssh` | 容器里把 `/srv/.ssh` 盖成空目录 | [详情](#挂载-root-的安全说明) |

**多机同步**

| 功能 | 怎么用 | 详情 |
|---|---|---|
| 文件夹同步 | 选中文件夹 → “同步文件夹”，选对方设备和单向/双向，对方在“设置 → 同步”里接受后生效 | [详情](#文件夹同步) |
| 同步状态与速度 | 设置 → 同步：每个目录的状态、剩余量，每台设备的实时速度和累计传输量 | [详情](#状态与速度) |
| 同步冲突处理 | 两台设备改了同一个文件时，选择保留哪个版本，另一个进回收站 | [详情](#冲突忽略规则限速与标记) |
| 忽略规则 | 每个同步目录可以编辑 `.stignore` 风格的忽略规则 | [详情](#冲突忽略规则限速与标记) |
| 同步限速 | 设置 → 同步 → 带宽：下载 / 上传限速 | [详情](#冲突忽略规则限速与标记) |
| 同步标记 | 文件列表里已同步的文件夹带一个状态图标 | [详情](#冲突忽略规则限速与标记) |

**编辑**

| 功能 | 怎么用 | 详情 |
|---|---|---|
| 文本编辑器 | 16px JetBrains Mono，按语言缩进，Alt+Shift+F 格式化（YAML、JSON、JS/TS、CSS、HTML、Vue、Markdown、Python、SQL、Go），保存时格式化，离线语法高亮和检查 | [详情](#文本编辑器) |

**部署与发布**

| 功能 | 怎么用 | 详情 |
|---|---|---|
| 多架构 Docker | 一份 `Dockerfile.multiarch` 构建 amd64 和 arm64 镜像 | [详情](#多架构-docker-构建与发布) |
| docker-compose + `.env` | 一份 compose 两个架构通用；架构、IP、端口、挂载目录、设备名都在 `.env` 里，`scripts/init-env.sh` 一键生成 | [详情](#部署) |
| 版本号与变更记录 | 镜像带版本 tag 和变更标签，git 打标签，一条命令发布两个架构 | [详情](#多架构-docker-构建与发布) |

### 分支、镜像与机器

| 分支 | 机器 | 镜像 | Tailscale 地址 | `.env` 里的 `ARCH` |
|---|---|---|---|---|
| `main` | amd64 | `q25459768/filebrowser:latest`（amd64 镜像） | `100.64.0.7` | `amd64` |
| `arm64` | arm64 | `q25459768/filebrowser:latest`（arm64 镜像） | `100.64.0.30` | `arm64` |

- **两台机器用同一份 `docker-compose.yml`**：镜像 tag 是同时含 amd64 和 arm64 的多架构 tag（`latest`），`platform` 由 `.env` 里的 `ARCH` 决定，所以不会再拿到另一个架构的镜像。每台机器自己的值（架构、Tailscale 地址、端口、目录、设备名）都在不提交的 `.env` 里，见[部署](#部署)。
- `arm64` 分支和 `main` 的内容现在是一样的；它只是发布脚本用来在 arm 机器上原生构建的来源（`git archive arm64`）。修改 `main` 之后把它合并进 `arm64` 即可，不需要再改 compose。
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

### 回收站

- **用法**：删除文件或文件夹时先移到回收站。删除确认框里有“永久删除（跳过回收站）”选项。侧栏的“回收站”页面列出所有已删除的项目（名称、原位置、大小、删除时间），可以**还原**、**永久删除**或**清空回收站**。
- **在设置里开关和限制空间**：设置 → 全局设置 → 回收站：“删除时使用回收站”开关，“最多保留（GB）”（0 不限制），“保留天数”（0 表示一直保留到需要空间）。没有设置过时的默认值是：开启、10 GiB、30 天。
- **存储方式**：每个用户 scope 下的 `/.trash/<id>/data` 是被删除的文件或文件夹，`/.trash/<id>/meta.json` 记录原路径、名称、是否文件夹、大小、删除时间和操作者。同一文件系统内是改名，所以很快；大小在删除时统计（最多花 10 秒，超过则只统计一部分作为估值）。
- **空间限制**：每次放入和每次打开回收站页面时，先清理超过保留天数的项目，再从最旧的开始清理，直到总大小不超过上限。**单个项目比整个上限还大时不会进回收站，而是直接永久删除**，并提示“太大，放不进回收站”（响应头 `X-Trash: deleted`，放进去的是 `trashed`）。
- **还原**：还原到原路径，原来的文件夹不存在会自动创建；原位置已经有同名项目时返回 409，界面会问是否在旁边还原为副本（`名称(1).扩展名`）。
- **其他**：在 `/.trash` 里删除是永久删除；回收站是普通文件夹，在文件列表里也能看到；需要有 `delete` 权限，还原还需要 `create` 权限。同步冲突里被丢弃的版本也放进回收站。
- **接口**：`GET /api/trash`（列表、总大小和当前设置）、`POST /api/trash/{id}/restore[?rename=true]`、`DELETE /api/trash/{id}`、`DELETE /api/trash`（清空）；`DELETE /api/resources/{path}[?permanent=true]` 删除。设置在 `GET/PUT /api/settings` 的 `trash` 字段（`{enabled, maxBytes, days}`）。
- **注意**：在双向同步的目录里删除文件，文件会离开同步目录，对方也会删除它（对方由 Syncthing 的回收站版本管理保护）；在本机回收站里还原会让它重新同步回去。

### 活动日志

- **用法**：设置 → 活动（仅管理员）：时间、用户、操作、路径（移动、复制、重命名会显示 `原路径 → 新路径`）、详情、来源地址；可以按操作类型和用户筛选，按路径或详情搜索，“显示更多”翻页，可勾选每 5 秒自动刷新。
- **记录的操作**：新建文件夹、上传、保存、删除（含移入回收站）、重命名、移动、复制、分享、登录和登录失败、修改权限、还原、从回收站删除、压缩、解压，以及同步的发起、接受、停止、暂停、继续、添加和移除设备、处理冲突、修改忽略规则和限速。失败的操作不记录；没有认证（`FB_NOAUTH`）时每次打开页面都会“登录”，所以不记录登录。
- **存储**：放在 File Browser 自己的数据库里（`activity` 包，bolt 后端）。保留天数由 `--activityDays`（环境变量 `FB_ACTIVITY_DAYS`，默认 90，0 关闭记录）决定，最多 50000 条，每写入 200 条清理一次过期的。
- **接口**：`GET /api/activity?user=&action=&q=&limit=&offset=`（管理员，新的在前，返回 `{items, total}`）。

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
3. **对方接受**：对方的「设置 → 同步」里出现“某某共享了文件夹”，选保存位置（不存在会自动创建；输入时有**路径自动补全**，见下）和“仅接收”或“双向”，点“接受”。在此之前不会同步任何东西。
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

#### 冲突、忽略规则、限速与标记

- **同步标记**：文件列表里，已同步的文件夹（同步目录的根）左上角有一个同步图标：绿色已同步、蓝色同步中、灰色暂停或对方离线、红色出错。只对管理员显示，每 8 秒更新一次状态。
- **冲突**：两台设备改了同一个文件，Syncthing 会把其中一个版本改名为 `名称.sync-conflict-日期-时间-设备.扩展名`。设置 → 同步里的“冲突”卡片列出这些文件（只在同步目录里找，跳过 `.stfolder` 和 `.stversions`，最多 1000 个），显示本机版本和对方版本的大小和时间，可以选**保留本机的**（删除冲突文件）或**保留对方的**（对方的版本取代原文件）；被丢弃的那个版本放进回收站（回收站关闭时直接删除）。接口：`GET /api/sync/conflicts`、`POST /api/sync/conflicts/resolve {path, keep: "conflict"|"original"}`。
- **忽略规则**：每个同步目录行上的“忽略规则”按钮打开编辑框，每行一个规则（`*.tmp`、`node_modules`、`/build`、`(?d).DS_Store`，`!` 开头表示重新包含，`//` 开头是注释）。被忽略的文件既不发送也不接收。服务器拒绝 `#include`（它会让 Syncthing 读取其他文件），最多 1000 行、每行 500 个字符。接口：`GET/PUT /api/sync/folders/{id}/ignores`（`{lines: []}`）。
- **限速**：设置 → 同步 → 带宽：下载限速和上传限速（KiB/s，0 不限），以及“局域网内的连接也限速”（Tailscale 地址在 Syncthing 看来属于局域网，要让限速生效必须勾选）。接口：`GET/PUT /api/sync/limits`（`{maxRecvKbps, maxSendKbps, limitBandwidthInLan}`，0–10000000）。

#### 配置项

只有设置了 `--syncthingUrl` 才会启用整个功能（设置页的“同步”标签才会出现）。这些参数只在运行时生效，不会写入数据库，API key 也不会发给浏览器。

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `--syncthingUrl` | `FB_SYNCTHING_URL` | 空（关闭） | Syncthing 的地址，例如 `http://syncthing:8384` |
| `--syncthingApiKey` | `FB_SYNCTHING_API_KEY` | 无 | 设置了 URL 时必填 |
| `--syncthingPort` | `FB_SYNCTHING_PORT` | `22000` | 添加设备时地址没写端口所用的同步端口；必须是 1–65535 |
| `--syncthingPrivateNetwork` | `FB_SYNCTHING_PRIVATE_NETWORK` | `false` | 启动时自动关闭 Syncthing 的全球发现、本地发现、中继、NAT 穿透和使用统计（`urAccepted=-1`）；Syncthing 比 File Browser 晚启动时会每 5 秒重试，最多 60 次 |
| `--activityDays` | `FB_ACTIVITY_DAYS` | `90` | 活动日志保留的天数，0 关闭记录（见[活动日志](#活动日志)） |

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

### 右键权限（chmod / chown）

- **用法**：右键点击文件或文件夹（也可以多选），选“权限”（锁形图标，顶栏和手机的选择条里也有）。弹窗里有读/写/执行 × 所有者/所属组/其他人的勾选格和八进制输入框（两者联动）；管理员还能改所有者和所属组（填服务器上已有的名称或数字 ID，不改就留空）；对文件夹可以勾选“同时应用到文件夹内的所有内容”，并分别指定里面文件和里面文件夹的权限（默认文件为去掉执行位的权限，文件夹为同样的权限）。没改任何东西时“应用”按钮是灰的。
- **接口**（`http/permissions.go`）：`GET /api/permissions/{path}` 返回 `{path, isDir, mode, uid, gid, owner, group, canChown}`；`PATCH /api/permissions/{path}`，body 为 `{mode?, recursive?, fileMode?, dirMode?, owner?, group?}`，返回 `{changed, skipped, failed, errors[]}`。`mode` 作用于项目自身；`fileMode` / `dirMode` 只在 `recursive` 时作用于里面的文件和文件夹。
- **权限范围**：改 mode 需要用户有 `modify` 权限；改 owner / group 只允许管理员，并且服务器进程必须是 root（否则 `canChown` 为 false，界面里不显示这两项）；路径必须通过访问规则（`d.Check`）。
- **安全限制**：
  - **拒绝 scope 根**（例如 `/root` 本身）：改家目录的权限会让 sshd 拒绝登录。
  - **拒绝符号链接**：目标本身是符号链接、或路径经过符号链接，都返回 400；递归时符号链接被跳过且不会被跟随（chown 用 `Lchown`）。
  - **只允许 9 个 rwx 位**（`0`–`0777`）：setuid、setgid、sticky 位会被拒绝，原有的这些位会被清掉。
  - 递归时被访问规则隐藏的条目会被跳过；最多处理 200000 个条目（`maxPermissionEntries`）；先收集再从最深处开始应用，这样收紧文件夹权限不会妨碍处理它里面的内容。
  - 每次修改都会写日志：`permissions: user "x" changed /path mode=... recursive=...: N changed, M skipped, K failed`。
  - Windows 构建不支持改所有者（`permissions_windows.go` 是桩实现）。
- **限制**：多选时显示的是第一个项目的值，会应用到所有选中的项目；名称按服务器（容器）里的用户库解析，宿主机上的用户请填数字 ID；不处理 ACL 和扩展属性。注意：在双向同步的目录里改权限，Syncthing 默认会把权限位同步到对方（所有者不会）。

### 文本编辑器

在现有的 Ace 编辑器（`frontend/src/views/files/Editor.vue`）上做了改进：

- **默认外观**：16px（原来 14px，仍可用 ＋/－ 调整并记在浏览器里）；字体是打包进应用的 JetBrains Mono（拉丁字符，400 和 700），后备 Fira Code、Cascadia Code、SF Mono、Menlo、Consolas；行高 1.6；缩进参考线；允许滚动到末尾之后。
- **按语言缩进**（`frontend/src/utils/editor/formatters.ts` 的 `indentFor`）：YAML、JSON、JS/TS、CSS/SCSS/Less、HTML、Vue、Markdown、XML、TOML 用 2 个空格；Python、Rust、Java、Kotlin、C/C++、C# 用 4 个空格；Go 和 Makefile 用 Tab；其他 4 个空格。
- **格式化**：顶栏的“格式化”按钮（魔棒图标）或快捷键 **Alt+Shift+F**；编辑器右上还有“保存时格式化”开关（默认关，记在浏览器里的 `editorFormatOnSave`）。格式化是一次可撤销的编辑，光标尽量保持在原位；文件有语法错误时只提示原因，不会改动内容，保存也不会被阻止。没有格式化器的类型（例如 `.txt`）不显示按钮。

| 后缀 | 格式化器 | 运行位置 |
|---|---|---|
| `yml` `yaml` | Prettier（yaml） | 浏览器 |
| `json` `jsonc` `json5` | Prettier | 浏览器 |
| `js` `jsx` `mjs` `cjs` `ts` `tsx` `mts` `cts` | Prettier（babel / typescript） | 浏览器 |
| `css` `scss` `less` | Prettier（postcss） | 浏览器 |
| `html` `htm` `vue` | Prettier | 浏览器 |
| `md` `markdown` | Prettier | 浏览器 |
| `py` `pyi` | Ruff（WebAssembly，行宽 88） | 浏览器 |
| `sql` | sql-formatter（关键字大写） | 浏览器 |
| `go` | gofmt（标准库 `go/format`），`POST /api/format/go` | 服务器 |

- Prettier 行宽 100；各格式化器的代码都是按需动态加载的，打开编辑器时只下载当前文件需要的部分。Ruff 的 wasm 约 10.5 MB，只在第一次格式化 Python 时才下载，之后由浏览器缓存。
- **不支持**：shell、TOML、XML、Rust 等（没有合适的浏览器端格式化器）；可以以后再加。
- **离线语法高亮与语法检查**：原来的编辑器把 Ace 的语言模式、主题和 worker 设成从 `cdn.jsdelivr.net` 加载，但应用的内容安全策略（`default-src 'self'`）会拦截它，而且私有网络本来就可能没有外网。现在 `vite.config.ts` 里的 `aceAssets` 插件把需要的压缩版文件原样复制到 `assets/ace/`（约 60 种语言模式及其代码片段、全部 48 个主题、7 个 worker：css、html、javascript、json、lua、xml、yaml），`aceAssets.ts` 负责注册地址。列表之外的语言仍可打开，按纯文本显示。列表在 `frontend/src/utils/editor/aceFiles.ts`。
- **体积**：前端产物从约 12 MB 增加到约 35 MB（Ruff wasm 10.5 MB、Ace 资源约 6 MB、Prettier 各语言插件和字体），Go 二进制相应变大，但这些文件都是按需下载的。
- **顺带的修复**：静态文件处理器原来对所有 `.js` 文件只提供预压缩的 `.gz` 副本，没有副本就 404；构建会跳过极小的文件，所以这种脚本（例如导出 wasm 地址的小块）会 404。现在没有 `.gz` 时回退到原文件（`http/static_test.go`）。

### 磁盘占用

- **用法**：文件列表顶栏的“磁盘占用”（圆环图标），对当前文件夹，或选中的那一个文件夹。页面按大小从大到小列出每个子项：横条表示占整个文件夹的比例，显示大小、百分比和文件数；点文件夹逐层钻取，点面包屑回到上层，“在文件中打开”回到文件视图。
- **实现**（`http/diskusage.go`）：`GET /api/du/{path}` 并发测量（4 个）每个子文件夹；不跟随符号链接；遵守访问规则（被规则禁止的项目不统计也不显示），点开头的文件和 `.trash` 都计入；整个请求最多 30 秒，没算完的文件夹标记为“不完整”（显示 `≥`）；完整的结果缓存 2 分钟（最多 5000 项），所以钻取和返回很快。
- **限制**：大小是文件内容的大小（不是磁盘块占用）；硬链接会重复计算。

### 路径自动补全

同步页“接受共享”的保存位置和解压对话框的目标文件夹，输入时会提示已有的文件夹（`components/PathInput.vue`，逻辑在 `utils/pathSuggest.ts`）：

- 输入 `/Back` 会列出根目录下以 `Back` 开头的文件夹（不区分大小写，自然排序，最多 12 个，点开头的文件夹只在输入点时才提示）；选中后路径以 `/` 结尾并立刻列出下一层。
- **↑ / ↓** 选择，**Tab** 或 **Enter** 采用，**Esc** 只关闭提示列表（不会关掉弹窗），也可以直接点。
- 只提示文件夹；上一级不存在或读不了时安静地不提示；同一个文件夹的列表缓存 10 秒，输入有 120 毫秒的防抖，不会每个字符都请求。
- 路径是相对用户 scope 的，和服务器接收的格式一致；首尾空格在提交时才去掉。

### 压缩与解压

- **压缩**：选中文件或文件夹 → “压缩”，输入名称、选格式：zip、tar、tar.gz、tar.bz2、tar.xz、tar.zst。压缩包建在当前文件夹，重名时自动加 `(1)`；选中的项目里有嵌套的，只打包外层；压缩包不会把自己打进去。
- **解压**：选中压缩包 → “解压”，默认解压到旁边一个以压缩包命名的新文件夹（可以改），可勾选“替换已存在的文件”。支持 zip、tar（及 gz、bz2、xz、zst）、7z、rar。
- **安全**（`http/archive.go`）：接口 `POST /api/archive`，`{action: "compress"|"extract", ...}`。解压时拒绝含 `..` 的条目（含反斜杠写法）、把绝对路径当作相对路径处理、**不创建符号链接和硬链接**、不处理设备等特殊文件、遵守访问规则、**默认不覆盖已有文件**；总解压大小限制 20 GiB、条目数限制 100 万，超出时返回 413，并且删除这次解压新建的文件夹，不留下一半的内容；不是合法压缩包返回 400；需要 `create` 权限（压缩还要能读取）。跳过的条目会在结果里报告数量和前几条原因。
- **限制**：同步执行，很大的压缩包会让这个请求持续较久（关闭页面会取消）；解压出来的文件用服务器设置的文件和文件夹权限，不保留压缩包里的权限和所有者。

### 批量重命名与复制一份

- **批量重命名**：选中两个以上项目 → “批量重命名”：查找和替换（可选正则表达式、区分大小写）、名称前后加文字、字母大小写（小写、大写、首字母大写）、编号（起始值、步长、位数、分隔符、放在开头或结尾）。规则只作用于名称部分，文件的扩展名保持不变；规则按这个顺序依次应用。下面的预览表实时显示每个项目的新名字，**有问题时不能应用**：名称为空或含 `/`、过长、两个项目得到同一个名字、与未选中的已有项目重名、正则无效。
- **互换名字**：当某个新名字正好是另一个被重命名项目的当前名字时，先改成临时名字再改成最终名字（`utils/batchRename.ts` 的 `renameSteps`），不会互相覆盖。
- **复制一份**：选中项目 → “复制一份”，在原位置生成副本，已有同名时自动加 `(1)`、`(2)`。

### 双栏模式

- **用法**：侧栏的“双栏”（或文件列表顶栏的双栏图标，会把当前文件夹放在左栏）。左右各一个文件夹，点哪个栏就在哪个栏里操作（蓝色边框是当前栏）。
- **拖放**：把选中的项目拖到另一个栏即**移动**，按住 **Ctrl / Cmd** 为**复制**；拖到某个文件夹行上就放进那个文件夹。同名时弹出和文件列表一样的冲突处理框。
- **按钮和快捷键**：顶栏有“复制到另一栏”“移动到另一栏”“新建文件夹”“删除”“交换两栏”“刷新”。键盘：Tab 切换栏、F5 复制、F6 移动、F7 新建文件夹、Delete 删除（走回收站）、Backspace 上一级、Enter 打开、Ctrl/Cmd+A 全选、上下方向键移动选择。选择：单击、Ctrl/Cmd 单击切换、Shift 单击选择范围；双击文件夹进入，双击文件在文件视图里打开。表头点击排序（名称、大小、修改时间，文件夹始终在前）。
- **其他**：两栏的路径记在浏览器里（`dualPanePaths`）；窄屏下两栏上下排列。它是独立的页面（`views/DualPane.vue`），不使用文件列表的选择状态；移动和复制复用 `utils/drop.ts` 的 `transferItems`。

### 多架构 Docker 构建与发布

- **`Dockerfile.multiarch`**：从源码构建，三个阶段。① 在构建机上用 `node:24-alpine` 构建前端；② 在构建机上用 `golang:1.25-alpine` 交叉编译 Go 二进制（`CGO_ENABLED=0`，`-trimpath`，`-ldflags "-s -w"`，带 `VERSION` 和 `COMMIT`）；③ `alpine:3.23` 最终镜像，带 `tini`、`su-exec`、`jq`、`ca-certificates`、`mailcap`。前端和 Go 编译都在构建机上原生运行，不需要 QEMU。
- **运行用户**：`docker/multiarch/entrypoint.sh` 按 `PUID` / `PGID` 创建用户后降权运行；`PUID=0` 时直接以 root 运行。它只对 `/config` 和 `/database` 做 `chown`，不碰 `/srv`。
- **QEMU**：最终阶段有一步 `apk add` 要在目标架构下执行，所以在 amd64 机器上构建 arm64 镜像之前，要先执行一次 `docker run --privileged --rm tonistiigi/binfmt --install all`；在 arm64 机器上原生构建则不需要。
- **版本号与变更记录**：版本形如 `<上游版本>-fork.<n>`（当前见仓库根目录的 `VERSION` 文件），它同时是：侧栏显示的版本、Docker 镜像的版本 tag、git 标签 `v<版本>`。每个版本在 `CHANGES.md` 里要有一节（`## <版本> (日期)` 加若干 `- ` 条目），`scripts/release.sh` 没有对应条目就会拒绝发布。每个版本带这些 tag 和标签：
  - `q25459768/filebrowser:<版本>` 和 `:latest`：**多架构 tag**（同时含 amd64 和 arm64，Docker 按平台自动选），`docker-compose.yml` 默认用 `latest`；
  - `:<版本>-amd64` / `:<版本>-arm64`：单一架构、固定版本，是上面多架构 tag 的组成部分，也可用于回滚；
  - `:amd64` / `:arm64`：单一架构的滚动 tag（保留，旧的用法仍然有效）；
  - 镜像标签 `org.opencontainers.image.version`、`org.opencontainers.image.revision`（提交）和 `com.filebrowser.fork.changes`（该版本的变更，一行，用 `; ` 分隔）：`docker inspect --format '{{ index .Config.Labels "com.filebrowser.fork.changes" }}' q25459768/filebrowser:<版本>-amd64`。
  - 想固定某个版本时，把 `docker-compose.yml` 里 File Browser 的 `image:` 改成 `q25459768/filebrowser:<版本>`（多架构 tag，两个架构通用，例如 `2.63.23-fork.2`）。
- **发布流程**（`main` 为 amd64，`arm64` 分支为 arm64）：
  1. 在 `main` 上改好代码，把 `VERSION` 加一，并在 `CHANGES.md` 里写这个版本的变更，提交。
  2. 把 `main` 合并进 `arm64` 分支（两个分支内容相同，不需要再改 compose）。
  3. 在 amd64 机器、`main` 分支上执行：`DOCKERHUB_USER=... DOCKERHUB_TOKEN=... scripts/release-all.sh`。脚本会检查分支、工作区干净、`arm64` 已合并 `main`、两个分支的 `VERSION` 一致、标签尚不存在；然后本机构建并推送 amd64，把 `arm64` 分支的源码用 `git archive` 传到 arm 机器（`ARM_HOST`，默认 `100.64.0.30`，ssh 端口 `ARM_PORT`，默认 `10086`）原生构建并推送 arm64；令牌通过标准输入传递，不出现在命令行，结束后两台机器都会 `docker logout`；然后创建多架构 tag（`<版本>` 和 `latest`，指向两个架构的镜像）；最后创建带变更说明的 git 标签并推送 `main`、`arm64` 和标签。
  4. 在两台机器上 `docker compose -f docker-compose.yml pull && docker compose -f docker-compose.yml up -d`。
  - `scripts/release.sh amd64|arm64` 可以单独构建并推送一个架构；`scripts/changes.sh <版本>` 打印某个版本的变更。
  - Docker Hub 的令牌不要写进仓库，用完即换。

### 部署

两个服务：`filebrowser` 和 `syncthing`。**每台机器自己的值都在 `.env` 里**（不提交，已在 `.gitignore`），`docker-compose.yml` 只引用变量。

1. **首次**：在仓库目录里运行 `scripts/init-env.sh`。它会探测架构（`uname -m`）和 Tailscale 地址（`tailscale ip -4`），生成 Syncthing 密钥，用主机名作设备名，写出 `.env`（权限 600），**不会覆盖已有的 `.env`**。探测不到时可以手动给：`HOST_IP=100.64.0.7 DATA_DIR=/root SYNCTHING_NAME=aus-main-debian scripts/init-env.sh`。也可以 `cp .env.example .env` 自己编辑。缺少必填的变量时 compose 会直接报错并说明。
   - **不要用没加引号的 heredoc（`cat > docker-compose.yml << EOF`）来写 compose 文件或 `.env`**：shell 会在写文件的时候把里面所有的 `${...}` 替换掉。变量没设置时，端口会变成 `":38002:80"`（报 “ports contains an invalid type”），`platform` 和镜像 tag 会被写死成 amd64（在 arm64 机器上就是 `exec /sbin/tini: exec format error`）。需要用 heredoc 时写成 `<< 'EOF'`（引号），或者直接用上面的脚本。
2. **启动**：`docker compose -f docker-compose.yml up -d`，然后打开 `http://<HOST_IP>:<WEB_PORT>`，默认 `http://100.64.0.7:38002`。
3. **`.env` 变量**（compose 里没有默认值，**全部都要写在 `.env` 里**；`SYNCTHING_API_KEY` 由 `scripts/init-env.sh` 生成，示例文件里没有这一行）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `ARCH` | **必填** | 这台机器的架构：`amd64` 或 `arm64`（`uname -m` 是 `x86_64` / `aarch64`）；决定两个服务的 `platform` |
| `HOST_IP` | **必填** | 这台机器的 Tailscale 地址；所有端口都只发布在它上面（忘了填会报错，不会退回到监听所有网卡） |
| `SYNCTHING_API_KEY` | **必填** | File Browser 和 Syncthing 共用的密钥 |
| `DATA_DIR` | `/root` | 宿主机上被服务和同步的文件夹，两个容器都挂到 `/srv`（必须是同一个） |
| `WEB_PORT` | `38002` | File Browser 的网页端口 |
| `SYNCTHING_GUI_PORT` | `38003` | Syncthing 自带网页界面的端口（映射到容器的 8384） |
| `SYNC_PORT` | `23000` | 同步端口（TCP + UDP），所有机器用同一个值；添加设备时地址没写端口就用它 |
| `DISCOVERY_PORT` | `23001` | 本地发现端口（UDP，发现已被关闭，所以没有流量） |
| `SYNCTHING_NAME` | 无 | 对方看到的设备名，**每台机器要不同** |
| `PUID` / `PGID` | `0` / `0` | 容器运行的用户；服务 root 拥有的文件需要 0 |

4. **数据**：`DATA_DIR` 挂到 `/srv`；`./config`、`./database` 是 File Browser 的配置和数据库；`./syncthing-config` 是 Syncthing 的配置和设备密钥。
5. **镜像**：`syncthing/syncthing:2`（固定主版本，支持 amd64 和 arm64）；两个服务都写了 `platform`。
6. **Syncthing 的网页界面**：发布在 `<HOST_IP>:<SYNCTHING_GUI_PORT>`，方便你查看或手动调整。**它在你设置密码之前没有任何保护**（在它的“设置 → GUI”里设置）。File Browser 用 API 密钥访问它，设置密码不影响。
7. **常见错误**：`exec format error` 是镜像架构和机器不符，检查 `.env` 里的 `ARCH` 和 compose 文件里有没有被 heredoc 写死；`ports contains an invalid type` 是 `HOST_IP` 等变量为空（同样先检查 compose 文件是不是被 heredoc 展开过）；`.env` 里的注释请单独占一行。`hostname must be a string`（或别的变量“必须是字符串”）说明某个变量解析成了空值：`.env` 没有被读到（它必须和 `docker-compose.yml` 在同一个目录，文件名就是 `.env`，或者用 `--env-file` 指定），或者当前 shell 里导出了同名的空变量（shell 优先于 `.env`，用 `unset` 清掉）。用 `docker compose -f docker-compose.yml config --environment | grep -E "ARCH|HOST_IP|SYNCTHING"` 可以看到 compose 实际读到的值。注意：compose 里的变量**没有默认值，也没有必填检查**，缺失的变量只会得到一条 “Defaulting to a blank string” 的警告，然后变成空字符串（例如 `PUID` 为空时容器会退回到用户 1000，读不了 root 的文件）。所以 `.env` 必须把下面表里的变量都写全。引号、等号两边的空格、Windows 换行不会造成问题。
8. **从旧版本升级**：旧的 `.env` 里没有 `ARCH` 和 `HOST_IP`，需要补上；以前写死在 compose 里的 `100.64.0.7`、`/root`、`38002`、`23000` 等现在都是 `.env` 变量的默认值或需要你填的值。

#### 挂载 `/root` 的安全说明

- 实测结果：两个容器都**不会**改变挂载目录的属主或权限（`PUID=0` 和 `PUID=1000` 都试了，目录、`.ssh`、`authorized_keys` 的属主和权限全程不变）。Syncthing 的启动脚本只 `chown` 它自己的 `/var/syncthing`，不递归；File Browser 只 `chown` `/config` 和 `/database`。所以挂载 `/root` 本身不会破坏 ssh。
- `PUID=1000` 去访问属主为 root 的目录会得到 403。解决办法是保持 `PUID=0` / `PGID=0`，**不要**为此去 `chown` / `chmod` 家目录：sshd 对家目录和 `.ssh` 的属主、权限要求很严，一改就会让 ssh 异常。
- 因为 `/root` 被整体服务，里面的 `~/.ssh` 私钥原本会被 File Browser 读到，同步目录也可能被指向 `.ssh` 并覆盖 `authorized_keys`。所以 compose 在两个服务里都给 `/srv/.ssh` 盖了一个空的只读 `tmpfs`（`/srv/.ssh:ro,size=4k,mode=0555`）：容器里看到的是空目录，宿主机上的真实目录完全不动。实测读取私钥返回 404，写入失败。其他机密目录（例如 `.gnupg`）照样多加一行 `tmpfs` 即可。
- 权限弹窗（chmod / chown）**拒绝 scope 根**，所以不会因为在界面里误改 `/root` 本身的权限而让 ssh 异常。
- 上面的 compose 同时设置了 `FB_NOAUTH=true`：能连到这个端口的人不用登录就能读写被服务的整个目录。只走 Tailscale 时，风险取决于 tailnet 里有谁和 ACL 怎么设；不要用 Tailscale Funnel 把它暴露到公网。

### 数据与代码改动清单

- **近期新增（后端）**：`activity/` + `storage/bolt/activity.go`、`http/{activity,trash,diskusage,archive,sync_extras}.go`，修改 `http/{resource,settings,auth,tus_handlers,http,static}.go` 和 `settings/settings.go`（回收站设置、`ActivityDays`）。
- **近期新增（前端）**：`views/{Trash,DiskUsage,DualPane}.vue`、`views/settings/Activity.vue`、`components/prompts/{Compress,Extract,BatchRename,SyncIgnores}.vue`、`api/{trash,du,archive,activity}.ts`、`stores/sync.ts`、`utils/{batchRename,dualPane,archives,syncState}.ts`，`utils/drop.ts` 抽出了通用的 `transferItems`。

- 后端新增：`syncthing/`（客户端）、`http/sync.go`、`http/permissions.go`（+ `permissions_unix.go` / `permissions_windows.go`）、`http/format.go`、`http/static_test.go`、`http/users_favorites_test.go`、`http/sync_test.go`、`http/permissions_test.go`、`http/format_test.go`、`users/favorites_test.go`；修改 `users/users.go`（`Favorites`、`FolderColors` 及校验）、`http/users.go`（校验失败返回 400）、`http/http.go`（注册 `/api/sync`）、`http/static.go`（向前端传 `EnableSync`）、`settings/settings.go` 和 `cmd/root.go`（运行时的 Syncthing 参数）。
- 前端新增：`api/sync.ts`、`types/sync.d.ts`、`utils/{drop,favorites,folderColors,syncRate}.ts`、`api/{permissions,formatGo}.ts`、`utils/editor/{formatters,aceAssets,aceFiles}.ts`、`components/prompts/{FolderColor,SyncFolder,Permissions}.vue`、`components/settings/SyncDeviceForm.vue`、`views/settings/Sync.vue`；修改 `stores/auth.ts`、`Sidebar.vue`、`ListingItem.vue`、`FileListing.vue` 等。
- 部署与发布新增：`Dockerfile.multiarch`、`docker/multiarch/entrypoint.sh`、`docker-compose.yml`、`.env.example`、`compose.multiarch.yaml`、`VERSION`、`CHANGES.md`、`scripts/{release,release-all,changes,init-env}.sh`。

### 开发与测试

```bash
go build ./... && go vet ./... && go test ./...          # 后端
cd frontend && pnpm install --frozen-lockfile
pnpm run typecheck && pnpm run lint && pnpm test          # 前端（vitest）
pnpm run build                                            # 构建前端到 frontend/dist，再 go build 即可得到带界面的二进制
```

- **后端测试**：`users/favorites_test.go`（收藏夹和颜色的规范化与校验）、`http/users_favorites_test.go`（普通用户可以改、非法输入返回 400）、`http/sync_test.go`（用内存里的假 Syncthing 服务器，覆盖权限、路径校验、符号链接、重叠、设备管理、接受共享、状态和速度字段、错误映射）、`http/permissions_test.go`（模式、递归、拒绝 scope 根/符号链接/setuid、规则隐藏、改属主）、`http/format_test.go`、`http/static_test.go`。对符号链接防护做过变异测试：去掉防护后对应用例会失败。
- **前端测试**：`utils/__tests__/favorites.test.ts`、`stores/__tests__/auth-favorites.test.ts`（含“令牌刷新后收藏夹仍在”的回归测试）、`utils/__tests__/sync-rate.test.ts`、`utils/__tests__/formatters.test.ts`（真实运行 Prettier、Ruff、sql-formatter，Go 的服务器调用用 mock）。
- **近期功能的测试**：`http/trash_test.go`（移入和还原、配额淘汰、过期、过大直接删除、设置往返）、`http/activity_test.go`、`http/diskusage_test.go`（含竞态检测）、`http/archive_test.go`（6 种格式往返、恶意条目、限制）、`http/sync_extras_test.go`；前端 `utils/__tests__/{batch-rename,dual-pane,archives}.test.ts`。
- **端到端**：用 Docker 起两套真实的 Syncthing + File Browser，再用 Playwright 驱动浏览器，把配对、共享、接受、同步、暂停恢复、拖放到收藏夹、收藏夹和颜色在子目录跳转和刷新后是否保留、传输速度显示，以及编辑器（字体、各类型格式化、保存时格式化、资源是否全部来自应用自身）和权限弹窗（右键入口、改权限、改属主、递归）都走过一遍。近期的功能（回收站和设置、活动页、磁盘占用、压缩解压、批量重命名、复制一份、双栏拖放，以及同步标记、冲突、忽略规则、限速）也都在真实浏览器里走过一遍，发现并修了其中的两个问题。这部分是手工脚本，没有放进仓库。
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
