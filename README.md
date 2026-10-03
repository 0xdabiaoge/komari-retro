# Komari Retro

<div align="center">

**轻量、高效、自托管的独立服务器监控运维套件**

[![GitHub Release](https://img.shields.io/github/v/release/0xdabiaoge/komari-retro?color=blue&style=flat-square)](https://github.com/0xdabiaoge/komari-retro/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?style=flat-square&logo=go)](https://golang.org)
[![React Version](https://img.shields.io/badge/React-19.x-61DAFB.svg?style=flat-square&logo=react)](https://react.dev)

[简体中文](./README.md) | [Documentation](./komari-document) | [GitHub 仓库](https://github.com/0xdabiaoge/komari-retro)

</div>

---

## 📖 项目简介 (Introduction)

**Komari Retro** 是由 [@0xdabiaoge](https://github.com/0xdabiaoge) 维护的 Komari 独立定制分支。

本项目脱离原项目的频繁滚动更新，旨在打造一个**结构完整、自成一体、长期稳定、自主可控**的现代化服务器监控与管理面板，采用 Monorepo 架构将主控服务端、监控探针、管理前端以及完整开发文档聚合为一体，开箱即用。

---

## 📌 版本基准 (Upstream Base Versions)

本项目各核心组件所基于的原版代码版本如下：

| 组件名称 | 目录位置 | 基于上游版本 | 技术栈与特性 |
| :--- | :--- | :--- | :--- |
| **Komari Server (服务端)** | 根目录 `/` | **v1.2.5** | Go 1.24 / Gin / GORM / SQLite & MySQL / JSON-RPC 2.0 / WebSocket v2 / 静态资源内嵌 |
| **Komari Agent (客户端探针)** | `/komari-agent` | **最新稳定版 (v1.1.x+)** | Go 1.24 / gopsutil / 支持多网卡、GPU 监控、自定义 DNS、Web SSH 远程终端、自动重连 |
| **Komari Web (前端仪表盘)** | `/komari-web` | **最新版 (React 19)** | React 19 / TypeScript / Vite 6 / Radix UI / Monaco Editor / PWA 支持 / 多语言适配 |
| **Komari Document (官方文档)** | `/komari-document` | **最新版** | VitePress 1.6+ / 完整中英双语部署与开发接口文档 |

---

## 🗂️ 项目结构 (Repository Structure)

本项目采用 Monorepo 单体仓库管理：

```text
komari-retro/
├── cmd/                        # 服务端命令行指令 (server, chpasswd, disable-2fa 等)
├── database/                   # 数据库持久化层与模型定义 (SQLite / MySQL)
├── pkg/                        # 核心通用依赖与 RPC 路由中间件
├── protocol/                   # 探针通信协议定义
├── utils/                      # 工具函数库与 GeoIP 解析
├── web/                        # Web 路由、API 接口及静态资源嵌入逻辑
│   └── public/defaultTheme/    # 预编译的前端静态资源 (支持无 Node 环境一键构建 Go 二进制)
├── komari-agent/               # 独立探针客户端源码 (支持 Linux / Windows / macOS / Docker)
├── komari-web/                 # 现代化 React 19 仪表盘前端源码
├── komari-document/            # VitePress 架构的完整使用与二次开发文档
├── install-komari.sh           # 服务端一键自动化部署脚本
├── Dockerfile                  # 容器化打包文件
└── main.go                     # 主程序入口
```

---

## ✨ 核心特性 (Features)

- 🚀 **单二进制极简部署**：主控端与前端静态资源编译为单一独立二进制文件，无需依赖外部 Nginx 或 Node.js 运行环境。
- 📊 **毫秒级实时监控**：基于 WebSocket v2 全双工通信，实时采集与推送 CPU、内存、Swap、磁盘、网络实时带宽及流量。
- 🖥️ **跨平台探针**：探针支持 Linux (主流架构 x86/ARM/MIPS/RISC-V)、Windows、macOS 以及 Docker 容器部署。
- 💻 **内置 Web SSH 终端**：支持安全受控的远程终端控制与命令执行（支持二次身份验证 2FA 保护）。
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
4. 初始管理员账号为 `admin`，初始密码可在启动日志中获取，或使用命令行直接重置：
   ```bash
   ./komari chpasswd -p <你的新密码>
   ```

---

### 3. Docker 与 Docker Compose 部署

#### 方式 A：Docker 命令直接运行
```bash
# 创建数据存储目录
mkdir -p ./data

# 启动容器
docker run -d \
  -p 25774:25774 \
  -v $(pwd)/data:/app/data \
  --name komari \
  --restart unless-stopped \
  ghcr.io/0xdabiaoge/komari-retro:latest
```

#### 方式 B：Docker Compose 启动与更新
```bash
# 启动服务
docker compose up -d

# 后续拉取手动构建的最新镜像并无缝更新
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

## 🛠️ 本地从源码构建 (Build from Source)

环境要求：
- **Go**: 1.24 及以上
- **Node.js**: 20 及以上 (仅前端需要重新打包时)
- **pnpm**: 9+ 或 12+

### 方式 A：直接编译服务端（免 Node 环境）
由于仓库内已预置编译好的最新前端资源（位于 `web/public/defaultTheme`），您可以直接编译 Go 二进制：
```bash
go build -o komari .
```

### 方式 B：全量编译（前端 + 服务端）
```bash
# 1. 构建前端静态资源
cd komari-web
pnpm install
pnpm run build
cd ..

# 2. 同步静态文件到服务端内嵌目录
mkdir -p web/public/defaultTheme/dist
cp -r komari-web/dist/* web/public/defaultTheme/dist/
cp komari-web/komari-theme.json web/public/defaultTheme/

# 3. 编译服务端独立可执行文件
go build -ldflags "-s -w" -o komari .

# 4. 编译客户端探针
cd komari-agent
go build -ldflags "-s -w" -o komari-agent .
```

---

## 🚀 自动化构建与镜像发布 (CI / CD)

本项目将 GitHub Actions 工作流精简为两套核心流程（支持全中文交互）：

1. **发布版本与跨平台产物到 Releases (`release.yml`)**
   - **触发时机**：当涉及改动并完成版本号提升打 Tag（如 `v1.2.6`）推送至 GitHub、或在 GitHub 发布 Release、或手动调度触发。
   - **构建内容**：一次性构建前端静态资产，并利用 Zig 交叉编译 7 个系统平台架构（Linux amd64/arm64/386/riscv64、Windows amd64/arm64/386）的服务端与客户端探针，自动打包并挂载至 GitHub Releases。

2. **手动构建并推送 Docker 镜像 (`docker-publish.yml`)**
   - **触发时机**：纯手动按需触发（在仓库 **Actions** 页面选择该工作流并点击 **Run workflow**）。
   - **参数输入**：支持自定义镜像标签（默认 `latest`，可指定如 `v1.2.5`）以及是否同时关联 `latest` 标签。
   - **构建内容**：自动完成前端及 Linux 多架构静态二进制编译，打包多架构镜像并推送至 `ghcr.io/0xdabiaoge/komari-retro`。
   - **更新应用**：使用 Docker / Docker Compose 部署的机器执行 `docker compose pull && docker compose up -d` 即可无缝拉取最新镜像。

---

## ⚠️ 安全须知 (Security Notice)

Komari 是一款仅供您本人拥有或获得合法运维授权的系统上使用的监控与运维管理软件。请勿将其用于未经授权的未受控环境、恶意持久化或滥用行为。使用者应对自身的部署与操作承担全部责任。

---

## 📄 开源许可与致谢 (License & Acknowledgements)

- 本项目基于 [MIT License](./LICENSE) 开源。
- 感谢原项目 [komari-monitor/komari](https://github.com/komari-monitor/komari) 及原作者 **Akizon77** 优秀的原始设计与开源贡献。
