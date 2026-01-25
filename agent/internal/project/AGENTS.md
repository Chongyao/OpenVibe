# AGENTS.md - Multi-Project Manager

Manages multiple OpenCode instances via tmux sessions and port allocation.

## Overview

Enables Agent to route requests to different OpenCode instances based on project path.

```
Agent receives request with projectPath
  -> Manager.GetOpenCodeURL(path)
  -> Returns http://localhost:{allocated_port}
  -> Agent forwards to correct OpenCode instance
```

## Key Types

| Type | Purpose |
|------|---------|
| `Manager` | Orchestrates instances, port pool, tmux |
| `Instance` | Single project state (path, port, status) |
| `PortPool` | Allocates/releases ports from range |
| `TmuxExecutor` | Spawns/kills tmux sessions |

## Instance Status Flow

```
stopped -> starting -> running
              |
              v
            error
```

## Configuration

```go
&project.Config{
    AllowedPaths: []string{"/home/user/proj1", "/home/user/proj2"},
    PortMin:      4096,  // Default
    PortMax:      4105,  // Default
    MaxInstances: 5,     // Default
}
```

## Key Functions

### Manager.Start(ctx, path)

1. Validate path in whitelist
2. Check if already running (return existing)
3. Check max instances limit
4. Acquire port from pool
5. Start tmux session with `opencode serve --port {port}`
6. Wait for health check (30s timeout)
7. Set status to `running`

### Manager.GetOpenCodeURL(path)

Returns `http://localhost:{port}` for running instance.
Errors if not found or not running.

### Manager.RefreshStatus(ctx)

Syncs internal state with actual tmux sessions.
Releases ports for crashed instances.

## Tmux Session Naming

```go
const TmuxSessionPrefix = "ov-"
// Project at /home/user/MyProject -> tmux session "ov-MyProject"
```

## Port Pool

- Thread-safe allocation via mutex
- Tracks path -> port mapping
- `Acquire(path)` - Allocates next available port
- `Release(port)` - Returns port to pool

## Anti-Patterns

| Pattern | Why Bad |
|---------|---------|
| Starting without health check | OpenCode may not be ready |
| Not releasing ports on error | Port exhaustion |
| Trusting tmux session exists | May have crashed |
