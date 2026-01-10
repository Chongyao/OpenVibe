# AGENTS.md - OpenVibe Development Guide

> **Status**: Phase 2 (Go Core) | **Vision**: Mobile vibe coding terminal with E2EE + BYOS

## Build Commands

```bash
# Mobile App (Next.js 16 PWA) - /app
cd app
npm run dev                              # Dev server at :3000
npm run build && npm run lint            # Build + lint check
npm run lint                             # ESLint only

# Hub Server (Go 1.22+) - /hub
cd hub
go run ./cmd/hub                         # Run server at :8080
go run ./cmd/hub --port 8080 \
  --opencode http://localhost:4096 \
  --redis localhost:6379 \
  --agent-token secret                   # Full options
go test -v ./... -run TestName           # Run single test
go test -v -race ./...                   # All tests with race detection

# Host Agent (Go 1.22+) - /agent
cd agent
go run ./cmd/agent                       # Run agent
go run ./cmd/agent --hub ws://hub:8080/agent \
  --opencode http://localhost:4096 \
  --token secret                         # Full options

# Integration test
./scripts/test-phase2.sh                 # Full chain test
```

## Architecture

```
App (Next.js) <--WS--> Hub (Go) <--WS Tunnel--> Agent (Go) <--HTTP--> OpenCode
                         |
                       Redis (消息缓冲)
```

| Component | Stack | Directory | Status |
|-----------|-------|-----------|--------|
| Mobile App | Next.js 16, React 19, TailwindCSS 4 | `/app` | Active |
| Hub Server | Go 1.22, gorilla/websocket, go-redis | `/hub` | Active |
| Host Agent | Go 1.22, gorilla/websocket | `/agent` | Active |

## Code Style

### TypeScript/React

```typescript
// 'use client' for client components, file extension .tsx for JSX
'use client';

// Imports: external -> internal alias -> relative (alphabetized)
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { Message, ServerMessage } from '@/types';
import { MessageBubble } from './MessageBubble';

// Named exports, PascalCase components, camelCase functions
export function TerminalView({ sessionId }: Props) {
  if (!sessionId) return null;  // Early returns first
  return <div>...</div>;
}

// Use memo for performance-critical components
export const MessageBubble = memo(function MessageBubble({ message }: Props) {
  const parsedContent = useMemo(() => parseContent(message.content), [message.content]);
  return <div>{parsedContent}</div>;
});

// Types: interface for objects, type for unions/literals
interface Message { id: string; content: string; msgId?: number; }
type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';
```

### Go

```go
// Imports: stdlib -> external -> internal (goimports)
import (
    "context"
    "fmt"
    
    "github.com/gorilla/websocket"
    "github.com/redis/go-redis/v9"
    
    "github.com/openvibe/hub/internal/buffer"
)

// Pointer receiver for mutations, wrap errors with context
func (b *RedisBuffer) Push(ctx context.Context, sessionID string, msg Message) (int64, error) {
    id, err := b.client.Incr(ctx, b.keyMsgID(sessionID)).Result()
    if err != nil {
        return 0, fmt.Errorf("failed to get next id: %w", err)
    }
    return id, nil
}

// Constructor pattern: New* returns pointer
func NewManager(cfg *Config) *Manager {
    return &Manager{
        config: cfg,
        agents: make(map[string]*Agent),
    }
}
```

## Security (Phase 3 Target)

**Target Crypto**: X25519 key exchange + AES-256-GCM + HKDF-SHA256

### Rules (NEVER VIOLATE)
1. **Never log plaintext** - encrypt before logging sensitive data
2. **Constant-time comparison** - `subtle.ConstantTimeCompare` for tokens
3. **Validate before process** - check message structure first

```go
// Good: constant-time token comparison
if subtle.ConstantTimeCompare([]byte(payload.Token), []byte(cfg.AgentToken)) != 1 {
    return ErrUnauthorized
}
```

## UI/UX - Cyberpunk Console

```css
:root {
  --bg-primary: #09090b;      /* Deep space gray */
  --bg-secondary: #18181b;    /* Card background */
  --accent-primary: #00ff9d;  /* Neon green */
  --accent-error: #ff0055;    /* Cyber red */
}
```

## Key Protocols

### Mosh-Style Sync (断点续传)
```typescript
// Client tracks lastAckId, requests sync on reconnect
{ type: 'sync', payload: { sessionId, lastAckId: 1000 } }
// Server returns missed messages
{ type: 'sync.batch', payload: { messages: [...], latestId: 1050 } }
```

### Agent Tunnel
```go
// Agent registers with Hub
{ type: 'agent.register', payload: { agentId, token, capabilities } }
// Hub forwards requests
{ type: 'agent.request', id: 'req-1', payload: { sessionId, action, data } }
// Agent streams response
{ type: 'agent.stream', id: 'req-1', payload: { text: '...' } }
```

## File Structure

```
app/src/
  app/           # Next.js App Router pages
  components/    # Reusable UI components
  hooks/         # Custom React hooks (useWebSocket, useSettings)
  types/         # TypeScript interfaces

hub/
  cmd/hub/       # Entry point
  internal/
    buffer/      # Redis message buffering
    config/      # Configuration
    proxy/       # OpenCode HTTP proxy (fallback)
    server/      # WebSocket server
    tunnel/      # Agent tunnel manager

agent/
  cmd/agent/     # Entry point
  internal/
    opencode/    # OpenCode HTTP client
    tunnel/      # Tunnel client
```

## Git Conventions

- Branches: `feat/`, `fix/`, `refactor/`, `docs/`
- Commits: Conventional (`feat:`, `fix:`, `chore:`, `refactor:`)
- PRs: <400 lines changed, no force push to `main`

## Testing

```bash
# Hub integration test
./scripts/test-phase2.sh

# With Redis
REDIS_ADDR=localhost:6379 ./scripts/test-phase2.sh
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENVIBE_TOKEN` | Client auth token | (none) |
| `OPENVIBE_AGENT_TOKEN` | Agent auth token | (none) |
| `REDIS_PASSWORD` | Redis password | (none) |
| `NEXT_PUBLIC_WS_URL` | WebSocket URL | auto-detect |

## Known Bugs & Solutions

### 1. Stale Closure in WebSocket onConnect Callback
**Symptom**: Auto session creation fails on first load, but manual "New Chat" works.

**Root Cause**: React state is stale inside `onConnect` callback. When `onConnect` fires, `state` is still `'disconnected'` due to closure, so `if (state !== 'connected')` check fails.

**Solution**: Don't rely on React state inside WebSocket callbacks. Use refs or call `send()` directly since `onConnect` only fires when connected.

```typescript
// BAD: state is stale
onConnect: () => {
  if (state === 'connected') handleNewSession(); // state is 'disconnected' here!
}

// GOOD: onConnect only fires when connected, no state check needed
onConnect: () => {
  if (!isCreatingSessionRef.current) {
    isCreatingSessionRef.current = true;
    send({ type: 'session.create', ... });
  }
}
```

### 2. Browser Cache Serving Stale JavaScript
**Symptom**: Changes deployed but browser still runs old code.

**Root Cause**: Hub serves static files without `Cache-Control` headers.

**Solution**: Add cache headers in Hub's static file handler:
- HTML files: `Cache-Control: no-cache, no-store, must-revalidate`
- Static assets (`/_next/static/`): `Cache-Control: public, max-age=31536000, immutable`

### 3. Agent Disconnection Not Obvious to User
**Symptom**: User sees "Connecting..." forever, no error message.

**Root Cause**: Hub returns generic error when no agent is connected.

**Solution**: Check agent status first and return friendly error message:
```go
if agent, ok := tunnelMgr.GetAnyAgent(); !ok {
    sendError("No agent connected. Please start the OpenVibe agent on your development server.")
}
```

### 4. WebSocket Close Code 1005 (No Status)
**Symptom**: Hub logs show `websocket: close 1005 (no status)` immediately after client connects.

**Possible Causes**:
1. Client JavaScript error before WebSocket fully initializes
2. Browser security policy blocking WebSocket
3. Network/firewall issues
4. Stale cached JavaScript (see bug #2)

**Debug Steps**:
1. Check browser console for JS errors
2. Verify `curl http://hub:8080/agents` shows connected agent
3. Test with Node.js WebSocket client to isolate browser issues
4. Hard refresh browser (Ctrl+Shift+R) to clear cache
