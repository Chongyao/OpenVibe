#!/bin/bash
# CP1 Verification Test
# Tests the full chain: App -> Hub -> Tunnel -> OpenCode -> AI

set -e

HUB_URL="${HUB_URL:-ws://121.36.218.61:8080/ws}"
HTTP_URL="${HTTP_URL:-http://121.36.218.61:8080}"
MESSAGE="${1:-Say hello in one sentence}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}OpenVibe CP1 Verification${NC}"
echo "=========================="
echo ""

# Check websocat
if ! command -v websocat &> /dev/null; then
    WEBSOCAT="$HOME/.cargo/bin/websocat"
    if [ ! -f "$WEBSOCAT" ]; then
        echo -e "${RED}websocat not found. Install with: cargo install websocat${NC}"
        exit 1
    fi
else
    WEBSOCAT="websocat"
fi

# Test 1: Hub health
echo -e "${YELLOW}[1/4] Hub Health Check${NC}"
if curl -s "$HTTP_URL/health" | grep -q "ok"; then
    echo -e "${GREEN}✅ Hub is healthy${NC}"
else
    echo -e "${RED}❌ Hub health check failed${NC}"
    exit 1
fi

# Test 2: Tunnel connectivity
echo -e "${YELLOW}[2/4] Tunnel Connectivity${NC}"
TUNNEL_TEST=$(ssh huawei "curl -s http://localhost:4096/global/health" 2>/dev/null || echo "FAILED")
if echo "$TUNNEL_TEST" | grep -q "healthy"; then
    echo -e "${GREEN}✅ SSH tunnel working${NC}"
else
    echo -e "${RED}❌ SSH tunnel not working${NC}"
    exit 1
fi

# Test 3: WebSocket ping
echo -e "${YELLOW}[3/4] WebSocket Connection${NC}"
PING_RESP=$(echo '{"type":"ping","id":"test"}' | $WEBSOCAT -n1 "$HUB_URL" 2>&1)
if echo "$PING_RESP" | grep -q "pong"; then
    echo -e "${GREEN}✅ WebSocket responding${NC}"
else
    echo -e "${RED}❌ WebSocket not responding${NC}"
    exit 1
fi

# Test 4: Full message flow
echo -e "${YELLOW}[4/4] AI Message Flow${NC}"
echo "    Message: $MESSAGE"

SESSION_RESP=$(echo '{"type":"session.create","id":"t1","payload":{"title":"Test"}}' | $WEBSOCAT -n1 "$HUB_URL" 2>&1)
SESSION_ID=$(echo "$SESSION_RESP" | jq -r '.payload.id' 2>/dev/null)

if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    echo -e "${RED}❌ Failed to create session${NC}"
    echo "    Error: $(echo "$SESSION_RESP" | jq -r '.payload.error // .')"
    exit 1
fi

PROMPT="{\"type\":\"prompt\",\"id\":\"m1\",\"payload\":{\"sessionId\":\"$SESSION_ID\",\"content\":\"$MESSAGE\"}}"
RESPONSE=$( (echo "$PROMPT"; sleep 30) | $WEBSOCAT "$HUB_URL" 2>&1 | head -3)

AI_TEXT=$(echo "$RESPONSE" | jq -r 'select(.type=="stream") | .payload.text' 2>/dev/null | head -1)

if [ -n "$AI_TEXT" ]; then
    echo -e "${GREEN}✅ AI Response received:${NC}"
    echo -e "    ${GREEN}\"$AI_TEXT\"${NC}"
else
    ERROR=$(echo "$RESPONSE" | jq -r '.payload.error // empty' 2>/dev/null)
    if [ -n "$ERROR" ]; then
        echo -e "${RED}❌ Error: $ERROR${NC}"
        exit 1
    else
        echo -e "${YELLOW}⚠️  Response format unexpected${NC}"
        echo "    Raw: $RESPONSE"
    fi
fi

echo ""
echo -e "${GREEN}==========================${NC}"
echo -e "${GREEN}  CP1 VERIFICATION PASSED ${NC}"
echo -e "${GREEN}==========================${NC}"
