package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/openvibe/agent/internal/handler"
	"github.com/openvibe/agent/internal/procmgr"
	"github.com/openvibe/agent/internal/tunnel"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	hubURL := flag.String("hub", "ws://localhost:8080/agent", "Hub WebSocket URL")
	agentID := flag.String("id", "", "Agent ID (defaults to hostname)")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_AGENT_TOKEN env)")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "Legacy OpenCode server URL (fallback)")

	var workspaces stringSlice
	flag.Var(&workspaces, "workspace", "Workspace directories to scan for projects (can be specified multiple times)")

	basePort := flag.Int("base-port", 14001, "Base port for OpenCode instances")
	maxInstances := flag.Int("max-instances", 5, "Maximum concurrent OpenCode instances")

	flag.Parse()

	id := *agentID
	if id == "" {
		hostname, _ := os.Hostname()
		id = hostname
	}

	authToken := *token
	if authToken == "" {
		authToken = os.Getenv("OPENVIBE_AGENT_TOKEN")
	}

	if len(workspaces) == 0 {
		homeDir, _ := os.UserHomeDir()
		defaultWorkspace := homeDir + "/workspace/projects"
		if _, err := os.Stat(defaultWorkspace); err == nil {
			workspaces = []string{defaultWorkspace}
		}
	}

	log.Printf("OpenVibe Agent starting")
	log.Printf("  Agent ID: %s", id)
	log.Printf("  Hub URL: %s", *hubURL)
	log.Printf("  Legacy OpenCode URL: %s", *opencodeURL)
	log.Printf("  Workspaces: %v", workspaces)
	log.Printf("  Base port: %d, Max instances: %d", *basePort, *maxInstances)

	h := handler.New(&handler.Config{
		Workspaces: workspaces,
		LegacyURL:  *opencodeURL,
		ProcMgrCfg: &procmgr.Config{
			BasePort:     *basePort,
			MaxInstances: *maxInstances,
		},
	})

	client := tunnel.NewClient(*hubURL, id, authToken, h)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.StartCleanupLoop(ctx)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		h.Shutdown()
		cancel()
	}()

	if err := client.Run(ctx); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}
