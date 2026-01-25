package project

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const DockerContainerPrefix = "openvibe-opencode-"

type DockerExecutor struct {
	httpClient *http.Client
	imageName  string
}

func NewDockerExecutor(imageName string) *DockerExecutor {
	if imageName == "" {
		imageName = "openvibe/opencode:latest"
	}
	return &DockerExecutor{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		imageName:  imageName,
	}
}

func (d *DockerExecutor) StartContainer(ctx context.Context, containerName, workdir string, port int) error {
	// Check if container already exists
	if d.ContainerExists(ctx, containerName) {
		// Try to start it if stopped
		startCmd := exec.CommandContext(ctx, "docker", "start", containerName)
		if err := startCmd.Run(); err == nil {
			return nil
		}
		// If start failed, remove and recreate
		d.StopContainer(ctx, containerName)
	}

	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",
		"--network", "host",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:/project", workdir),
		"-w", "/project",
		d.imageName,
		"opencode", "serve", "--port", fmt.Sprintf("%d", port),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start docker container: %w, output: %s", err, string(output))
	}

	return nil
}

func (d *DockerExecutor) StopContainer(ctx context.Context, containerName string) error {
	// Stop the container
	stopCmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	stopCmd.Run() // Ignore error, container might not be running

	// Remove the container
	rmCmd := exec.CommandContext(ctx, "docker", "rm", containerName)
	output, err := rmCmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "No such container") {
			return nil
		}
		return fmt.Errorf("failed to remove docker container: %w, output: %s", err, outputStr)
	}

	return nil
}

func (d *DockerExecutor) ContainerExists(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "-q", "-f", fmt.Sprintf("name=^%s$", containerName))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func (d *DockerExecutor) ContainerRunning(ctx context.Context, containerName string) bool {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-q", "-f", fmt.Sprintf("name=^%s$", containerName))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func (d *DockerExecutor) ListContainers(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--filter", fmt.Sprintf("name=%s", DockerContainerPrefix),
		"--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list docker containers: %w", err)
	}

	var containers []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, DockerContainerPrefix) {
			containers = append(containers, line)
		}
	}
	return containers, nil
}

func (d *DockerExecutor) IsPortInUse(ctx context.Context, port int) bool {
	url := fmt.Sprintf("http://localhost:%d/global/health", port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (d *DockerExecutor) WaitForHealth(ctx context.Context, port int, timeout time.Duration) error {
	healthURL := fmt.Sprintf("http://localhost:%d/global/health", port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("opencode health check timeout after %v", timeout)
}

func (d *DockerExecutor) GetContainerLogs(ctx context.Context, containerName string, tail int) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", tail), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	return string(output), nil
}
