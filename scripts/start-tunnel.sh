#!/bin/bash
# Create SSH reverse tunnel from Arch server to Huawei Cloud
# This allows Hub on Huawei to access OpenCode on Arch via localhost:4096
#
# Run this on the Arch server (where OpenCode runs)

set -e

REMOTE_HOST="huawei"
LOCAL_PORT=4096   # OpenCode port on Arch
REMOTE_PORT=4096  # Port on Huawei Cloud (Hub will connect to this)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Starting SSH Reverse Tunnel${NC}"
echo "=============================="
echo ""
echo "Local:  localhost:$LOCAL_PORT (OpenCode)"
echo "Remote: $REMOTE_HOST:$REMOTE_PORT (Hub can access)"
echo ""

# Check if tunnel already exists
if ssh -O check $REMOTE_HOST 2>/dev/null; then
    echo -e "${YELLOW}Existing SSH connection found, reusing...${NC}"
fi

# Start tunnel in background with auto-reconnect
while true; do
    echo -e "${GREEN}Establishing tunnel...${NC}"
    
    ssh -N -R $REMOTE_PORT:localhost:$LOCAL_PORT $REMOTE_HOST \
        -o ServerAliveInterval=30 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        -o StrictHostKeyChecking=no
    
    echo -e "${RED}Tunnel disconnected, reconnecting in 5 seconds...${NC}"
    sleep 5
done
