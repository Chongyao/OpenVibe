# Multi-Project Support Architecture

> Status: Design Phase | Target: Phase 2.5

## Overview

Enable OpenVibe to work with multiple project directories by dynamically managing OpenCode instances.

## Current Limitation

OpenCode binds to the directory where it was started. All sessions created through its API inherit that directory. There's no API to change the project directory after startup.

## Solution: Dynamic OpenCode Instance Management

The Agent will manage a pool of OpenCode instances, one per active project.

```
┌─────────────────────────────────────────────────────────────┐
│                        Agent                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              OpenCode Process Manager                │    │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │    │
│  │  │ Instance A  │ │ Instance B  │ │ Instance C  │    │    │
│  │  │ port:14001  │ │ port:14002  │ │ port:14003  │    │    │
│  │  │ /project/A  │ │ /project/B  │ │ /project/C  │    │    │
│  │  └─────────────┘ └─────────────┘ └─────────────┘    │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Project Scanner                         │    │
│  │  Scans configured workspace directories for projects │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Protocol Extensions

### New Message Types

#### `project.list` - List Available Projects

Request:
```json
{
  "type": "project.list",
  "id": "req-123",
  "payload": {}
}
```

Response:
```json
{
  "type": "response",
  "id": "req-123",
  "payload": {
    "projects": [
      {
        "path": "/home/user/projects/OpenVibe",
        "name": "OpenVibe",
        "status": "running",
        "port": 14001
      },
      {
        "path": "/home/user/projects/SmartQuant",
        "name": "SmartQuant", 
        "status": "stopped",
        "port": null
      }
    ]
  }
}
```

#### `project.select` - Select/Activate a Project

Request:
```json
{
  "type": "project.select",
  "id": "req-124",
  "payload": {
    "path": "/home/user/projects/OpenVibe"
  }
}
```

Response:
```json
{
  "type": "response",
  "id": "req-124",
  "payload": {
    "path": "/home/user/projects/OpenVibe",
    "name": "OpenVibe",
    "status": "running",
    "port": 14001
  }
}
```

The Agent will:
1. Start an OpenCode instance for the project if not running
2. Set this as the active project for the connection
3. Route subsequent requests to this OpenCode instance

#### `project.stop` - Stop a Project's OpenCode Instance

Request:
```json
{
  "type": "project.stop",
  "id": "req-125",
  "payload": {
    "path": "/home/user/projects/OpenVibe"
  }
}
```

## Agent Implementation

### New Packages

#### `internal/procmgr` - Process Manager

```go
// Manager manages OpenCode process lifecycle
type Manager struct {
    instances   map[string]*Instance  // path -> instance
    basePort    int                   // starting port (14001)
    mu          sync.RWMutex
}

// Instance represents a running OpenCode process
type Instance struct {
    Path      string
    Port      int
    Process   *os.Process
    Client    *opencode.Client
    StartedAt time.Time
    LastUsed  time.Time
}

// GetOrStart returns an existing instance or starts a new one
func (m *Manager) GetOrStart(ctx context.Context, path string) (*Instance, error)

// Stop stops an instance
func (m *Manager) Stop(path string) error

// StopAll stops all instances (for graceful shutdown)
func (m *Manager) StopAll() error

// Cleanup stops idle instances (not used for > 30 minutes)
func (m *Manager) Cleanup() error
```

#### `internal/project` - Project Scanner

```go
// Scanner scans directories for projects
type Scanner struct {
    workspaces []string  // directories to scan
}

// Project represents a discovered project
type Project struct {
    Path   string `json:"path"`
    Name   string `json:"name"`
    Type   string `json:"type"`  // "git", "npm", "go", etc.
}

// Scan returns all projects in configured workspaces
func (s *Scanner) Scan() ([]Project, error)

// IsProject checks if a directory is a valid project
func (s *Scanner) IsProject(path string) bool
```

### Configuration

Agent will accept new flags:

```bash
./agent \
  --hub ws://hub:8080/agent \
  --workspace /home/user/projects \
  --workspace /home/user/work \
  --base-port 14001 \
  --max-instances 5
```

## Hub Changes

Hub needs to:
1. Forward new message types to Agent
2. Track which project each client connection is using

Minimal changes - mostly passthrough.

## App Changes

### New UI Component: Project Selector

Location: Header bar, next to connection status

States:
1. **No project selected**: Show "Select Project" button
2. **Project selected**: Show project name with dropdown to change
3. **Loading**: Show spinner when switching projects

### User Flow

1. User opens app → sees "Select Project" prompt
2. User taps → sees list of available projects
3. User selects project → loading indicator
4. Agent starts OpenCode if needed → project active
5. User can now create sessions in that project

### Storage

- Current project stored in localStorage per agent
- Session list filtered by current project

## File Structure Changes

```
agent/
  cmd/agent/
    main.go              # Add new flags
  internal/
    opencode/
      client.go          # Existing (unchanged)
    procmgr/             # NEW
      manager.go         # Process lifecycle
      instance.go        # Single instance wrapper
    project/             # NEW  
      scanner.go         # Directory scanner
    tunnel/
      client.go          # Add project.* handlers

hub/
  internal/
    server/
      server.go          # Add project.* message forwarding

app/
  src/
    components/
      ProjectSelector.tsx  # NEW
    hooks/
      useProject.ts        # NEW - project state management
    types/
      index.ts             # Add Project type
```

## Implementation Order

1. **Agent: Process Manager** - Core functionality
2. **Agent: Project Scanner** - List projects
3. **Agent: Tunnel handlers** - Handle new message types
4. **Hub: Message forwarding** - Passthrough
5. **App: Project Selector UI** - User interface
6. **Integration testing**

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Port conflicts | Use dynamic port allocation, check availability |
| Resource exhaustion | Limit max instances, auto-cleanup idle ones |
| Orphan processes | Track PIDs, cleanup on agent shutdown |
| Slow project switching | Pre-warm instances, show loading state |

## Success Criteria

- [ ] User can list projects from configured workspaces
- [ ] User can select a project from the app
- [ ] Agent starts OpenCode instance for selected project
- [ ] Sessions are created in the correct project directory
- [ ] Idle instances are cleaned up after 30 minutes
- [ ] Graceful shutdown stops all instances
