#!/bin/bash
# Deploy Hub to Huawei Cloud server
# Run this from the project root on Arch server
#
# Prerequisites:
# - SSH key auth configured for huawei host
# - User should have passwordless sudo OR run systemd commands manually

set -e

REMOTE_HOST="huawei"
REMOTE_USER="zcy"
REMOTE_DIR="/home/zcy/openvibe"
HUB_BINARY="hub/bin/openvibe-hub"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Deploying OpenVibe Hub to Huawei Cloud${NC}"
echo "========================================"

echo -e "${YELLOW}[1/4] Building Hub binary...${NC}"
cd hub
GOOS=linux GOARCH=amd64 go build -o bin/openvibe-hub ./cmd/hub
cd ..
echo -e "${GREEN}✅ Build complete${NC}"

echo -e "${YELLOW}[2/4] Preparing remote directory...${NC}"
ssh $REMOTE_HOST "mkdir -p $REMOTE_DIR"
echo -e "${GREEN}✅ Directory ready${NC}"

echo -e "${YELLOW}[3/4] Copying binary to server...${NC}"
scp $HUB_BINARY $REMOTE_HOST:$REMOTE_DIR/
ssh $REMOTE_HOST "chmod +x $REMOTE_DIR/openvibe-hub"
echo -e "${GREEN}✅ Binary deployed${NC}"

echo -e "${YELLOW}[4/4] Installing systemd service...${NC}"
scp scripts/openvibe-hub.service $REMOTE_HOST:/tmp/

echo -e "${YELLOW}NOTE: Systemd installation requires sudo. Run these commands manually if needed:${NC}"
echo "  sudo cp /tmp/openvibe-hub.service /etc/systemd/system/"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl enable openvibe-hub"
echo "  sudo systemctl restart openvibe-hub"

ssh $REMOTE_HOST "sudo cp /tmp/openvibe-hub.service /etc/systemd/system/ && \
    sudo systemctl daemon-reload && \
    sudo systemctl enable openvibe-hub && \
    sudo systemctl restart openvibe-hub" 2>/dev/null || {
    echo -e "${YELLOW}⚠️  Sudo commands failed. Please run manually on the remote server.${NC}"
}

echo ""
echo -e "${YELLOW}Service Status:${NC}"
ssh $REMOTE_HOST "sudo systemctl status openvibe-hub --no-pager" 2>/dev/null || \
    ssh $REMOTE_HOST "pgrep -a openvibe-hub" || \
    echo "Unable to check status - verify manually"

echo ""
echo -e "${GREEN}=======================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}Hub running at: http://121.36.218.61:8080${NC}"
echo -e "${GREEN}=======================================${NC}"
