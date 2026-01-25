package project

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrNoAvailablePort = errors.New("no available port in pool")
	ErrPortNotInUse    = errors.New("port not in use")
	ErrAllPortsInUse   = errors.New("all ports in range are occupied by other services")
)

// PortChecker is an interface for checking if a port is in use
type PortChecker interface {
	IsPortInUse(ctx context.Context, port int) bool
}

type PortPool struct {
	minPort       int
	maxPort       int
	portToProject map[int]string
	mu            sync.Mutex
}

func NewPortPool(minPort, maxPort int) *PortPool {
	return &PortPool{
		minPort:       minPort,
		maxPort:       maxPort,
		portToProject: make(map[int]string),
	}
}

func (p *PortPool) Acquire(projectPath string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port, path := range p.portToProject {
		if path == projectPath {
			return port, nil
		}
	}

	for port := p.minPort; port <= p.maxPort; port++ {
		if _, ok := p.portToProject[port]; !ok {
			p.portToProject[port] = projectPath
			return port, nil
		}
	}

	return 0, ErrNoAvailablePort
}

func (p *PortPool) AcquireAvailable(ctx context.Context, projectPath string, checker PortChecker) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port, path := range p.portToProject {
		if path == projectPath {
			return port, nil
		}
	}

	for port := p.minPort; port <= p.maxPort; port++ {
		if _, ok := p.portToProject[port]; ok {
			continue
		}

		if checker.IsPortInUse(ctx, port) {
			continue
		}

		p.portToProject[port] = projectPath
		return port, nil
	}

	return 0, ErrAllPortsInUse
}

func (p *PortPool) Release(port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.portToProject[port]; !ok {
		return ErrPortNotInUse
	}

	delete(p.portToProject, port)
	return nil
}

func (p *PortPool) GetPort(projectPath string) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port, path := range p.portToProject {
		if path == projectPath {
			return port, true
		}
	}
	return 0, false
}

func (p *PortPool) UsedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.portToProject)
}

func (p *PortPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return (p.maxPort - p.minPort + 1) - len(p.portToProject)
}

func (p *PortPool) MarkInUse(port int, projectPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.portToProject[port] = projectPath
}
