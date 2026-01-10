package project

import "time"

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

type Instance struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Port        int       `json:"port"`
	TmuxSession string    `json:"tmuxSession"`
	Status      Status    `json:"status"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
}

func (i *Instance) IsRunning() bool {
	return i.Status == StatusRunning
}

func (i *Instance) OpenCodeURL() string {
	if i.Port == 0 {
		return ""
	}
	return "http://localhost:" + itoa(i.Port)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}
