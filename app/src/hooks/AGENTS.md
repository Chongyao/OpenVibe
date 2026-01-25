# AGENTS.md - React Hooks

Custom hooks for WebSocket, state management, and session handling.

## Overview

| Hook | Purpose | Key State |
|------|---------|-----------|
| `useWebSocket` | WebSocket connection + Mosh-style sync | `state`, `send`, `connect` |
| `useMessageHandler` | Message routing + streaming assembly | internal dispatch |
| `useSessionStorage` | Session persistence (localStorage) | sessions, messages |
| `useSettings` | App settings context | theme, serverUrl |
| `useProjects` | Project list + start/stop | projects, statuses |

## Critical: Stale Closure Gotcha

**NEVER rely on React state inside WebSocket callbacks.**

```typescript
// BAD: state is stale inside onConnect
const { state } = useWebSocket({
  onConnect: () => {
    if (state === 'connected') doSomething(); // state is 'disconnected' here!
  }
});

// GOOD: use refs or trust the callback context
const { send } = useWebSocket({
  onConnect: () => {
    // onConnect only fires when connected - no state check needed
    if (!isCreatingRef.current) {
      isCreatingRef.current = true;
      send({ type: 'session.create', ... });
    }
  }
});
```

## useWebSocket

Core connection hook with auto-reconnect and Mosh-style sync.

### Key Implementation Details

- **Reconnect delays**: `[1000, 2000, 5000, 10000, 30000]` ms
- **Auto-sync on reconnect**: Sends `sync` request with `lastAckId`
- **Message buffering**: Tracks `lastAckIDRef`, sends `ack` for each msgId
- **Callback refs**: Uses `callbacksRef` to avoid stale closures

### Return Value

```typescript
{ 
  state: ConnectionState,  // 'connecting' | 'connected' | 'disconnected' | 'error'
  send: (msg: ClientMessage, handler?) => boolean,
  connect: () => void,     // Manual reconnect
  disconnect: () => void 
}
```

### Message Handler Pattern

```typescript
// Request with response handler
send({ type: 'session.create', id, payload }, (response) => {
  // Called when response.id matches request.id
  // Deleted after non-stream response
});
```

## useMessageHandler

Assembles streaming responses into complete messages.

- Handles `stream`, `stream.end`, `response`, `error` types
- Maintains pending message map by request ID
- Calls `onMessage` with assembled content

## useSessionStorage

LocalStorage persistence for sessions.

- `sessions`: Session metadata (id, title, createdAt)
- `getMessages(sessionId)`: Lazy-load messages
- `saveMessage(sessionId, message)`: Append to session
- `deleteSession(sessionId)`: Remove session + messages

## useSettings

Theme and server configuration context.

```typescript
const { settings, updateSettings } = useSettings();
// settings.theme: 'dark' | 'light' | 'system'
// settings.serverUrl: string
```

## useProjects

Multi-project mode management.

- `projects`: List of available projects
- `startProject(path)`: Start OpenCode instance
- `stopProject(path)`: Stop instance
- `refreshProjects()`: Fetch current state

### sendRef Null Check

```typescript
// sendRef may be null on first render
send: (msg) => {
  const result = sendRef.current?.({ ...msg } as ClientMessage);
  if (result === undefined) {
    console.warn('[useProjects] WebSocket not ready');
  }
}
```

## Anti-Patterns

| Pattern | Why Bad | Fix |
|---------|---------|-----|
| State in callbacks | Stale closure | Use refs |
| Multiple `in_progress` | Race conditions | Single active task |
| Missing cleanup | Memory leaks | Return cleanup in useEffect |
