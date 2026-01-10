package procmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/openvibe/agent/internal/opencode"
)

type InstanceStatus string

const (
	StatusStarting InstanceStatus = "starting"
	StatusRunning  InstanceStatus = "running"
	StatusStopping InstanceStatus = "stopping"
	StatusStopped  InstanceStatus = "stopped"
	StatusError    InstanceStatus = "error"
)

type Instance struct {
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	Port      int            `json:"port"`
	Status    InstanceStatus `json:"status"`
	StartedAt time.Time      `json:"startedAt,omitempty"`
	LastUsed  time.Time      `json:"lastUsed,omitempty"`
	Error     string         `json:"error,omitempty"`

	process *os.Process
	client  *opencode.Client
	mu      sync.RWMutex
}

func NewInstance(path string, name string, port int) *Instance {
	return &Instance{
		Path:   path,
		Name:   name,
		Port:   port,
		Status: StatusStopped,
	}
}

func (i *Instance) Start(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.Status == StatusRunning {
		return nil
	}

	i.Status = StatusStarting

	cmd := exec.CommandContext(ctx, "opencode", "serve", "--port", fmt.Sprintf("%d", i.Port))
	cmd.Dir = i.Path
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		i.Status = StatusError
		i.Error = err.Error()
		return fmt.Errorf("failed to start opencode: %w", err)
	}

	i.process = cmd.Process
	i.StartedAt = time.Now()
	i.LastUsed = time.Now()

	go func() {
		cmd.Wait()
		i.mu.Lock()
		if i.Status != StatusStopping {
			i.Status = StatusStopped
		}
		i.mu.Unlock()
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", i.Port)
	i.client = opencode.NewClient(baseURL)

	if err := i.waitForReady(ctx); err != nil {
		i.Stop()
		return err
	}

	i.Status = StatusRunning
	return nil
}

func (i *Instance) waitForReady(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for opencode to start")
			}

			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/global/health", i.Port))
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func (i *Instance) Stop() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.process == nil {
		i.Status = StatusStopped
		return nil
	}

	i.Status = StatusStopping
	if err := i.process.Signal(os.Interrupt); err != nil {
		i.process.Kill()
	}

	done := make(chan struct{})
	go func() {
		i.process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		i.process.Kill()
	}

	i.process = nil
	i.client = nil
	i.Status = StatusStopped
	return nil
}

func (i *Instance) Client() *opencode.Client {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.client
}

func (i *Instance) Touch() {
	i.mu.Lock()
	i.LastUsed = time.Now()
	i.mu.Unlock()
}

func (i *Instance) GetStatus() InstanceStatus {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.Status
}

func (i *Instance) HandleRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	i.Touch()

	client := i.Client()
	if client == nil {
		return nil, fmt.Errorf("instance not running")
	}

	return client.HandleRequest(ctx, sessionID, action, data)
}
