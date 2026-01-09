#!/bin/bash
# Test connection script for CP1 verification
# Usage: ./test-connection.sh "your message"

set -e

HUB_URL="${HUB_URL:-ws://121.36.218.61:8080/ws}"
TOKEN="${OPENVIBE_TOKEN:-}"
MESSAGE="${1:-Hello, can you introduce yourself?}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}OpenVibe Connection Test${NC}"
echo "========================="
echo ""

# Check if websocat is installed
if ! command -v websocat &> /dev/null; then
    echo -e "${RED}Error: websocat is not installed${NC}"
    echo "Install with: cargo install websocat"
    echo "Or: pacman -S websocat (Arch Linux)"
    exit 1
fi

# Build WebSocket URL with token
WS_URL="$HUB_URL"
if [ -n "$TOKEN" ]; then
    WS_URL="${HUB_URL}?token=${TOKEN}"
fi

echo -e "${YELLOW}Connecting to Hub...${NC}"
echo "URL: $HUB_URL"
echo ""

# Create a temp file for the response
RESPONSE_FILE=$(mktemp)
trap "rm -f $RESPONSE_FILE" EXIT

# Test 1: Health check
echo -e "${YELLOW}[1/3] Testing health endpoint...${NC}"
HEALTH_URL="http://121.36.218.61:8080/health"
if curl -s -f "$HEALTH_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Hub is healthy${NC}"
else
    echo -e "${RED}❌ Hub health check failed${NC}"
    echo "Make sure the Hub is running on the server"
    exit 1
fi

# Test 2: WebSocket connection
echo ""
echo -e "${YELLOW}[2/3] Testing WebSocket connection...${NC}"

# Send ping message
PING_MSG='{"type":"ping","id":"test-1","payload":null}'
RESPONSE=$(echo "$PING_MSG" | timeout 5 websocat -n1 "$WS_URL" 2>/dev/null || true)

if echo "$RESPONSE" | grep -q '"type":"pong"'; then
    echo -e "${GREEN}✅ WebSocket connection working${NC}"
else
    echo -e "${RED}❌ WebSocket connection failed${NC}"
    echo "Response: $RESPONSE"
    exit 1
fi

# Test 3: Create session and send message
echo ""
echo -e "${YELLOW}[3/3] Testing message sending...${NC}"
echo "Message: $MESSAGE"
echo ""

# Create session first
CREATE_MSG='{"type":"session.create","id":"test-2","payload":{"title":"Test Session"}}'

# Send message sequence
(
    echo "$CREATE_MSG"
    sleep 2
    echo "{\"type\":\"prompt\",\"id\":\"test-3\",\"payload\":{\"content\":\"$MESSAGE\"}}"
    sleep 30  # Wait for response
) | websocat "$WS_URL" 2>/dev/null | while read -r line; do
    TYPE=$(echo "$line" | jq -r '.type' 2>/dev/null)
    
    case "$TYPE" in
        "response")
            echo -e "${GREEN}✅ Session created${NC}"
            ;;
        "stream")
            # Extract and print content
            CONTENT=$(echo "$line" | jq -r '.payload' 2>/dev/null)
            echo -n "$CONTENT"
            ;;
        "stream.end")
            echo ""
            echo ""
            echo -e "${GREEN}✅ Response received successfully!${NC}"
            break
            ;;
        "error")
            ERROR=$(echo "$line" | jq -r '.payload.error' 2>/dev/null)
            echo -e "${RED}❌ Error: $ERROR${NC}"
            break
            ;;
    esac
done

echo ""
echo -e "${GREEN}==========================${NC}"
echo -e "${GREEN}CP1 Verification Complete!${NC}"
echo -e "${GREEN}==========================${NC}"
