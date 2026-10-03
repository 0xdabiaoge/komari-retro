#!/bin/bash

# Color definitions for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "$1"
}

log_success() {
    echo -e "${GREEN}$1${NC}"
}

log_error() {
    echo -e "${RED}$1${NC}"
}

log_step() {
    echo -e "${YELLOW}$1${NC}"
}

# Global variables
INSTALL_DIR="/opt/komari"
DATA_DIR="/opt/komari"
SERVICE_NAME="komari"
BINARY_PATH="$INSTALL_DIR/komari"
DEFAULT_PORT="25774"
LISTEN_PORT=""

# Show banner
show_banner() {
    clear
    echo "=============================================================="
    echo "            Komari Monitoring System Installer"
    echo "       https://github.com/0xdabiaoge/komari-retro"
    echo "=============================================================="
    echo
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

# Check for systemd
check_systemd() {
    if ! command -v systemctl >/dev/null 2>&1; then
        return 1
    else
        return 0
    fi
}

# Detect system architecture
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64)
            echo "arm64"
            ;;
        i386|i686)
            echo "386"
            ;;
        riscv64)
            echo "riscv64"
            ;;
        *)
            log_error "不支持的架构: $arch"
            exit 1
            ;;
    esac
}

# Check if Komari is already installed
is_installed() {
    if [ -f "$BINARY_PATH" ]; then
        return 0
    else
        return 1
    fi
}

# Install dependencies
install_dependencies() {
    log_step "检查并安装依赖..."

    if ! command -v curl >/dev/null 2>&1; then
        if command -v apt >/dev/null 2>&1; then
            log_info "使用 apt 安装依赖..."
            apt update
            apt install -y curl
        elif command -v yum >/dev/null 2>&1; then
            log_info "使用 yum 安装依赖..."
            yum install -y curl
        elif command -v apk >/dev/null 2>&1; then
            log_info "使用 apk 安装依赖..."
            apk add curl
        else
            log_error "未找到支持的包管理器 (apt/yum/apk)"
            exit 1
        fi
    fi
}

# Nginx reverse proxy configuration
setup_nginx_proxy() {
    local target_port="${1:-$LISTEN_PORT}"
    if [ -z "$target_port" ]; then
        target_port="$DEFAULT_PORT"
    fi

    log_step "=== 配置 Nginx 反向代理 ==="

    if ! command -v nginx >/dev/null 2>&1; then
        read -p "未检测到 Nginx，是否现在安装？[Y/n]: " inst_ng
        if [[ ! "$inst_ng" =~ ^[Nn]$ ]]; then
            log_step "安装 Nginx 及 OpenSSL..."
            if command -v apt >/dev/null 2>&1; then
                apt update && apt install -y nginx openssl
            elif command -v yum >/dev/null 2>&1; then
                yum install -y nginx openssl
            elif command -v apk >/dev/null 2>&1; then
                apk add nginx openssl
            else
                log_error "未识别支持的包管理器，请手动安装 Nginx。"
                return 1
            fi
        else
            log_error "已取消 Nginx 安装。"
            return 1
        fi
    fi

    local default_domain="kmro.1687.de5.net"
    read -p "请输入要绑定的域名 [默认: $default_domain]: " input_domain
    local domain="${input_domain:-$default_domain}"

    read -p "请输入后端目标端口 [默认: $target_port]: " input_proxy_port
    local proxy_port="${input_proxy_port:-$target_port}"

    # 创建证书目录并生成自签名证书 (供 Cloudflare Full SSL / HTTPS 使用)
    mkdir -p /etc/nginx/ssl
    if [ ! -f /etc/nginx/ssl/komari.crt ]; then
        log_info "生成自签名证书 (供 Cloudflare Full SSL / HTTPS 使用)..."
        openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
            -keyout /etc/nginx/ssl/komari.key \
            -out /etc/nginx/ssl/komari.crt \
            -subj "/CN=${domain}" >/dev/null 2>&1
    fi

    # 确定配置保存目录
    local conf_dir="/etc/nginx/conf.d"
    mkdir -p "$conf_dir"
    local conf_file="${conf_dir}/komari_${domain}.conf"

    cat > "$conf_file" << EOF
server {
    listen 80;
    listen 443 ssl http2;
    server_name ${domain};

    ssl_certificate /etc/nginx/ssl/komari.crt;
    ssl_certificate_key /etc/nginx/ssl/komari.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 获取 Cloudflare 真实访问者 IP
    set_real_ip_from 0.0.0.0/0;
    real_ip_header CF-Connecting-IP;

    client_max_body_size 50M;

    location / {
        proxy_pass http://127.0.0.1:${proxy_port};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
EOF

    # 检查并重载 nginx
    if nginx -t >/dev/null 2>&1; then
        systemctl enable nginx >/dev/null 2>&1
        systemctl restart nginx
        log_success "Nginx 反向代理配置成功！"
        log_info "  访问地址: http://${domain} 或 https://${domain}"
        log_info "  反代目标: http://127.0.0.1:${proxy_port}"
        log_info "  配置文件: ${conf_file}"
    else
        log_error "Nginx 配置语法测试失败，请检查: nginx -t"
        return 1
    fi
}

# Cloudflare Tunnel setup
setup_cf_tunnel() {
    local target_port="${1:-$LISTEN_PORT}"
    if [ -z "$target_port" ]; then
        target_port="$DEFAULT_PORT"
    fi

    log_step "=== 配置 Cloudflare Tunnel 隧道 ==="

    if ! command -v cloudflared >/dev/null 2>&1; then
        log_step "正在下载并安装 cloudflared..."
        local arch=$(detect_arch)
        local cf_arch="amd64"
        case "$arch" in
            amd64) cf_arch="amd64" ;;
            arm64) cf_arch="arm64" ;;
            386) cf_arch="386" ;;
            *) cf_arch="amd64" ;;
        esac
        local cf_url="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${cf_arch}"
        if ! curl -fsSL -o /usr/local/bin/cloudflared "$cf_url"; then
            log_error "下载 cloudflared 失败，请检查网络"
            return 1
        fi
        chmod +x /usr/local/bin/cloudflared
        log_success "cloudflared 安装完成: /usr/local/bin/cloudflared"
    fi

    read -p "请输入 Cloudflare Tunnel Token (在 Cloudflare Zero Trust 获取): " token

    if [ -z "$token" ]; then
        log_error "Token 不能为空"
        return 1
    fi

    log_step "正在安装并启动 cloudflared 服务..."
    cloudflared service uninstall >/dev/null 2>&1 || true

    if cloudflared service install "$token"; then
        systemctl daemon-reload
        systemctl enable --now cloudflared
        sleep 2
        if systemctl is-active --quiet cloudflared; then
            log_success "Cloudflare Tunnel 服务启动成功！"
            log_info "  状态查看: systemctl status cloudflared"
        else
            log_error "Cloudflare Tunnel 服务启动失败，请检查: journalctl -u cloudflared -f"
            return 1
        fi
    else
        log_error "cloudflared service install 执行失败"
        return 1
    fi
}

# Binary installation
install_binary() {
    log_step "开始二进制安装..."

    if is_installed; then
        log_info "Komari 已安装。要升级，请使用升级选项。"
        return
    fi

    # 监听端口输入，校验范围 1-65535
    while true; do
        read -p "请输入监听端口 [默认: $DEFAULT_PORT]: " input_port
        if [[ -z "$input_port" ]]; then
            LISTEN_PORT="$DEFAULT_PORT"
            break
        elif [[ "$input_port" =~ ^[0-9]+$ ]] && (( input_port >= 1 && input_port <= 65535 )); then
            LISTEN_PORT="$input_port"
            break
        else
            log_error "端口号无效，请输入 1-65535 之间的数字。"
        fi
    done

    install_dependencies

    local arch=$(detect_arch)
    log_info "检测到架构: $arch"

    log_step "创建安装目录: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"

    log_step "创建数据目录: $DATA_DIR"
    mkdir -p "$DATA_DIR"

    local file_name="komari-linux-${arch}"
    local download_url="https://github.com/0xdabiaoge/komari-retro/releases/latest/download/${file_name}"

    log_step "获取 Komari 二进制文件..."
    log_info "下载 URL: $download_url"

    if ! curl -f -L -o "$BINARY_PATH" "$download_url"; then
        log_step "从 GitHub Releases 下载失败，检查本地可用文件..."
        if [ -f "./komari" ]; then
            log_info "检测到当前目录存在 ./komari，使用本地文件..."
            cp "./komari" "$BINARY_PATH"
        elif [ -f "/root/komari-workspace/komari" ]; then
            log_info "检测到 /root/komari-workspace/komari，使用本地文件..."
            cp "/root/komari-workspace/komari" "$BINARY_PATH"
        else
            log_error "下载失败且未发现本地二进制文件。"
            log_info "提示: GitHub Releases 产物生成后可直接下载，或将编译好的 komari 放在当前目录重试。"
            return 1
        fi
    fi

    chmod +x "$BINARY_PATH"
    log_success "Komari 二进制文件准备就绪: $BINARY_PATH"

    if ! check_systemd; then
        log_step "警告：未检测到 systemd，跳过服务创建。"
        log_step "您可以从命令行手动运行 Komari："
        log_step "    $BINARY_PATH server -l 0.0.0.0:$LISTEN_PORT"
        echo
        log_success "安装完成！"
        return
    fi

    create_systemd_service "$LISTEN_PORT"

    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}.service
    systemctl start ${SERVICE_NAME}.service

    if systemctl is-active --quiet ${SERVICE_NAME}.service; then
        log_success "Komari 服务启动成功"
        
        log_step "正在获取初始密码..."
        sleep 5 
        local password=$(journalctl -u ${SERVICE_NAME} --since "1 minute ago" | grep "admin account created." | tail -n 1 | sed -e 's/.*admin account created.//')
        if [ -z "$password" ]; then
            log_error "未能获取初始密码，请检查日志"
        fi
        show_access_info "$password" "$LISTEN_PORT"

        echo
        read -p "是否需要配置 Nginx 反向代理？[y/N]: " setup_ng
        if [[ "$setup_ng" =~ ^[Yy]$ ]]; then
            setup_nginx_proxy "$LISTEN_PORT"
        fi

        echo
        read -p "是否需要配置 Cloudflare Tunnel 隧道？[y/N]: " setup_cf
        if [[ "$setup_cf" =~ ^[Yy]$ ]]; then
            setup_cf_tunnel "$LISTEN_PORT"
        fi
    else
        log_error "Komari 服务启动失败"
        log_info "查看日志: journalctl -u ${SERVICE_NAME} -f"
        return 1
    fi
}

# Create systemd service file
create_systemd_service() {
    local port="$1"
    log_step "创建 systemd 服务..."

    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    cat > "$service_file" << EOF
[Unit]
Description=Komari Monitor Service
After=network.target

[Service]
Type=simple
ExecStart=${BINARY_PATH} server -l 0.0.0.0:${port}
WorkingDirectory=${DATA_DIR}
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

    log_success "systemd 服务文件创建完成"
}

# Show access information
show_access_info() {
    local password=$1
    local port=${2:-$DEFAULT_PORT}
    echo
    log_success "Komari 核心安装完成！"
    echo
    log_info "访问信息："
    log_info "  URL: http://$(hostname -I | awk '{print $1}'):${port}"
    if [ -n "$password" ]; then
        log_info "初始登录信息（仅显示一次）: $password"
    fi
    echo
    log_info "服务管理命令："
    log_info "  状态:  systemctl status $SERVICE_NAME"
    log_info "  启动:   systemctl start $SERVICE_NAME"
    log_info "  停止:    systemctl stop $SERVICE_NAME"
    log_info "  重启: systemctl restart $SERVICE_NAME"
    log_info "  日志:    journalctl -u $SERVICE_NAME -f"
}

# Upgrade function
upgrade_komari() {
    log_step "升级 Komari..."

    if ! is_installed; then
        log_error "Komari 未安装。请先安装它。"
        return 1
    fi

    if ! check_systemd; then
        log_error "未检测到 systemd。无法管理服务。"
        return 1
    fi

    log_step "停止 Komari 服务..."
    systemctl stop ${SERVICE_NAME}.service

    log_step "备份当前二进制文件..."
    cp "$BINARY_PATH" "${BINARY_PATH}.backup.$(date +%Y%m%d_%H%M%S)"

    local arch=$(detect_arch)
    local file_name="komari-linux-${arch}"
    local download_url="https://github.com/0xdabiaoge/komari-retro/releases/latest/download/${file_name}"

    log_step "下载最新版本..."
    if ! curl -f -L -o "$BINARY_PATH" "$download_url"; then
        log_error "下载失败，正在从备份恢复"
        mv "${BINARY_PATH}.backup."* "$BINARY_PATH"
        systemctl start ${SERVICE_NAME}.service
        return 1
    fi

    chmod +x "$BINARY_PATH"

    log_step "重启 Komari 服务..."
    systemctl start ${SERVICE_NAME}.service

    if systemctl is-active --quiet ${SERVICE_NAME}.service; then
        log_success "Komari 升级成功"
    else
        log_error "服务在升级后未能启动"
    fi
}

# Uninstall function
uninstall_komari() {
    log_step "卸载 Komari..."

    if ! is_installed; then
        log_info "Komari 未安装"
        return 0
    fi

    read -p "这将删除 Komari 服务与二进制。您确定吗？(Y/n): " confirm
    if [[ $confirm =~ ^[Nn]$ ]]; then
        log_info "卸载已取消"
        return 0
    fi

    if check_systemd; then
        log_step "停止并禁用服务..."
        systemctl stop ${SERVICE_NAME}.service >/dev/null 2>&1
        systemctl disable ${SERVICE_NAME}.service >/dev/null 2>&1
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
        log_success "systemd 服务已删除"
    fi

    log_step "删除二进制文件..."
    rm -f "$BINARY_PATH"
    rmdir "$INSTALL_DIR" 2>/dev/null || log_info "数据目录 $INSTALL_DIR 不为空，已保留"
    log_success "Komari 二进制文件已删除"

    log_success "Komari 卸载完成"
    log_info "数据文件保留在 $DATA_DIR"
}

# Show service status
show_status() {
    if ! is_installed; then
        log_error "Komari 未安装"
        return
    fi
    if ! check_systemd; then
        log_error "未检测到 systemd。无法获取服务状态。"
        return
    fi
    log_step "Komari 服务状态:"
    systemctl status ${SERVICE_NAME}.service --no-pager -l
}

# Show service logs
show_logs() {
    if ! is_installed; then
        log_error "Komari 未安装"
        return
    fi
    if ! check_systemd; then
        log_error "未检测到 systemd。无法获取服务日志。"
        return
    fi
    log_step "查看 Komari 服务日志..."
    journalctl -u ${SERVICE_NAME} -f --no-pager
}

# Restart service
restart_service() {
    if ! is_installed; then
        log_error "Komari 未安装"
        return
    fi
    if ! check_systemd; then
        log_error "未检测到 systemd。无法重启服务。"
        return
    fi
    log_step "重启 Komari 服务..."
    systemctl restart ${SERVICE_NAME}.service
    if systemctl is-active --quiet ${SERVICE_NAME}.service; then
        log_success "服务重启成功"
    else
        log_error "服务重启失败"
    fi
}

# Stop service
stop_service() {
    if ! is_installed; then
        log_error "Komari 未安装"
        return
    fi
    if ! check_systemd; then
        log_error "未检测到 systemd。无法停止服务。"
        return
    fi
    log_step "停止 Komari 服务..."
    systemctl stop ${SERVICE_NAME}.service
    log_success "服务已停止"
}

# Main menu
main_menu() {
    show_banner
    echo "请选择操作："
    echo "  1) 安装 Komari"
    echo "  2) 升级 Komari"
    echo "  3) 卸载 Komari"
    echo "  4) 查看服务状态"
    echo "  5) 查看服务日志"
    echo "  6) 重启 Komari 服务"
    echo "  7) 停止 Komari 服务"
    echo "  8) 配置 Nginx 反向代理"
    echo "  9) 配置 Cloudflare Tunnel 隧道"
    echo "  10) 退出"
    echo

    read -p "输入选项 [1-10]: " choice

    case $choice in
        1) install_binary ;;
        2) upgrade_komari ;;
        3) uninstall_komari ;;
        4) show_status ;;
        5) show_logs ;;
        6) restart_service ;;
        7) stop_service ;;
        8) setup_nginx_proxy ;;
        9) setup_cf_tunnel ;;
        10) exit 0 ;;
        *) log_error "无效选项" ;;
    esac
}

# Main execution
check_root
main_menu
