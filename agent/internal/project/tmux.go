package project

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type TmuxExecutor struct{}

func NewTmuxExecutor() *TmuxExecutor {
	return &TmuxExecutor{}
}

func (t *TmuxExecutor) StartSession(ctx context.Context, sessionName, workdir string, port int) error {
	opencodeCmd := fmt.Sprintf("opencode serve --port %d", port)

	cmd := exec.CommandContext(ctx, "tmux", "new-session",
		"-d",
		"-s", sessionName,
		"-c", workdir,
		opencodeCmd,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start tmux session: %w, output: %s", err, string(output))
	}

	return nil
}

func (t *TmuxExecutor) StopSession(ctx context.Context, sessionName string) error {
	cmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", sessionName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "no server running") ||
			strings.Contains(string(output), "session not found") {
			return nil
		}
		return fmt.Errorf("failed to stop tmux session: %w, output: %s", err, string(output))
	}
	return nil
}

func (t *TmuxExecutor) SessionExists(ctx context.Context, sessionName string) bool {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

func (t *TmuxExecutor) ListSessions(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "no server running") {
			return nil, nil
		}
		return nil, err
	}

	var sessions []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ov-") {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

func (t *TmuxExecutor) WaitForHealth(ctx context.Context, port int, timeout time.Duration) error {
	url := fmt.Sprintf("http://localhost:%d/global/health", port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cmd := exec.CommandContext(ctx, "curl", "-sf", url)
		if cmd.Run() == nil {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("opencode health check timeout after %v", timeout)
}
