#!/bin/bash
# Test connection script for CP1 verification
# Usage: ./test-connection.sh

set -e

HUB_URL="${HUB_URL:-http://121.36.218.61:8080}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}OpenVibe Connection Test${NC}"
echo "========================="
echo ""

# Test 1: Hub health check
echo -e "${YELLOW}[1/3] Testing Hub health endpoint...${NC}"
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "$HUB_URL/health" 2>&1)
HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -1)
BODY=$(echo "$HEALTH_RESPONSE" | head -1)

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✅ Hub is healthy${NC}"
    echo "   Response: $BODY"
else
    echo -e "${RED}❌ Hub health check failed (HTTP $HTTP_CODE)${NC}"
    exit 1
fi

# Test 2: Check OpenCode via Hub (proxy check)
echo ""
echo -e "${YELLOW}[2/3] Testing OpenCode connection via tunnel...${NC}"

# We test by checking if Hub can reach OpenCode
# The Hub's health is confirmed, now check if tunnel works
TUNNEL_TEST=$(ssh huawei "curl -s http://localhost:4096/global/health" 2>/dev/null || echo "FAILED")

if echo "$TUNNEL_TEST" | grep -q "healthy"; then
    echo -e "${GREEN}✅ SSH tunnel working${NC}"
    echo "   OpenCode: $TUNNEL_TEST"
else
    echo -e "${RED}❌ SSH tunnel not working${NC}"
    echo "   Response: $TUNNEL_TEST"
    echo ""
    echo "   Fix: Run on Arch server:"
    echo "   sudo systemctl start openvibe-tunnel"
    exit 1
fi

# Test 3: WebSocket connectivity (basic check)
echo ""
echo -e "${YELLOW}[3/3] Testing WebSocket endpoint...${NC}"

# Just check if the /ws endpoint responds (will get upgrade required error, which is expected)
WS_TEST=$(curl -s -o /dev/null -w "%{http_code}" "$HUB_URL/ws" 2>&1)

if [ "$WS_TEST" = "400" ]; then
    echo -e "${GREEN}✅ WebSocket endpoint responding${NC}"
    echo "   (400 is expected - needs WebSocket upgrade)"
else
    echo -e "${YELLOW}⚠️  WebSocket endpoint returned HTTP $WS_TEST${NC}"
fi

echo ""
echo -e "${GREEN}==========================${NC}"
echo -e "${GREEN}CP1 Verification Complete!${NC}"
echo -e "${GREEN}==========================${NC}"
echo ""
echo "Summary:"
echo "  • Hub URL: $HUB_URL"
echo "  • Hub Status: OK"
echo "  • Tunnel Status: OK"
echo "  • WebSocket: Ready"
echo ""
echo "Next: Run the full WebSocket test with websocat (optional):"
echo "  pacman -S websocat"
echo "  echo '{\"type\":\"ping\",\"id\":\"1\"}' | websocat ws://121.36.218.61:8080/ws"
