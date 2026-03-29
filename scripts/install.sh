#!/usr/bin/env bash
set -euo pipefail

# CloudGuard Monitor — Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/trekmax/cloudguard-monitor/main/scripts/install.sh | bash

VERSION="${CLOUDGUARD_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/cloudguard"
DATA_DIR="/var/lib/cloudguard"
SERVICE_USER="cloudguard"
REPO="trekmax/cloudguard-monitor"

info()  { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m    $*"; }
err()   { echo -e "\033[1;31m[ERROR]\033[0m $*" >&2; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            err "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    if [ "$OS" != "linux" ]; then
        err "This install script only supports Linux. OS detected: $OS"
        exit 1
    fi

    info "Platform: ${OS}/${ARCH}"
}

# Check for root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Create service user
create_user() {
    if ! id -u "$SERVICE_USER" &>/dev/null; then
        useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
        info "Created system user: $SERVICE_USER"
    fi
}

# Download binary
download_binary() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/cloudguard-${OS}-${ARCH}"
    info "Downloading CloudGuard Monitor v${VERSION}..."

    curl -fsSL -o "${INSTALL_DIR}/cloudguard" "$DOWNLOAD_URL"

    # Verify checksum
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"
    if curl -fsSL -o /tmp/cloudguard-checksums.txt "$CHECKSUM_URL" 2>/dev/null; then
        EXPECTED=$(grep "cloudguard-${OS}-${ARCH}$" /tmp/cloudguard-checksums.txt | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            ACTUAL=$(sha256sum "${INSTALL_DIR}/cloudguard" | awk '{print $1}')
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                rm -f "${INSTALL_DIR}/cloudguard"
                err "Checksum verification failed! Binary removed."
                exit 1
            fi
            ok "Checksum verified"
        fi
        rm -f /tmp/cloudguard-checksums.txt
    else
        info "Checksum file not available, skipping verification"
    fi

    chmod +x "${INSTALL_DIR}/cloudguard"
    ok "Binary installed to ${INSTALL_DIR}/cloudguard"
}

# Setup configuration
setup_config() {
    mkdir -p "$CONFIG_DIR" "$DATA_DIR"
    chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"

    if [ ! -f "${CONFIG_DIR}/cloudguard.yaml" ]; then
        # Generate token
        TOKEN=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p | tr -d '\n')

        cat > "${CONFIG_DIR}/cloudguard.yaml" <<YAML
server:
  host: "0.0.0.0"
  port: 8080

collector:
  cpu_interval: 5
  memory_interval: 5
  disk_interval: 30
  network_interval: 5

database:
  path: "${DATA_DIR}/cloudguard.db"
  retention_days: 30

log:
  level: "info"
  format: "text"

auth:
  token: "${TOKEN}"

tls:
  enabled: false
  auto_cert: true

security:
  ip_whitelist: []
YAML

        chmod 600 "${CONFIG_DIR}/cloudguard.yaml"
        chown "$SERVICE_USER:$SERVICE_USER" "${CONFIG_DIR}/cloudguard.yaml"
        ok "Configuration created at ${CONFIG_DIR}/cloudguard.yaml"
    else
        info "Configuration already exists, skipping"
        TOKEN=$(grep 'token:' "${CONFIG_DIR}/cloudguard.yaml" | head -1 | awk '{print $2}' | tr -d '"')
    fi
}

# Install systemd service
install_service() {
    cat > /etc/systemd/system/cloudguard.service <<EOF
[Unit]
Description=CloudGuard Monitor Server
After=network.target

[Service]
Type=simple
User=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/cloudguard --config ${CONFIG_DIR}/cloudguard.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536
WorkingDirectory=${DATA_DIR}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable cloudguard
    systemctl start cloudguard
    ok "systemd service installed and started"
}

# Print summary
print_summary() {
    echo ""
    echo "=========================================="
    ok "CloudGuard Monitor installed successfully!"
    echo "=========================================="
    echo ""
    echo "  API Endpoint:  http://$(hostname -I | awk '{print $1}'):8080"
    echo "  API Token:     ${TOKEN}"
    echo "  Config:        ${CONFIG_DIR}/cloudguard.yaml"
    echo "  Data:          ${DATA_DIR}/"
    echo ""
    echo "  Service:       systemctl status cloudguard"
    echo "  Logs:          journalctl -u cloudguard -f"
    echo ""
    echo "  Connect via CLI:"
    echo "    cloudguard-cli connect http://$(hostname -I | awk '{print $1}'):8080 --token ${TOKEN}"
    echo ""
}

# Main
main() {
    info "Installing CloudGuard Monitor..."
    check_root
    detect_platform
    create_user
    download_binary
    setup_config
    install_service
    print_summary
}

main "$@"
