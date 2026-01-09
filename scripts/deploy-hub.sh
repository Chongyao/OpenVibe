#!/bin/bash
# Deploy Hub to Huawei Cloud server
# Run this from the project root on Arch server

set -e

REMOTE_HOST="huawei"
REMOTE_USER="zcy"
REMOTE_DIR="/home/zcy/openvibe"
HUB_BINARY="hub/bin/openvibe-hub"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Deploying OpenVibe Hub to Huawei Cloud${NC}"
echo "========================================"

# Step 1: Build for Linux
echo -e "${YELLOW}[1/4] Building Hub binary...${NC}"
cd hub
GOOS=linux GOARCH=amd64 go build -o bin/openvibe-hub ./cmd/hub
cd ..
echo -e "${GREEN}✅ Build complete${NC}"

# Step 2: Create remote directory
echo -e "${YELLOW}[2/4] Preparing remote directory...${NC}"
ssh $REMOTE_HOST "mkdir -p $REMOTE_DIR"
echo -e "${GREEN}✅ Directory ready${NC}"

# Step 3: Copy binary
echo -e "${YELLOW}[3/4] Copying binary to server...${NC}"
scp $HUB_BINARY $REMOTE_HOST:$REMOTE_DIR/
ssh $REMOTE_HOST "chmod +x $REMOTE_DIR/openvibe-hub"
echo -e "${GREEN}✅ Binary deployed${NC}"

# Step 4: Copy systemd service
echo -e "${YELLOW}[4/4] Installing systemd service...${NC}"
scp scripts/openvibe-hub.service $REMOTE_HOST:/tmp/
ssh $REMOTE_HOST "echo 'zcy123456' | sudo -S cp /tmp/openvibe-hub.service /etc/systemd/system/ && \
    sudo systemctl daemon-reload && \
    sudo systemctl enable openvibe-hub && \
    sudo systemctl restart openvibe-hub"
echo -e "${GREEN}✅ Service installed and started${NC}"

# Show status
echo ""
echo -e "${YELLOW}Service Status:${NC}"
ssh $REMOTE_HOST "sudo systemctl status openvibe-hub --no-pager" || true

echo ""
echo -e "${GREEN}=======================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${GREEN}Hub running at: http://121.36.218.61:8080${NC}"
echo -e "${GREEN}=======================================${NC}"
