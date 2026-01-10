package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/openvibe/agent/internal/opencode"
	"github.com/openvibe/agent/internal/procmgr"
	"github.com/openvibe/agent/internal/project"
)

type Handler struct {
	procMgr        *procmgr.Manager
	scanner        *project.Scanner
	legacyClient   *opencode.Client
	activeProjects map[string]string
	mu             sync.RWMutex
}

type Config struct {
	Workspaces []string
	LegacyURL  string
	ProcMgrCfg *procmgr.Config
}

func New(cfg *Config) *Handler {
	var legacyClient *opencode.Client
	if cfg.LegacyURL != "" {
		legacyClient = opencode.NewClient(cfg.LegacyURL)
	}

	return &Handler{
		procMgr:        procmgr.NewManager(cfg.ProcMgrCfg),
		scanner:        project.NewScanner(cfg.Workspaces),
		legacyClient:   legacyClient,
		activeProjects: make(map[string]string),
	}
}

func (h *Handler) HandleRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	switch action {
	case "project.list":
		return h.handleProjectList(ctx)
	case "project.select":
		return h.handleProjectSelect(ctx, data)
	case "project.stop":
		return h.handleProjectStop(ctx, data)
	case "project.status":
		return h.handleProjectStatus(ctx)
	default:
		return h.handleOpenCodeRequest(ctx, sessionID, action, data)
	}
}

func (h *Handler) handleProjectList(ctx context.Context) (<-chan []byte, error) {
	ch := make(chan []byte, 1)

	go func() {
		defer close(ch)

		projects, err := h.scanner.Scan()
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			ch <- errPayload
			return
		}

		type projectWithStatus struct {
			Path   string                 `json:"path"`
			Name   string                 `json:"name"`
			Type   project.ProjectType    `json:"type"`
			Status procmgr.InstanceStatus `json:"status"`
			Port   *int                   `json:"port,omitempty"`
		}

		result := make([]projectWithStatus, 0, len(projects))
		for _, p := range projects {
			ps := projectWithStatus{
				Path: p.Path,
				Name: p.Name,
				Type: p.Type,
			}

			if inst, ok := h.procMgr.Get(p.Path); ok {
				ps.Status = inst.GetStatus()
				port := inst.Port
				ps.Port = &port
			} else {
				ps.Status = procmgr.StatusStopped
			}

			result = append(result, ps)
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"projects": result,
		})
		ch <- payload
	}()

	return ch, nil
}

func (h *Handler) handleProjectSelect(ctx context.Context, data json.RawMessage) (<-chan []byte, error) {
	ch := make(chan []byte, 1)

	go func() {
		defer close(ch)

		var req struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": "invalid request"})
			ch <- errPayload
			return
		}

		if err := h.scanner.ValidatePath(req.Path); err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("invalid path: %v", err)})
			ch <- errPayload
			return
		}

		inst, err := h.procMgr.GetOrStart(ctx, req.Path)
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			ch <- errPayload
			return
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"path":   inst.Path,
			"name":   inst.Name,
			"status": inst.GetStatus(),
			"port":   inst.Port,
		})
		ch <- payload
	}()

	return ch, nil
}

func (h *Handler) handleProjectStop(ctx context.Context, data json.RawMessage) (<-chan []byte, error) {
	ch := make(chan []byte, 1)

	go func() {
		defer close(ch)

		var req struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": "invalid request"})
			ch <- errPayload
			return
		}

		if err := h.procMgr.Stop(req.Path); err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			ch <- errPayload
			return
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"success": true,
			"path":    req.Path,
		})
		ch <- payload
	}()

	return ch, nil
}

func (h *Handler) handleProjectStatus(ctx context.Context) (<-chan []byte, error) {
	ch := make(chan []byte, 1)

	go func() {
		defer close(ch)

		instances := h.procMgr.List()

		type instanceInfo struct {
			Path   string                 `json:"path"`
			Name   string                 `json:"name"`
			Port   int                    `json:"port"`
			Status procmgr.InstanceStatus `json:"status"`
		}

		result := make([]instanceInfo, 0, len(instances))
		for _, inst := range instances {
			result = append(result, instanceInfo{
				Path:   inst.Path,
				Name:   inst.Name,
				Port:   inst.Port,
				Status: inst.GetStatus(),
			})
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"instances": result,
		})
		ch <- payload
	}()

	return ch, nil
}

func (h *Handler) handleOpenCodeRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	h.mu.RLock()
	activePath := h.activeProjects[sessionID]
	h.mu.RUnlock()

	if activePath != "" {
		if inst, ok := h.procMgr.Get(activePath); ok {
			return inst.HandleRequest(ctx, sessionID, action, data)
		}
	}

	if h.legacyClient != nil {
		return h.legacyClient.HandleRequest(ctx, sessionID, action, data)
	}

	ch := make(chan []byte, 1)
	errPayload, _ := json.Marshal(map[string]string{"error": "no project selected"})
	ch <- errPayload
	close(ch)
	return ch, nil
}

func (h *Handler) SetActiveProject(sessionID, projectPath string) {
	h.mu.Lock()
	h.activeProjects[sessionID] = projectPath
	h.mu.Unlock()
}

func (h *Handler) Shutdown() error {
	return h.procMgr.StopAll()
}

func (h *Handler) StartCleanupLoop(ctx context.Context) {
	h.procMgr.StartCleanupLoop(ctx)
}
