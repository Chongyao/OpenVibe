# AGENTS.md - OpenVibe Development Guide

> **Status**: Greenfield | **Vision**: Mobile vibe coding terminal with E2EE + BYOS

## Build Commands

```bash
# Mobile App (Next.js PWA) - /app
npm run dev                              # Dev server
npm run build && npm run lint            # Build + lint
npm run test -- path/to/file.test.ts     # Single test

# Relay Server (Go) - /relay
go run ./cmd/relay                       # Run server
go test -v ./pkg/crypto -run TestName    # Single test

# Host Agent (Go) - /agent
go run ./cmd/agent                       # Run agent
go test -v -race ./internal/pty          # Tests with race detection

# Capacitor - Mobile builds
npx cap sync && npx cap open ios         # iOS
```

## Architecture

```
App (Next.js+Capacitor) <--E2EE--> Blind Hub (Go+Redis) <--E2EE--> Host Agent (Go)
```

| Component | Stack | Directory |
|-----------|-------|-----------|
| Mobile App | Next.js 14+, TypeScript, TailwindCSS, Capacitor | `/app` |
| Blind Hub | Go 1.22+, gorilla/websocket, Redis | `/relay` |
| Host Agent | Go 1.22+, creack/pty | `/agent` |

## Code Style

### TypeScript/React

```typescript
// Imports: external -> internal -> relative (alphabetized)
import { useState } from 'react';
import { useSession } from '@/hooks/useSession';
import { MessageCard } from './MessageCard';

// Named exports, PascalCase components, camelCase functions
export function TerminalView({ sessionId }: Props) {
  if (!sessionId) return null;  // Early returns
  return <div>...</div>;
}

// Types: interface for objects, type for unions
interface Message { id: string; content: string; }
type State = 'connecting' | 'connected' | 'disconnected';
```

### Go

```go
// Imports: stdlib -> external -> internal (goimports)
import (
    "context"
    "github.com/gorilla/websocket"
    "github.com/openvibe/agent/internal/pty"
)

// MixedCaps exported, mixedCaps unexported, -er interfaces
type Encryptor interface {
    Encrypt(ctx context.Context, data []byte) ([]byte, error)
}

// Pointer receiver for mutations, wrap errors with context
func (s *Session) Send(msg []byte) error {
    if err != nil {
        return fmt.Errorf("session send: %w", err)
    }
}
```

## Security (CRITICAL)

**Crypto**: X25519 key exchange + AES-256-GCM + HKDF-SHA256

### Rules (NEVER VIOLATE)

1. **Never log plaintext** - encrypt before logging
2. **Never store keys plaintext** - use OS keychain
3. **Constant-time comparison** - `subtle.ConstantTimeCompare`
4. **Zero secrets on exit** - explicit memory zeroing
5. **Validate before decrypt** - check message structure first

```go
// Good: constant-time
if subtle.ConstantTimeCompare(expected, actual) != 1 { return ErrAuth }

// Bad: timing attack
if string(expected) == string(actual) { ... }
```

## UI/UX - Cyberpunk Console

```css
:root {
  --bg-primary: #09090b;      /* Deep space gray */
  --accent-primary: #00ff9d;  /* Neon green */
  --accent-error: #ff0055;    /* Cyber red */
}
```

- No scrollbars (gesture-based)
- Haptic feedback on actions
- Respect iOS safe areas
- Dynamic Macro Deck buttons

## Testing

```typescript
// Vitest + RTL
it('encrypts round-trip', async () => {
  const ciphertext = await service.encrypt(plaintext);
  expect(await service.decrypt(ciphertext)).toEqual(plaintext);
});
```

```go
// Table-driven
func TestEncrypt(t *testing.T) {
    tests := []struct{ name string; input []byte; wantErr bool }{
        {"empty", []byte{}, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

## Git

- Branches: `feat/`, `fix/`, `refactor/`, `docs/`
- Commits: Conventional (`feat:`, `fix:`, `chore:`)
- PRs: <400 lines, no force push to `main`

## File Structure

```
app/src/{components,hooks,lib,app}/
relay/{cmd/relay,internal,pkg}/
agent/{cmd/agent,internal,pkg}/
```

## Key Decisions (from init.md)

- All App<->Agent traffic is E2EE; Hub only sees ciphertext
- WebSocket for all real-time (not HTTP)
- Mosh-style state sync (last-ack-ID), not TCP streaming
- Design for flaky mobile networks - always handle reconnection
