# Komari Agent 卸载指南

在节点列表或节点详情页打开删除对话框，选择目标平台后复制卸载命令，并在被控机器上执行。面板的“删除节点”操作只移除面板中的节点记录，不会远程执行卸载命令。

默认服务名或容器名为 `komari-agent`。如果安装时使用了 `--install-service-name`，请在对话框中填写相同名称；如果使用了 `--install-dir`，也请填写原安装目录。自定义名称和目录不匹配时，系统服务可能仍在运行，或者安装文件无法被清理。

卸载命令会移除 Agent 可执行文件和自动发现身份文件，并只在安装目录为空时删除该目录。其他文件会保留，避免误删同目录中的用户数据。Docker 卸载会删除指定容器和其绑定挂载的自动发现文件，保留镜像和命名卷。

## 手动卸载

### Linux（systemd）

系统级默认安装使用 `/opt/komari`：

```bash
service_name=komari-agent
sudo systemctl stop "$service_name.service" 2>/dev/null || true
sudo systemctl disable "$service_name.service" 2>/dev/null || true
sudo rm -f "/etc/systemd/system/$service_name.service"
sudo systemctl daemon-reload
sudo rm -f /opt/komari/agent /opt/komari/auto-discovery.json
sudo rmdir /opt/komari 2>/dev/null || true
```

普通用户安装使用 `systemctl --user`，默认文件在 `$XDG_DATA_HOME/komari`（未设置时为 `$HOME/.local/share/komari`），服务文件在 `$HOME/.config/systemd/user`。请以安装 Agent 的用户身份停止服务并清理相应文件。OpenRC、OpenWrt procd、Upstart、自定义目录和 macOS/Docker 安装建议使用管理界面生成的命令。

### Windows

以管理员身份打开 PowerShell。默认服务名是 `komari-agent`，默认安装目录是 `$Env:ProgramFiles\Komari`：

```powershell
nssm stop komari-agent
nssm remove komari-agent confirm
Remove-Item -LiteralPath "$Env:ProgramFiles\Komari\komari-agent.exe", "$Env:ProgramFiles\Komari\nssm.exe", "$Env:ProgramFiles\Komari\auto-discovery.json" -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath "$Env:ProgramFiles\Komari" -Force -ErrorAction SilentlyContinue
```

如果安装时指定了自定义服务名或目录，请替换为对应值。最后一条命令只会删除空目录。

### Docker

```bash
docker rm -f komari-agent
```

此命令保留镜像和卷。若容器由 Docker Compose 管理，请使用对应项目目录中的 `docker compose down`，避免 Compose 再次创建容器。
