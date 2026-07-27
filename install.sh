#!/bin/bash

# Content Node Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/zergolf1994/content-node/main/install.sh | sudo -E bash -s -- [OPTIONS]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Defaults
UNINSTALL=false
DATABASE_URL=""
REDIS_URL=""
PORT="8082"
DOMAIN_STATIC=""
DOMAINS=()            # --domain d1 d2 ... (ว่าง = catch-all รับทุกโดเมน)
APP_HOST="localhost"  # --app-host 10.0.0.1 (โหมด nginx ชี้ app คนละเครื่อง)
INSTALL_APP=false
INSTALL_NGINX=false

APP_NAME="content-node"
APP_DIR="/opt/$APP_NAME"
SERVICE_NAME="content-node"
GITHUB_REPO="zergolf1994/content-node"
RELEASES_URL="https://github.com/$GITHUB_REPO/releases/latest/download"

print_status()  { echo -e "${GREEN}[INFO]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# ชื่อตัวแปร nginx ห้ามมี "-" — content-node → content_node
CONN_VAR="${APP_NAME//-/_}_conn"

# map เลือกค่า header Connection ตามคำขอแต่ละอัน
#
# keepalive ไปยัง upstream จะทำงานก็ต่อเมื่อ Connection เป็นค่าว่าง แต่
# WebSocket ต้องการ "upgrade" — ของเดิม hardcode 'upgrade' ไว้ตายตัว
# keepalive 32 ที่ตั้งไว้จึงไม่เคยถูกใช้เลย เปิด TCP ใหม่ทุก request
nginx_conn_map() {
    cat <<MAP
map \$http_upgrade \$${CONN_VAR} {
    default upgrade;
    ''      '';
}
MAP
}

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall)         UNINSTALL=true; shift ;;
        --app)               INSTALL_APP=true; shift ;;
        --nginx)             INSTALL_NGINX=true; shift ;;
        --database-url)      DATABASE_URL="$2"; shift 2 ;;
        --mongodb-uri)       DATABASE_URL="$2"; shift 2 ;; # alias เดิม
        --redis-url)         REDIS_URL="$2"; shift 2 ;;
        --port)              PORT="$2"; shift 2 ;;
        --app-host)          APP_HOST="$2"; shift 2 ;;
        --domain-static)     DOMAIN_STATIC="$2"; shift 2 ;;
        -d|--domain)
            # เก็บทุก arg ต่อจากนี้ที่ไม่ใช่ flag เป็นรายชื่อโดเมน
            shift
            while [[ $# -gt 0 && "$1" != -* ]]; do
                DOMAINS+=("$1"); shift
            done ;;
        -h|--help)
            echo "Content Node Installer"
            echo ""
            echo "Usage: curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh | sudo -E bash -s -- [OPTIONS]"
            echo ""
            echo "Modes (ไม่ระบุ = ทำทั้งคู่):"
            echo "  --app                ติดตั้ง app + systemd อย่างเดียว"
            echo "  --nginx              ตั้ง nginx อย่างเดียว (app ติดตั้งแล้ว/อยู่เครื่องอื่น)"
            echo ""
            echo "Options:"
            echo "  --uninstall          Uninstall completely (app + nginx vhost)"
            echo "  --database-url URI   MongoDB connection string (DATABASE_URL)"
            echo "  --mongodb-uri URI    Alias ของ --database-url"
            echo "  --redis-url URL      Redis URL (optional — ไม่ตั้ง = ไม่ใช้ cache)"
            echo "  --port PORT          HTTP port (default: 8082)"
            echo "  -d, --domain D1 D2   โดเมนเฉพาะ (ไม่ระบุ = catch-all รับทุกโดเมน)"
            echo "  --app-host HOST      ให้ nginx ชี้ app เครื่องอื่น (default: localhost)"
            echo "  --domain-static HOST Static/content domain fallback (optional)"
            echo "  -h, --help           Show this help"
            echo ""
            echo "Examples:"
            echo "  # ติดตั้งครบ (app + nginx catch-all)"
            echo "  curl -fsSL ... | sudo -E bash -s -- --database-url \"mongodb+srv://...\""
            echo ""
            echo "  # ติดตั้งพร้อมโดเมนเฉพาะ"
            echo "  curl -fsSL ... | sudo -E bash -s -- --database-url \"...\" --domain embed.example.com cdn.example.com"
            echo ""
            echo "  # App เครื่อง A / Nginx เครื่อง B"
            echo "  A: curl -fsSL ... | sudo -E bash -s -- --app --database-url \"...\""
            echo "  B: curl -fsSL ... | sudo -E bash -s -- --nginx --app-host 10.0.0.1"
            exit 0 ;;
        *)
            print_error "Unknown option: $1"; exit 1 ;;
    esac
done

# ไม่ระบุโหมด = ทำทั้ง app + nginx (เหมือน installer เดิม)
if [ "$INSTALL_APP" = false ] && [ "$INSTALL_NGINX" = false ]; then
    INSTALL_APP=true
    INSTALL_NGINX=true
fi

# ─── Uninstall ────────────────────────────────────────────────
if [ "$UNINSTALL" = true ]; then
    print_warning "⚠️  Starting Uninstallation..."
    systemctl stop "${SERVICE_NAME}"    2>/dev/null || true
    systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ] && rm "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    [ -d "$APP_DIR" ] && rm -rf "$APP_DIR"
    # nginx vhost ของ app นี้ (ถ้ามี)
    NGINX_TOUCHED=false
    if [ -f "/etc/nginx/sites-available/$APP_NAME" ]; then
        rm -f "/etc/nginx/sites-available/$APP_NAME" "/etc/nginx/sites-enabled/$APP_NAME"
        NGINX_TOUCHED=true
    fi
    # catch-all เก่าที่เคยเขียนทับ default ไว้ — ลบเฉพาะไฟล์ที่เป็นของเรา
    if [ -f /etc/nginx/sites-available/default ] &&
        head -1 /etc/nginx/sites-available/default | grep -q "^# $APP_NAME: catch-all"; then
        rm -f /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default
        NGINX_TOUCHED=true
    fi
    if [ "$NGINX_TOUCHED" = true ]; then
        command -v nginx &>/dev/null && nginx -t 2>/dev/null && systemctl reload nginx || true
    fi
    print_status "✅ Uninstalled successfully!"
    exit 0
fi

# Check root
if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root (use sudo)"
    exit 1
fi

print_status "🚀 Starting Installation... (app=$INSTALL_APP nginx=$INSTALL_NGINX)"

# ═── App ──────────────────────────────────────────────────────
if [ "$INSTALL_APP" = true ]; then

# ─── System Dependencies ──────────────────────────────────────
print_status "Installing system dependencies (curl)..."
if command -v apt-get &>/dev/null; then
    apt-get update -qq
    apt-get install -y -qq curl
elif command -v yum &>/dev/null; then
    yum install -y curl
elif command -v dnf &>/dev/null; then
    dnf install -y curl
fi

# ─── Stop existing service ────────────────────────────────────
systemctl stop ${SERVICE_NAME} 2>/dev/null || true

# ─── Create app directory ─────────────────────────────────────
print_status "Creating app directory: $APP_DIR"
mkdir -p "$APP_DIR"
cd "$APP_DIR"

# ─── Download binary ──────────────────────────────────────────
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    BINARY="linux"
elif [ "$ARCH" = "aarch64" ]; then
    BINARY="linux-arm64"
else
    print_error "Unsupported architecture: $ARCH"
    exit 1
fi

print_status "Downloading binary ($BINARY) from latest release..."
curl -fsSL "$RELEASES_URL/$BINARY" -o "$APP_DIR/$APP_NAME"
chmod +x "$APP_DIR/$APP_NAME"

# ─── Create .env ─────────────────────────────────────────────
print_status "Creating .env file..."
cat > "$APP_DIR/.env" <<EOF
DATABASE_URL=$DATABASE_URL
REDIS_URL=$REDIS_URL
PORT=$PORT
DOMAIN_STATIC=$DOMAIN_STATIC
EOF

# ─── Systemd service ──────────────────────────────────────────
print_status "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=VdoHide Content Node
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$APP_NAME
Restart=always
RestartSec=5
EnvironmentFile=$APP_DIR/.env

[Install]
WantedBy=multi-user.target
EOF

# ─── Enable & start ───────────────────────────────────────────
systemctl daemon-reload
systemctl enable ${SERVICE_NAME}
systemctl start ${SERVICE_NAME}

fi # INSTALL_APP

# ═── Nginx ────────────────────────────────────────────────────
if [ "$INSTALL_NGINX" = true ]; then
    print_status "Configuring Nginx..."

    if ! command -v nginx &>/dev/null; then
        print_status "Installing Nginx..."
        apt-get update -qq
        apt-get install -y nginx
        systemctl enable nginx
        systemctl start  nginx
    fi

    if [ "${#DOMAINS[@]}" -eq 0 ]; then
        # ── Catch-all: รับทุกโดเมน ────────────────────────────
        #
        # เขียนลงไฟล์ของตัวเอง ไม่แตะ sites-available/default เพราะไฟล์นั้น
        # อาจเป็นของ app อื่นบนเครื่องเดียวกัน (เช่น player-node) — ของเดิม
        # เขียนทับทำให้ vhost ของเขาหายเงียบๆ
        #
        # ถ้ามี vhost อื่นจอง default_server อยู่แล้ว เราจะไม่ประกาศซ้ำ
        # (nginx จะ error "duplicate default server" แล้ว reload ไม่ผ่านทั้งเครื่อง)
        DEFAULT_OWNER=""
        if [ -d /etc/nginx/sites-enabled ]; then
            DEFAULT_OWNER=$(grep -rls "default_server" /etc/nginx/sites-enabled/ 2>/dev/null \
                | grep -v "/${APP_NAME}$" | head -1)
        fi

        LISTEN_80="listen 80 default_server;"
        LISTEN_V6="listen [::]:80 default_server;"
        if [ -n "$DEFAULT_OWNER" ]; then
            print_warning "มี vhost อื่นจอง default_server อยู่แล้ว: $DEFAULT_OWNER"
            print_warning "จะไม่ประกาศ default_server ซ้ำ — โดเมนที่ไม่ตรง vhost ไหนจะไปที่ตัวนั้นแทน"
            print_warning "ถ้าต้องการให้ $APP_NAME รับทุกโดเมน ให้ระบุ --domain หรือย้าย vhost นั้นออก"
            LISTEN_80="listen 80;"
            LISTEN_V6="listen [::]:80;"
        fi

        print_status "No --domain → catch-all (accept ALL domains) → $APP_HOST:$PORT"
        cat > /etc/nginx/sites-available/$APP_NAME <<EOF
# $APP_NAME: catch-all — accepts every domain
$(nginx_conn_map)

upstream $APP_NAME {
    server $APP_HOST:$PORT;
    keepalive          32;
    keepalive_timeout  60s;
    keepalive_requests 1000;
}

server {
    $LISTEN_80
    $LISTEN_V6
    server_name _;

    proxy_buffering         off;
    proxy_request_buffering off;

    location / {
        proxy_pass         http://$APP_NAME;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade           \$http_upgrade;
        proxy_set_header   Connection        \$${CONN_VAR};
        proxy_set_header   Host              \$host;
        proxy_set_header   X-Real-IP         \$remote_addr;
        proxy_set_header   X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
EOF
        ln -sf /etc/nginx/sites-available/$APP_NAME /etc/nginx/sites-enabled/$APP_NAME

        # เวอร์ชันก่อนหน้าเขียน catch-all ทับ sites-available/default ไว้
        # — ถ้าเจอไฟล์ที่เป็นของเรา (มี marker) ให้เก็บกวาดทิ้ง ไฟล์ของคนอื่นไม่แตะ
        if [ -f /etc/nginx/sites-available/default ] &&
            head -1 /etc/nginx/sites-available/default | grep -q "^# $APP_NAME: catch-all"; then
            print_warning "พบ catch-all เก่าของ $APP_NAME ใน sites-available/default — ลบทิ้ง"
            rm -f /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default
        fi
    else
        # ── โดเมนเฉพาะ ────────────────────────────────────────
        SERVER_NAMES="${DOMAINS[*]}"
        print_status "Domains: $SERVER_NAMES → $APP_HOST:$PORT"
        cat > /etc/nginx/sites-available/$APP_NAME <<EOF
$(nginx_conn_map)

upstream $APP_NAME {
    server $APP_HOST:$PORT;
    keepalive          32;
    keepalive_timeout  60s;
    keepalive_requests 1000;
}

server {
    listen 80;
    server_name $SERVER_NAMES;

    proxy_buffering         off;
    proxy_request_buffering off;

    location / {
        proxy_pass         http://$APP_NAME;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade           \$http_upgrade;
        proxy_set_header   Connection        \$${CONN_VAR};
        proxy_set_header   Host              \$host;
        proxy_set_header   X-Real-IP         \$remote_addr;
        proxy_set_header   X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
EOF
        ln -sf /etc/nginx/sites-available/$APP_NAME /etc/nginx/sites-enabled/
    fi

    if nginx -t; then
        systemctl reload nginx
        print_status "✅ Nginx configured"
    else
        print_error "❌ Nginx configuration test failed."
        exit 1
    fi
fi # INSTALL_NGINX

# ═── Done ─────────────────────────────────────────────────────
sleep 2
echo ""
echo "============================================"
if [ "$INSTALL_APP" = true ]; then
    if systemctl is-active --quiet ${SERVICE_NAME}; then
        print_status "✅ Installation completed successfully!"
    else
        print_warning "Service not running — check logs below"
        journalctl -u "${SERVICE_NAME}" -n 15 --no-pager
    fi
fi
echo "============================================"
echo ""
echo "  Port:       $PORT"
if [ "${#DOMAINS[@]}" -gt 0 ]; then
    echo "  Domains:"
    for d in "${DOMAINS[@]}"; do echo "    • http://$d"; done
elif [ "$INSTALL_NGINX" = true ]; then
    echo "  Domains:    all (catch-all)"
fi
echo ""
echo "  Commands:"
echo "    View logs:  journalctl -u ${SERVICE_NAME} -f"
echo "    Restart:    systemctl restart ${SERVICE_NAME}"
echo "    Health:     curl http://localhost:$PORT/health"
echo "    Uninstall:  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh | sudo bash -s -- --uninstall"
echo "============================================"
