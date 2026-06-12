#!/usr/bin/env bash
# ==============================================================================
# deploy_vps.sh - Sovereign PDF VPS Initializer & Deployment Orchestrator
# This script configures a clean remote Ubuntu server for production execution.
# Run this script on the target VPS with root privileges: sudo ./deploy_vps.sh
# ==============================================================================

set -euo pipefail

# Configuration Variables
APP_USER="sovereign"
APP_DIR="/opt/sovereign-pdf"
DATA_DIR="/var/lib/sovereign-pdf"
PORT=8080

echo "==> [1/5] Preparing Host & Installing System Dependencies..."
# 1. Update and clean repository lists
apt-get update -y
apt-get upgrade -y

# 2. Install core C/C++ engine dependencies and reverse proxy packages
apt-get install -y \
    tesseract-ocr \
    libreoffice \
    poppler-utils \
    ghostscript \
    nginx \
    ufw \
    curl

echo "==> [2/5] Creating System User & Directory Structures..."
# 3. Create a dedicated system user with no login shell for security
if ! id -u "$APP_USER" >/dev/null 2>&1; then
    useradd -r -s /usr/sbin/nologin -d "$APP_DIR" "$APP_USER"
fi

# 4. Construct execution and storage directories
mkdir -p "$APP_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$DATA_DIR/tmp_uploads"
mkdir -p "$DATA_DIR/tmp_outputs"

# Assign directory ownership to the non-root application user
chown -R "$APP_USER":"$APP_USER" "$APP_DIR"
chown -R "$APP_USER":"$APP_USER" "$DATA_DIR"

echo "==> [3/5] Constructing Systemd Service configuration..."
# 5. Build systemd daemon unit file to run the server in the background securely
cat <<EOF > /etc/systemd/system/sovereign-pdf.service
[Unit]
Description=Sovereign PDF Daemon Service
After=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_USER
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/sovereign-pdf -port=$PORT -data=$DATA_DIR
Restart=always
RestartSec=5
LimitNOFILE=65535

# Basic sandbox protections
ProtectSystem=full
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd to load the new unit file
systemctl daemon-reload

echo "==> [4/5] Constructing Nginx Reverse Proxy Config..."
# 6. Configure Nginx virtual host with 100M payload threshold for large documents
cat <<EOF > /etc/nginx/sites-available/sovereign-pdf
server {
    listen 80;
    server_name _; # Bind to all incoming hostnames (add domain name here if available)

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:$PORT;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # Disable buffering to support Server-Sent Events (SSE) streaming updates
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 24h;
    }
}
EOF

# Enable the configuration and test Nginx syntax
ln -sf /etc/nginx/sites-available/sovereign-pdf /etc/nginx/sites-enabled/default
nginx -t
systemctl restart nginx

echo "==> [5/5] Hardening System Network via UFW..."
# 7. Configure firewall constraints: allow SSH (so you don't lock yourself out!) and web traffic
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'Allow SSH Management'
ufw allow 80/tcp comment 'Allow HTTP Traffic'
ufw allow 443/tcp comment 'Allow HTTPS Traffic (SSL)'
ufw --force enable

echo "=============================================================================="
echo " Deployment Environment Initialized Successfully!"
echo "=============================================================================="
echo " Next Steps:"
echo " 1. Move your compiled Go binary to the VPS target directory:"
echo "    scp build/sovereign-pdf-linux-amd64 root@<vps-ip>:$APP_DIR/sovereign-pdf"
echo " 2. Fix binary execution permissions and start the systemd service:"
echo "    ssh root@<vps-ip> \"chown $APP_USER:$APP_USER $APP_DIR/sovereign-pdf && chmod +x $APP_DIR/sovereign-pdf && systemctl enable --now sovereign-pdf\""
echo " 3. Verify execution logs:"
echo "    journalctl -u sovereign-pdf.service -f"
echo "=============================================================================="
