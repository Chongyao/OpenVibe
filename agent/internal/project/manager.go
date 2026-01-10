package project

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

const (
	TmuxSessionPrefix    = "ov-"
	DefaultHealthTimeout = 30 * time.Second
)

type Config struct {
	AllowedPaths []string
	PortMin      int
	PortMax      int
	MaxInstances int
}

type Manager struct {
	config    *Config
	instances map[string]*Instance
	portPool  *PortPool
	tmux      *TmuxExecutor
	mu        sync.RWMutex
}

func NewManager(cfg *Config) *Manager {
	if cfg.PortMin == 0 {
		cfg.PortMin = 4096
	}
	if cfg.PortMax == 0 {
		cfg.PortMax = 4105
	}
	if cfg.MaxInstances == 0 {
		cfg.MaxInstances = 5
	}

	m := &Manager{
		config:    cfg,
		instances: make(map[string]*Instance),
		portPool:  NewPortPool(cfg.PortMin, cfg.PortMax),
		tmux:      NewTmuxExecutor(),
	}

	for _, path := range cfg.AllowedPaths {
		name := filepath.Base(path)
		m.instances[path] = &Instance{
			Path:        path,
			Name:        name,
			TmuxSession: TmuxSessionPrefix + name,
			Status:      StatusStopped,
		}
	}

	return m
}

func (m *Manager) List() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		copy := *inst
		result = append(result, &copy)
	}
	return result
}

func (m *Manager) GetByPath(path string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if inst, ok := m.instances[path]; ok {
		copy := *inst
		return &copy
	}
	return nil
}

func (m *Manager) Start(ctx context.Context, path string) (*Instance, error) {
	if err := m.validatePath(path); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[path]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", path)
	}

	if inst.Status == StatusRunning {
		copy := *inst
		return &copy, nil
	}

	runningCount := 0
	for _, i := range m.instances {
		if i.Status == StatusRunning {
			runningCount++
		}
	}
	if runningCount >= m.config.MaxInstances {
		return nil, fmt.Errorf("max instances reached (%d), stop another project first", m.config.MaxInstances)
	}

	port, err := m.portPool.Acquire(path)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire port: %w", err)
	}

	inst.Status = StatusStarting
	inst.Port = port
	inst.Error = ""

	if err := m.tmux.StartSession(ctx, inst.TmuxSession, path, port); err != nil {
		inst.Status = StatusError
		inst.Error = err.Error()
		m.portPool.Release(port)
		copy := *inst
		return &copy, err
	}

	if err := m.tmux.WaitForHealth(ctx, port, DefaultHealthTimeout); err != nil {
		inst.Status = StatusError
		inst.Error = err.Error()
		copy := *inst
		return &copy, err
	}

	inst.Status = StatusRunning
	inst.StartedAt = time.Now()
	copy := *inst
	return &copy, nil
}

func (m *Manager) Stop(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[path]
	if !ok {
		return fmt.Errorf("project not found: %s", path)
	}

	if inst.Status == StatusStopped {
		return nil
	}

	if err := m.tmux.StopSession(ctx, inst.TmuxSession); err != nil {
		return err
	}

	if inst.Port > 0 {
		m.portPool.Release(inst.Port)
	}

	inst.Status = StatusStopped
	inst.Port = 0
	inst.Error = ""
	inst.StartedAt = time.Time{}

	return nil
}

func (m *Manager) GetOpenCodeURL(path string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, ok := m.instances[path]
	if !ok {
		return "", fmt.Errorf("project not found: %s", path)
	}

	if inst.Status != StatusRunning {
		return "", fmt.Errorf("project not running: %s (status: %s)", path, inst.Status)
	}

	return inst.OpenCodeURL(), nil
}

func (m *Manager) RefreshStatus(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		if inst.Status == StatusRunning || inst.Status == StatusStarting {
			if !m.tmux.SessionExists(ctx, inst.TmuxSession) {
				if inst.Port > 0 {
					m.portPool.Release(inst.Port)
				}
				inst.Status = StatusStopped
				inst.Port = 0
				inst.Error = ""
				inst.StartedAt = time.Time{}
			}
		}
	}
}

func (m *Manager) validatePath(path string) error {
	for _, allowed := range m.config.AllowedPaths {
		if path == allowed {
			return nil
		}
	}
	return fmt.Errorf("path not in whitelist: %s", path)
}

func (m *Manager) SyncWithTmux(ctx context.Context) error {
	sessions, err := m.tmux.ListSessions(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range sessions {
		for _, inst := range m.instances {
			if inst.TmuxSession == session {
				inst.Status = StatusRunning
				inst.StartedAt = time.Now()
				break
			}
		}
	}

	return nil
}
