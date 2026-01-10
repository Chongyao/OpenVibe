package procmgr

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	BasePort     int
	MaxInstances int
	IdleTimeout  time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		BasePort:     14001,
		MaxInstances: 5,
		IdleTimeout:  30 * time.Minute,
	}
}

type Manager struct {
	config    *Config
	instances map[string]*Instance
	mu        sync.RWMutex
	nextPort  int
}

func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Manager{
		config:    cfg,
		instances: make(map[string]*Instance),
		nextPort:  cfg.BasePort,
	}
}

func (m *Manager) GetOrStart(ctx context.Context, path string) (*Instance, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[absPath]; ok {
		if inst.GetStatus() == StatusRunning {
			inst.Touch()
			return inst, nil
		}
	}

	if len(m.instances) >= m.config.MaxInstances {
		if err := m.cleanupOldestLocked(); err != nil {
			return nil, fmt.Errorf("max instances reached and cleanup failed: %w", err)
		}
	}

	port := m.allocatePortLocked()
	name := filepath.Base(absPath)
	inst := NewInstance(absPath, name, port)

	if err := inst.Start(ctx); err != nil {
		return nil, err
	}

	m.instances[absPath] = inst
	return inst, nil
}

func (m *Manager) Get(path string) (*Instance, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, ok := m.instances[absPath]
	if !ok || inst.GetStatus() != StatusRunning {
		return nil, false
	}
	return inst, true
}

func (m *Manager) Stop(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[absPath]
	if !ok {
		return nil
	}

	if err := inst.Stop(); err != nil {
		return err
	}

	delete(m.instances, absPath)
	return nil
}

func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for path, inst := range m.instances {
		if err := inst.Stop(); err != nil {
			lastErr = err
		}
		delete(m.instances, path)
	}
	return lastErr
}

func (m *Manager) List() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		result = append(result, inst)
	}
	return result
}

func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for path, inst := range m.instances {
		inst.mu.RLock()
		idle := now.Sub(inst.LastUsed) > m.config.IdleTimeout
		inst.mu.RUnlock()

		if idle {
			toRemove = append(toRemove, path)
		}
	}

	var lastErr error
	for _, path := range toRemove {
		if err := m.instances[path].Stop(); err != nil {
			lastErr = err
		}
		delete(m.instances, path)
	}

	return lastErr
}

func (m *Manager) StartCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Cleanup()
		}
	}
}

func (m *Manager) allocatePortLocked() int {
	port := m.nextPort
	m.nextPort++
	return port
}

func (m *Manager) cleanupOldestLocked() error {
	var oldest *Instance
	var oldestPath string

	for path, inst := range m.instances {
		inst.mu.RLock()
		if oldest == nil || inst.LastUsed.Before(oldest.LastUsed) {
			oldest = inst
			oldestPath = path
		}
		inst.mu.RUnlock()
	}

	if oldest == nil {
		return fmt.Errorf("no instances to cleanup")
	}

	if err := oldest.Stop(); err != nil {
		return err
	}

	delete(m.instances, oldestPath)
	return nil
}
