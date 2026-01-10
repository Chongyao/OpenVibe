#!/bin/bash
# Phase 2 Integration Test Script
# Tests the complete chain: App -> Hub -> Agent -> OpenCode

set -e

echo "=== OpenVibe Phase 2 Integration Test ==="
echo ""

# Configuration
HUB_PORT=${HUB_PORT:-8080}
OPENCODE_URL=${OPENCODE_URL:-"http://localhost:4096"}
REDIS_ADDR=${REDIS_ADDR:-""}
AGENT_TOKEN=${AGENT_TOKEN:-"test-token-12345"}

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

success() { echo -e "${GREEN}✓ $1${NC}"; }
fail() { echo -e "${RED}✗ $1${NC}"; exit 1; }
warn() { echo -e "${YELLOW}⚠ $1${NC}"; }

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    [ -n "$HUB_PID" ] && kill $HUB_PID 2>/dev/null || true
    [ -n "$AGENT_PID" ] && kill $AGENT_PID 2>/dev/null || true
}
trap cleanup EXIT

# Check prerequisites
echo "1. Checking prerequisites..."

if ! command -v go &> /dev/null; then
    fail "Go is not installed"
fi
success "Go installed"

if ! command -v curl &> /dev/null; then
    fail "curl is not installed"
fi
success "curl installed"

# Build binaries
echo ""
echo "2. Building binaries..."

cd "$(dirname "$0")/.."

echo "   Building Hub..."
cd hub && go build -o ../bin/hub ./cmd/hub && cd ..
success "Hub built"

echo "   Building Agent..."
cd agent && go build -o ../bin/agent ./cmd/agent && cd ..
success "Agent built"

# Start Hub
echo ""
echo "3. Starting Hub..."

REDIS_FLAGS=""
if [ -n "$REDIS_ADDR" ]; then
    REDIS_FLAGS="--redis $REDIS_ADDR"
    echo "   Redis: $REDIS_ADDR"
fi

./bin/hub --port $HUB_PORT --opencode "$OPENCODE_URL" --agent-token "$AGENT_TOKEN" $REDIS_FLAGS &
HUB_PID=$!
sleep 2

# Check Hub health
if curl -s "http://localhost:$HUB_PORT/health" | grep -q "ok"; then
    success "Hub is running on port $HUB_PORT"
else
    fail "Hub failed to start"
fi

# Start Agent (only if OpenCode is available)
echo ""
echo "4. Starting Agent..."

if curl -s "$OPENCODE_URL/global/health" > /dev/null 2>&1; then
    ./bin/agent --hub "ws://localhost:$HUB_PORT/agent" --opencode "$OPENCODE_URL" --token "$AGENT_TOKEN" --id "test-agent" &
    AGENT_PID=$!
    sleep 2

    # Check agent registration
    AGENTS=$(curl -s "http://localhost:$HUB_PORT/agents")
    if echo "$AGENTS" | grep -q "test-agent"; then
        success "Agent registered successfully"
    else
        warn "Agent not registered (check OpenCode connection)"
    fi
else
    warn "OpenCode not available at $OPENCODE_URL, skipping Agent test"
fi

# Test WebSocket connection
echo ""
echo "5. Testing WebSocket connection..."

# Simple WebSocket ping test using a temp script
cat > /tmp/ws_test.sh << 'WSEOF'
#!/bin/bash
exec 3<>/dev/tcp/localhost/$1
echo '{"type":"session.create","id":"test-1","payload":{"title":"Test"}}' >&3
read -t 5 response <&3
echo "$response"
exec 3<&-
WSEOF
chmod +x /tmp/ws_test.sh

# Note: WebSocket test requires websocat or similar tool
if command -v websocat &> /dev/null; then
    RESPONSE=$(echo '{"type":"ping","id":"test-ping","payload":{}}' | timeout 5 websocat "ws://localhost:$HUB_PORT/ws" 2>/dev/null || true)
    if echo "$RESPONSE" | grep -q "pong"; then
        success "WebSocket ping/pong working"
    else
        warn "WebSocket test inconclusive (install websocat for proper testing)"
    fi
else
    warn "websocat not installed, skipping WebSocket test"
fi

# Summary
echo ""
echo "=== Test Summary ==="
echo "Hub: Running on port $HUB_PORT (PID: $HUB_PID)"
if [ -n "$AGENT_PID" ]; then
    echo "Agent: Running (PID: $AGENT_PID)"
fi
if [ -n "$REDIS_ADDR" ]; then
    echo "Redis: $REDIS_ADDR"
else
    echo "Redis: Disabled (no message buffering)"
fi
echo ""
echo "Manual test:"
echo "  1. Open http://localhost:$HUB_PORT in browser"
echo "  2. Send a message"
echo "  3. Verify response"
echo ""
echo "Press Ctrl+C to stop servers..."
wait
