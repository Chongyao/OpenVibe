# AGENTS.md - OpenVibe Development Guide

**Generated:** 2026-01-11 | **Commit:** e8f9460 | **Branch:** main

> **Status**: Phase 2 (Go Core) | **Vision**: Mobile vibe coding terminal with E2EE + BYOS

## Architecture

```
App (Next.js) <--WS--> Hub (Go) <--WS Tunnel--> Agent (Go) <--HTTP--> OpenCode
                         |
                       Redis (message buffer)
```

| Component | Stack | Directory | Port |
|-----------|-------|-----------|------|
| Mobile App | Next.js 16, React 19, TailwindCSS 4 | `/app` | 3000 |
| Hub Server | Go 1.22, gorilla/websocket, go-redis | `/hub` | 8080 |
| Host Agent | Go 1.22, gorilla/websocket | `/agent` | - |

## Build Commands

```bash
# App (Next.js PWA)
cd app && npm run dev           # Dev server :3000
cd app && npm run build && npm run lint  # Build + lint

# Hub (Go)
cd hub && go run ./cmd/hub      # Server :8080
cd hub && go test -v -race ./...  # Tests with race detection

# Agent (Go)
cd agent && go run ./cmd/agent  # Single-project mode
cd agent && go run ./cmd/agent --projects /path1,/path2  # Multi-project

# Integration
./scripts/test-phase2.sh        # Full chain test
```

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| WebSocket client connection | `app/src/hooks/useWebSocket.ts` | Mosh-style sync, auto-reconnect |
| Message rendering | `app/src/components/MessageBubble.tsx` | Markdown + syntax highlight |
| Hub WebSocket server | `hub/internal/server/server.go` | Client connection handler |
| Agent tunnel manager | `hub/internal/tunnel/manager.go` | Agent registration, request routing |
| OpenCode HTTP client | `agent/internal/opencode/client.go` | SSE streaming to Hub |
| Multi-project routing | `agent/internal/project/manager.go` | Port pool, tmux sessions |
| Redis message buffer | `hub/internal/buffer/redis.go` | Mosh-style sync persistence |

## Code Style

### TypeScript/React

```typescript
'use client';  // Required for client components

// Import order: external -> @/ alias -> relative
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { Message } from '@/types';
import { MessageBubble } from './MessageBubble';

// Named exports, PascalCase components
export function TerminalView({ sessionId }: Props) {
  if (!sessionId) return null;  // Early returns first
  return <div>...</div>;
}

// memo() for performance-critical components
export const MessageBubble = memo(function MessageBubble({ message }: Props) {
  const parsedContent = useMemo(() => parseContent(message.content), [message.content]);
  return <div>{parsedContent}</div>;
});

// interface for objects, type for unions/literals
interface Message { id: string; content: string; }
type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error';
```

### Go

```go
// Import order: stdlib -> external -> internal (goimports)
import (
    "context"
    "fmt"
    
    "github.com/gorilla/websocket"
    
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
    return &Manager{config: cfg, agents: make(map[string]*Agent)}
}
```

## Key Protocols

### Mosh-Style Sync
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

## Security (Phase 3 Target)

**Target Crypto**: X25519 + AES-256-GCM + HKDF-SHA256

### Rules (NEVER VIOLATE)
1. **Never log plaintext** - encrypt sensitive data before logging
2. **Constant-time comparison** - `subtle.ConstantTimeCompare` for tokens
3. **Validate before process** - check message structure first

```go
// Good: constant-time token comparison
if subtle.ConstantTimeCompare([]byte(payload.Token), []byte(cfg.AgentToken)) != 1 {
    return ErrUnauthorized
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENVIBE_TOKEN` | Client auth token | (none) |
| `OPENVIBE_AGENT_TOKEN` | Agent auth token | (none) |
| `OPENVIBE_PROJECTS` | Comma-separated project paths | (none) |
| `REDIS_PASSWORD` | Redis password | (none) |
| `NEXT_PUBLIC_WS_URL` | WebSocket URL | auto-detect |

## Known Bugs & Solutions

### 1. Stale Closure in WebSocket onConnect
**Symptom**: Auto session creation fails on first load.
**Cause**: React state stale inside `onConnect` callback.
**Fix**: Use refs, don't check state in `onConnect` - it only fires when connected.

```typescript
// BAD: state is stale
onConnect: () => { if (state === 'connected') handleNewSession(); }

// GOOD: onConnect implies connected
onConnect: () => { if (!isCreatingRef.current) { isCreatingRef.current = true; send(...); } }
```

### 2. Browser Cache Serving Stale JS
**Symptom**: Changes deployed but browser runs old code.
**Fix**: Hub sets cache headers - HTML: `no-cache`, `/_next/static/`: `immutable`.

### 3. Messages Sent to Wrong OpenCode Instance (RESOLVED)
**Symptom**: User selects Project A, gets response from Project B.
**Cause**: `prompt` request was missing `projectPath`, Agent used default URL.
**Fix**: Commit `e8f9460` - Frontend now passes `projectPath` in prompt request, Hub forwards to Agent, Agent routes to correct OpenCode instance.
**Status**: ✅ FIXED - Full chain: Frontend → Hub → Agent → OpenCode all pass projectPath correctly.

### 4. sendRef May Be Null
**Symptom**: Clicking Start does nothing.
**Cause**: `sendRef.current` set in useEffect after first render.
**Fix**: Added null check with console.warn.

### 5. Session Directory Not Used for Routing
**Symptom**: Session created with correct directory, subsequent messages go elsewhere.
**Cause**: OpenCode API doesn't persist `directory` field.
**Fix**: Track session->projectPath mapping in Agent or Hub.

## Git Conventions

- Branches: `feat/`, `fix/`, `refactor/`, `docs/`
- Commits: Conventional (`feat:`, `fix:`, `chore:`, `refactor:`)
- PRs: <400 lines, no force push to `main`

## UI Theme - Cyberpunk Console

```css
:root {
  --bg-primary: #09090b;      /* Deep space gray */
  --bg-secondary: #18181b;    /* Card background */
  --accent-primary: #00ff9d;  /* Neon green */
  --accent-error: #ff0055;    /* Cyber red */
}
```
