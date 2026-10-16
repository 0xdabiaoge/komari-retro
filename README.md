# Komari Retro

<div align="center">

**轻量、高效、自托管的独立服务器监控运维套件**

[![GitHub Release](https://img.shields.io/github/v/release/0xdabiaoge/komari-retro?color=blue&style=flat-square)](https://github.com/0xdabiaoge/komari-retro/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.27.2-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
[![React Version](https://img.shields.io/badge/React-19.x-61DAFB.svg?style=flat-square&logo=react)](https://react.dev)

[简体中文](./README.md) | [GitHub 仓库](https://github.com/0xdabiaoge/komari-retro)

</div>

---

## 📖 项目简介 (Introduction)

**Komari Retro** 是由 [@0xdabiaoge](https://github.com/0xdabiaoge) 维护的 Komari 独立定制分支。

本项目脱离原项目的频繁滚动更新，旨在打造一个**结构完整、自成一体、长期稳定、自主可控**的现代化服务器监控与管理面板，采用 Monorepo 架构将主控服务端、监控探针、管理前端聚合为一体，开箱即用。

---

## 📌 版本基准 (Upstream Base Versions)

本项目各核心组件所基于的原版代码版本如下：

| 组件名称 | 目录位置 | 基于上游版本 | 技术栈与特性 |
| :--- | :--- | :--- | :--- |
| **Komari Server (服务端)** | 根目录 `/` | **v1.2.5** | Go 1.25+ / Gin / GORM / SQLite / JSON-RPC 2.0 / WebSocket v2 / 静态资源内嵌 |
| **Komari Agent (客户端探针)** | `/komari-agent` | **最新稳定版 (v1.1.x+)** | Go 1.25+ / gopsutil / 支持多网卡、GPU 监控、自定义 DNS、Web SSH 远程终端、自动重连 |
| **Komari Web (前端仪表盘)** | `/komari-web` | **最新版 (React 19)** | React 19 / TypeScript / Vite 6 / Radix UI / Monaco Editor / PWA 支持 / 多语言适配 |

---

## 🗂️ 项目结构 (Repository Structure)

本项目采用 Monorepo 单体仓库管理：

```text
komari-retro/
├── cmd/                        # 服务端命令行指令 (server, chpasswd 等)
├── database/                   # 数据库持久化层与模型定义 (SQLite)
├── pkg/                        # 核心通用依赖与 RPC 路由中间件
├── protocol/                   # 探针通信协议定义
├── utils/                      # 工具函数库与 GeoIP 解析
├── web/                        # Web 路由、API 接口及静态资源嵌入逻辑
│   └── public/defaultTheme/    # 预编译的前端静态资源 (支持无 Node 环境一键构建 Go 二进制)
├── komari-agent/               # 独立探针客户端源码 (支持 Linux / Windows / macOS / Docker)
├── komari-web/                 # 现代化 React 19 仪表盘前端源码
├── install-komari.sh           # 服务端一键自动化部署脚本
├── Dockerfile                  # 容器化打包文件
└── main.go                     # 主程序入口
```

---

## ✨ 核心特性 (Features)

- 🚀 **单二进制极简部署**：主控端与前端静态资源编译为单一独立二进制文件，无需依赖外部 Nginx 或 Node.js 运行环境。
- 📊 **毫秒级实时监控**：基于 WebSocket v2 全双工通信，实时采集与推送 CPU、内存、Swap、磁盘、网络实时带宽及流量。
- 🖥️ **跨平台探针**：探针支持 Linux (主流架构 x86/ARM/MIPS/RISC-V)、Windows、macOS 以及 Docker 容器部署。
- 💻 **内置 Web SSH 终端**：支持管理员权限控制的远程终端与文件管理。
- 🎨 **主题化与可视化定制**：支持深色/浅色模式、自定义主题切换、布局卡片自由拖拽定制。
- 🔔 **通知与告警**：支持离线监控、阈值警报以及多样化消息推送方式。

---

## 🚀 快速开始 (Quick Start)

### 1. 服务端一键脚本安装 (推荐)

适用于支持 `systemd` 的 Linux 发行版（Debian / Ubuntu / CentOS 等）：

```bash
curl -fsSL https://raw.githubusercontent.com/0xdabiaoge/komari-retro/main/install-komari.sh -o install-komari.sh
chmod +x install-komari.sh
sudo ./install-komari.sh
```

脚本将引导您完成端口设置与服务初始化，默认监听端口为 `25774`。

---

### 2. 二进制直接运行

1. 前往本项目的 [GitHub Releases](https://github.com/0xdabiaoge/komari-retro/releases) 下载对应系统架构的可执行文件。
2. 赋予执行权限并启动：
   ```bash
   chmod +x komari
   ./komari server -l 0.0.0.0:25774
   ```
3. 浏览器访问：`http://<服务器IP>:25774`。
4. 初始管理员账号为 `admin`，初始密码保存在 `data/initial-admin.txt`（权限 0600，改密后删除），或使用命令行直接重置：
   ```bash
   ./komari chpasswd -p <你的新密码>
   ```

---

### 3. Docker 与 Docker Compose 部署

#### 方式 A：Docker 命令直接运行
```bash
# 创建数据存储目录
mkdir -p ./data/docker

# 启动容器
docker run -d \
  -p 25774:25774 \
  -v $(pwd)/data/docker:/app/data \
  --name komari \
  --restart unless-stopped \
  ghcr.io/0xdabiaoge/komari-retro:latest
```

#### 方式 B：Docker Compose 启动与更新
```bash
# 启动服务
docker compose up -d

# 后续拉取镜像并重新创建容器（会短暂中断连接）
docker compose pull && docker compose up -d
```

---

### 4. 客户端探针安装 (Agent)

在被监控的服务器上执行一键安装：

**Linux 一键安装：**
```bash
curl -fsSL https://raw.githubusercontent.com/0xdabiaoge/komari-retro/main/komari-agent/install.sh | sudo sh -s -- -e "http://<你的面板地址>:25774" -t "<节点Token>"
```

**Windows PowerShell 安装：**
```powershell
Invoke-Expression (Invoke-RestMethod "https://raw.githubusercontent.com/0xdabiaoge/komari-retro/main/komari-agent/install.ps1") -Endpoint "http://<你的面板地址>:25774" -Token "<节点Token>"
```

---

<a id="agent-uninstall"></a>
### Agent 卸载

在管理后台的服务器列表中，打开目标节点的删除窗口，选择实际安装方式并填写自定义服务名、容器名或安装目录，复制窗口生成的卸载命令，在对应节点执行。仅删除面板中的节点记录不会卸载节点上的 Agent。

如果设备并非由你授权接入，应先停止 Agent 服务，再检查该设备的访问记录。

## 🛠️ 本地从源码构建 (Build from Source)

环境要求：
- **Go**: 模块最低 1.26，发布与安全验证固定使用 1.27.2
- **Node.js**: 22 LTS (仅前端需要重新打包时)
- **npm**: 使用提交的 `package-lock.json` 与 `npm ci`

### 方式 A：直接编译服务端（免 Node 环境）
由于仓库内已预置编译好的最新前端资源（两套主题各有管理员与游客产物，位于 `web/public`），您可以直接编译 Go 二进制：
```bash
go build -o komari .
```

### 方式 B：全量编译（前端 + 服务端）
```bash
# 1. 分别构建两套主题的管理员与游客产物
cd komari-web
npm ci
npm run build
npm run build:guest
cd ../komari-next
npm ci
npm run build
npm run build:guest
cd ..

# 2. 清理旧文件并同步四套产物到服务端内嵌目录
node script/embed-frontends.mjs

# 3. 编译服务端独立可执行文件
go build -ldflags "-s -w" -o komari .

# 4. 编译客户端探针
cd komari-agent
go build -ldflags "-s -w" -o komari-agent .
```

---

## 🚀 自动化构建与镜像发布 (CI / CD)

GitHub Actions 包含质量门禁和统一的发布流程：

0. **质量门禁 (`quality.yml`)**：服务端与探针的 test/vet/race/govulncheck，以及前端 npm ci、audit、测试、lint、构建和浏览器回归；发布必须先通过。

1. **自动递增版本并发布 (`build-release.yml`)**
   - **触发时机**：在 GitHub Actions 中手动运行；必须选择 `main` 分支。质量门禁通过后，工作流读取仓库中最高的稳定版标签并自动生成下一版本，无需手填版本号。
   - **递增规则**：版本格式为 `v主版本.次版本.修订号`，各段范围为 1–99；修订号到 99 后从 1 开始并向前进位，例如 `v1.2.99` 后为 `v1.3.1`。
   - **构建内容**：构建服务端和 Agent 的同版本跨平台产物，并将该版本写入 Release、Docker 版本标签及镜像元数据；同时发布 `linux/amd64` 和 `linux/arm64` 镜像。可选择是否更新 Docker `latest` 标签。
   - **质量检查不占版本号**：普通 push/PR 的质量门禁不会发布产品，也不会递增版本。未通过发布工作流构建的源码版本显示为 `dev`；只有正式发布产物才嵌入自动生成的版本号。
   - **更新应用**：执行 `docker compose pull && docker compose up -d`；容器重建会短暂中断连接。

脚本升级需要 `flock`（util-linux），先下载、验证 SHA256 和可执行性，再停服务保存二进制及数据快照。新版 `/ping` 健康检查失败时恢复原二进制和数据。新脚本要求 Release 提供 `.sha256` 资产，缺少摘要的历史版本会拒绝安装。

脚本与 Docker 同时运行时，必须使用独立数据目录。Compose 默认挂载 `./data/docker`；不要将两套实例都指向同一 SQLite。程序会阻止同一数据目录或同一数据库被多个新版进程使用。迁移旧部署时需要先确认各自的数据来源，不能直接更换挂载目录后当作数据已迁移。

默认只信任直接连接的来源 IP。反向代理请通过 `KOMARI_TRUSTED_PROXIES` 指定实际代理的 IP/CIDR，转发的协议和 IP 仅对可信代理生效。API 密钥支持 `read-only`（公开/监控查询 RPC）和 `full`（管理操作）；现有密钥默认保留完整权限，管理员可在登录设置中改为只读。

---

## 仪表盘性能与数据库空间

Agent 长连接默认最多 2048 条 TCP 连接；可通过 `KOMARI_MAX_CONNECTIONS` 在 256–8192 范围调整，非法值回退为 2048。上报 HTTP 使用独立的 32 活跃请求、64 等待请求短队列，后台控制保留原有 16 请求容量。所有入口仍需身份认证，超量请求返回 503 并要求重试。

仪表盘使用独立的只读 WAL 连接汇总最近 24 小时的数据。升级后首次启动会创建延迟统计覆盖索引；历史记录较多时需要等待索引创建完成，并预留临时磁盘空间。流量图固定 120 个时间桶，资源和延迟排行榜各显示前 100 项；具体节点/任务曲线按需加载。资源和流量缓存 30 秒，延迟排行缓存 2 分钟。延迟计数、丢包率及 P50/P95/P99 从精确频数计算，完全丢包的目标不参与稳定性排名。首次读取大量历史仍需要扫描数据，不能把缓存命中耗时当成冷查询耗时。

延迟记录默认保留 24 小时，每分钟小批量清理；负载原始记录超过约 4 小时后聚合，长期负载默认保留 30 天（`KOMARI_RECORD_RETENTION_DAYS`）。先成功聚合再清理，保留原始层的必要重叠窗口。节点数、每节点目标数、探测频率及保留时间共同决定稳定占用；500 节点 × 36 个目标 × 每分钟一次，每天会产生 2592 万条延迟记录。保留期内的数据不会被静默抽样或删除。

“系统 → 站点”的空间卡片显示数据库分配空间、可复用空闲页和 WAL 大小。过期记录被删除后，空闲页可以复用，因此文件大小不一定立即下降。新数据库自动开启小批量增量回收；已有数据库不会在启动或在线请求中自动执行全库整理。

如需立即缩小已有 SQLite 文件，先备份并校验，再安排维护窗口停止 Komari，使用**同一版本**二进制执行：

```bash
# 脚本部署：确认对应服务已停止，替换为实际数据库路径
komari --database /absolute/path/data/komari.db compact-database

# Compose 部署：在实际 Compose 目录中执行，服务名按部署配置调整
# 先备份；以下 stop/up 会短暂中断 Komari，不应在业务高峰执行。
docker compose stop komari
docker compose run --rm --no-deps komari compact-database
docker compose up -d komari
```

命令会拒绝被 Komari 进程锁定的数据库，整理前后执行完整性检查，并为后续增量回收转换格式。为整理预留至少两倍数据库文件大小的可用磁盘空间。若整理失败，先检查原因和备份，再启动原服务；不能通过删除 WAL/SHM/锁文件强行整理。SQLite 的空闲页复用及离线整理行为见 [官方说明](https://www.sqlite.org/lang_vacuum.html)。

隔离回归（不会访问生产）：`python3 script/dashboard-acceptance.py /path/to/komari`。大规模完整日数据测试：`go test ./web/rpc/jsonrpc -run '^$' -bench DashboardScale -benchtime=1x -timeout=20m`，需数 GB 临时空间；基准程序会生成并清理 500 节点、36 目标、2592 万条记录的独立数据库。

---

## ⚠️ 安全须知 (Security Notice)

Komari 是一款仅供您本人拥有或获得合法运维授权的系统上使用的监控与运维管理软件。请勿将其用于未经授权的未受控环境、恶意持久化或滥用行为。使用者应对自身的部署与操作承担全部责任。

---

## 📄 开源许可与致谢 (License & Acknowledgements)

- 本项目基于 [MIT License](./LICENSE) 开源。
- 感谢原项目 [komari-monitor/komari](https://github.com/komari-monitor/komari) 及原作者 **Akizon77** 优秀的原始设计与开源贡献。
