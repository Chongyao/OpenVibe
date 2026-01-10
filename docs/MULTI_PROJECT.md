# Multi-Project Support Design Document

> **Status**: Implementation Phase
> **Created**: 2025-01-10
> **Author**: LLM Agent (Sisyphus)

## 1. Problem Statement

OpenCode processes are bound to a working directory at startup and cannot switch dynamically. To support multiple projects from a single mobile interface, we need to manage multiple OpenCode instances.

### Current Limitation
```
Agent ---> OpenCode (port 4096, fixed to /home/zcy/workspace/projects/OpenVibe)
```

Users cannot switch between projects without restarting the agent.

## 2. Solution Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Agent (Go)                                   │
│                                                                     │
│  ┌───────────────┐    ┌───────────────────┐    ┌─────────────────┐ │
│  │ tunnel.Client │───▶│  ProjectManager   │───▶│  tmux executor  │ │
│  │               │    │                   │    │                 │ │
│  │ project.*     │    │ instances map     │    │ start/stop      │ │
│  │ session.*     │    │ portPool          │    │ health check    │ │
│  │ prompt        │    │ allowedPaths      │    │                 │ │
│  └───────────────┘    └───────────────────┘    └─────────────────┘ │
│                               │                                     │
│                               ▼                                     │
│                    ┌─────────────────────────────────────┐         │
│                    │        OpenCode Instances           │         │
│                    │                                     │         │
│                    │  ov-OpenVibe    :4096  (running)    │         │
│                    │  ov-SmartQuant  :4097  (running)    │         │
│                    │  ov-MyProject   :4098  (stopped)    │         │
│                    └─────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Project Discovery | Pre-configured whitelist | Security: explicit control over allowed paths |
| Process Lifecycle | Manual start/stop | User preference: projects persist in background for quick access |
| Session-Project Relation | Linked by `path` field | Sessions filter by project path |
| Process Manager | tmux | Reliable, persistent, easy to debug |
| Port Range | 4096-4105 | 10 ports, sufficient for personal use |

## 3. Data Structures

### 3.1 Instance

```go
// agent/internal/project/instance.go
package project

type Status string

const (
    StatusStopped  Status = "stopped"
    StatusStarting Status = "starting"
    StatusRunning  Status = "running"
    StatusError    Status = "error"
)

type Instance struct {
    Path        string    `json:"path"`        // Full path: /home/zcy/workspace/projects/OpenVibe
    Name        string    `json:"name"`        // Display name: OpenVibe
    Port        int       `json:"port"`        // OpenCode port: 4096-4105
    TmuxSession string    `json:"tmuxSession"` // tmux session: ov-OpenVibe
    Status      Status    `json:"status"`      
    Error       string    `json:"error,omitempty"`
    StartedAt   time.Time `json:"startedAt,omitempty"`
}
```

### 3.2 PortPool

```go
// agent/internal/project/portpool.go
type PortPool struct {
    minPort  int            // 4096
    maxPort  int            // 4105
    used     map[int]string // port -> projectPath
    mu       sync.Mutex
}

func NewPortPool(min, max int) *PortPool
func (p *PortPool) Acquire(projectPath string) (int, error)
func (p *PortPool) Release(port int)
func (p *PortPool) GetPort(projectPath string) (int, bool)
```

### 3.3 ProjectManager

```go
// agent/internal/project/manager.go
type Config struct {
    AllowedPaths []string // Whitelist: ["/home/zcy/workspace/projects/OpenVibe", ...]
    PortMin      int      // 4096
    PortMax      int      // 4105
    MaxInstances int      // 5
}

type Manager struct {
    config    *Config
    instances map[string]*Instance // path -> instance
    portPool  *PortPool
    tmux      *TmuxExecutor
    mu        sync.RWMutex
}

// Core methods
func (m *Manager) List() []*Instance
func (m *Manager) Start(path string) (*Instance, error)
func (m *Manager) Stop(path string) error
func (m *Manager) GetByPath(path string) *Instance
func (m *Manager) GetOpenCodeURL(path string) (string, error)
```

## 4. Message Protocol

### 4.1 New Message Types

| Type | Direction | Payload | Description |
|------|-----------|---------|-------------|
| `project.list` | Client → Agent | `{}` | List all configured projects |
| `project.list.response` | Agent → Client | `{ projects: Project[] }` | Project list with status |
| `project.start` | Client → Agent | `{ path: string }` | Start OpenCode for project |
| `project.start.response` | Agent → Client | `{ project: Project }` | Started project info |
| `project.stop` | Client → Agent | `{ path: string }` | Stop OpenCode for project |
| `project.stop.response` | Agent → Client | `{ success: boolean }` | Stop result |

### 4.2 Modified Request Flow

For `session.*` and `prompt` actions:

```
1. Client sends: { type: 'prompt', payload: { sessionId, projectPath, content } }
2. Agent looks up OpenCode URL by projectPath
3. Agent forwards to correct OpenCode instance
4. Response streams back to client
```

## 5. Implementation Plan

### Phase 1: Agent Backend (Priority: High)

| Step | File | Description |
|------|------|-------------|
| 1 | `internal/project/instance.go` | Instance data structure |
| 2 | `internal/project/portpool.go` | Port pool management |
| 3 | `internal/project/tmux.go` | tmux command execution |
| 4 | `internal/project/manager.go` | Project manager core logic |
| 5 | `internal/opencode/client.go` | Support dynamic port |
| 6 | `internal/tunnel/client.go` | Add project.* message routing |
| 7 | `cmd/agent/main.go` | Integrate ProjectManager |

### Phase 2: Frontend (Priority: Medium)

| Step | File | Description |
|------|------|-------------|
| 1 | `types/index.ts` | Add Project type |
| 2 | `hooks/useProjects.ts` | Project management hook |
| 3 | `components/ProjectSelector.tsx` | Show status, start/stop buttons |

## 6. Security Considerations

| Risk | Mitigation |
|------|------------|
| Arbitrary path execution | Whitelist: only paths in `AllowedPaths` config |
| Resource exhaustion | Max 5 concurrent instances, port range 4096-4105 |
| Command injection | Use `exec.Command` with parameterized args, no string concatenation |
| Zombie processes | Health check + auto-cleanup unresponsive instances |

### Path Validation

```go
func (m *Manager) validatePath(path string) error {
    // Must be in allowed paths
    for _, allowed := range m.config.AllowedPaths {
        if path == allowed {
            return nil
        }
    }
    return fmt.Errorf("path not in whitelist: %s", path)
}
```

## 7. tmux Integration

### Session Naming Convention
- Pattern: `ov-{ProjectName}`
- Example: `ov-OpenVibe`, `ov-SmartQuant`

### Start Command
```bash
tmux new-session -d -s ov-OpenVibe -c /home/zcy/workspace/projects/OpenVibe \
  "opencode serve --port 4096"
```

### Stop Command
```bash
tmux kill-session -t ov-OpenVibe
```

### Health Check
```bash
# Check if session exists
tmux has-session -t ov-OpenVibe 2>/dev/null && echo "running" || echo "stopped"

# Check if OpenCode responds
curl -sf http://localhost:4096/global/health
```

## 8. Configuration

### Agent CLI Flags

```bash
./agent \
  --hub ws://hub:8080/agent \
  --token secret \
  --projects "/home/zcy/workspace/projects/OpenVibe,/home/zcy/workspace/projects/SmartQuant" \
  --port-min 4096 \
  --port-max 4105 \
  --max-instances 5
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENVIBE_PROJECTS` | Comma-separated project paths | (none) |
| `OPENVIBE_PORT_MIN` | Minimum OpenCode port | 4096 |
| `OPENVIBE_PORT_MAX` | Maximum OpenCode port | 4105 |

## 9. Error Handling

| Error | User Message | Recovery |
|-------|--------------|----------|
| Project not in whitelist | "Project not configured" | Add to whitelist |
| No available ports | "Too many projects running" | Stop unused project |
| OpenCode start timeout | "Failed to start project" | Check logs, retry |
| OpenCode health check fail | "Project unhealthy" | Restart project |

## 10. Testing Plan

### Unit Tests
- `project/portpool_test.go`: Port allocation/release
- `project/manager_test.go`: Start/stop/list logic

### Integration Tests
```bash
# Start two projects
curl -X POST http://agent/project/start -d '{"path":"/home/zcy/.../OpenVibe"}'
curl -X POST http://agent/project/start -d '{"path":"/home/zcy/.../SmartQuant"}'

# Verify both running
tmux list-sessions | grep "ov-"

# Send prompt to each
curl http://localhost:4096/global/health
curl http://localhost:4097/global/health
```

## 11. Future Enhancements

- [ ] Auto-discovery: scan directory for projects
- [ ] Idle timeout: stop projects after N minutes of inactivity
- [ ] Resource limits: memory/CPU per instance
- [ ] Project templates: quick setup for new projects
