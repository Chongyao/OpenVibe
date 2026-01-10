#!/bin/bash
# OpenVibe One-Click Deploy Script
# Deploys both Hub and Frontend to Huawei Cloud
# Usage: ./scripts/deploy.sh [--sudo-pass PASSWORD]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
REMOTE_HOST="huawei"
REMOTE_DIR="/home/zcy/openvibe"
SUDO_PASS=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --sudo-pass)
            SUDO_PASS="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
echo "╔═══════════════════════════════════════════╗"
echo "║     🚀 OpenVibe One-Click Deploy 🚀       ║"
echo "╚═══════════════════════════════════════════╝"
echo -e "${NC}"

# Step 1: Build Frontend
echo -e "${YELLOW}[1/5] Building Next.js frontend...${NC}"
cd "$PROJECT_DIR/app"
npm run build 2>&1 | tail -5

if [ ! -d "out" ]; then
    echo -e "${RED}❌ Frontend build failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Frontend built${NC}"

# Step 2: Build Hub
echo -e "${YELLOW}[2/5] Building Go Hub binary...${NC}"
cd "$PROJECT_DIR/hub"
GOOS=linux GOARCH=amd64 go build -o bin/openvibe-hub ./cmd/hub
echo -e "${GREEN}✅ Hub built${NC}"

# Step 3: Prepare deployment package
echo -e "${YELLOW}[3/5] Preparing deployment package...${NC}"
cd "$PROJECT_DIR"
rm -rf /tmp/openvibe-deploy
mkdir -p /tmp/openvibe-deploy/static

cp hub/bin/openvibe-hub /tmp/openvibe-deploy/
cp -r app/out/* /tmp/openvibe-deploy/static/

cat > /tmp/openvibe-deploy/openvibe-hub.service << 'EOF'
[Unit]
Description=OpenVibe Hub Server
After=network.target

[Service]
Type=simple
User=zcy
WorkingDirectory=/home/zcy/openvibe
ExecStart=/home/zcy/openvibe/openvibe-hub -port 8080 -opencode http://localhost:4096 -static /home/zcy/openvibe/static
Restart=always
RestartSec=5
Environment=OPENVIBE_TOKEN=

[Install]
WantedBy=multi-user.target
EOF

echo -e "${GREEN}✅ Package prepared${NC}"

# Step 4: Upload to server
echo -e "${YELLOW}[4/5] Uploading to Huawei cloud...${NC}"

# Stop existing process
ssh "$REMOTE_HOST" "pkill openvibe-hub 2>/dev/null || true"
sleep 1

# Create directory and upload
ssh "$REMOTE_HOST" "mkdir -p $REMOTE_DIR"
rsync -avz --delete /tmp/openvibe-deploy/ "$REMOTE_HOST:$REMOTE_DIR/" 2>&1 | grep -E "^(sent|total)" || true

echo -e "${GREEN}✅ Files uploaded${NC}"

# Step 5: Start service
echo -e "${YELLOW}[5/5] Starting Hub service...${NC}"

# Make executable and start
ssh "$REMOTE_HOST" "chmod +x $REMOTE_DIR/openvibe-hub"

# Try to install systemd service with sudo
if [ -n "$SUDO_PASS" ]; then
    ssh "$REMOTE_HOST" "echo '$SUDO_PASS' | sudo -S cp $REMOTE_DIR/openvibe-hub.service /etc/systemd/system/ && \
        sudo systemctl daemon-reload && \
        sudo systemctl enable openvibe-hub && \
        sudo systemctl restart openvibe-hub" 2>/dev/null && {
        echo -e "${GREEN}✅ Systemd service installed${NC}"
        SYSTEMD_OK=true
    } || SYSTEMD_OK=false
else
    SYSTEMD_OK=false
fi

# Fallback: run directly if systemd failed
if [ "$SYSTEMD_OK" != "true" ]; then
    echo -e "${YELLOW}⚠️  Running Hub directly (systemd not available)${NC}"
    ssh "$REMOTE_HOST" "cd $REMOTE_DIR && nohup ./openvibe-hub -port 8080 -opencode http://localhost:4096 -static ./static > hub.log 2>&1 &"
    sleep 2
fi

# Verify
echo ""
echo -e "${YELLOW}Verifying deployment...${NC}"
sleep 2

if curl -s http://121.36.218.61:8080/health | grep -q "ok"; then
    echo -e "${GREEN}✅ Health check passed${NC}"
else
    echo -e "${RED}❌ Health check failed${NC}"
    ssh "$REMOTE_HOST" "tail -20 $REMOTE_DIR/hub.log" 2>/dev/null || true
    exit 1
fi

if curl -s http://121.36.218.61:8080/ | grep -q "OpenVibe"; then
    echo -e "${GREEN}✅ Frontend accessible${NC}"
else
    echo -e "${YELLOW}⚠️  Frontend may have issues${NC}"
fi

echo ""
echo -e "${CYAN}╔═══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║${NC}  ${GREEN}✨ Deployment Complete! ✨${NC}               ${CYAN}║${NC}"
echo -e "${CYAN}║${NC}                                           ${CYAN}║${NC}"
echo -e "${CYAN}║${NC}  🌐 ${GREEN}http://121.36.218.61:8080${NC}            ${CYAN}║${NC}"
echo -e "${CYAN}╚═══════════════════════════════════════════╝${NC}"
echo ""
