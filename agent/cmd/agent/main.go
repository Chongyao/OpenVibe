package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/openvibe/agent/internal/opencode"
	"github.com/openvibe/agent/internal/project"
	"github.com/openvibe/agent/internal/tunnel"
)

func main() {
	hubURL := flag.String("hub", "ws://localhost:8080/agent", "Hub WebSocket URL")
	agentID := flag.String("id", "", "Agent ID (defaults to hostname)")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_AGENT_TOKEN env)")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "OpenCode server URL (default for single-project mode)")

	projectsFlag := flag.String("projects", "", "Comma-separated list of allowed project paths (or use OPENVIBE_PROJECTS env)")
	portMin := flag.Int("port-min", 4096, "Minimum port for OpenCode instances")
	portMax := flag.Int("port-max", 4105, "Maximum port for OpenCode instances")
	maxInstances := flag.Int("max-instances", 5, "Maximum concurrent OpenCode instances")
	dockerImage := flag.String("docker-image", "openvibe/opencode:latest", "Docker image for OpenCode containers")

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

	projects := *projectsFlag
	if projects == "" {
		projects = os.Getenv("OPENVIBE_PROJECTS")
	}

	log.Printf("OpenVibe Agent starting")
	log.Printf("  Agent ID: %s", id)
	log.Printf("  Hub URL: %s", *hubURL)

	opencodeClient := opencode.NewClient(*opencodeURL)

	var projectMgr *project.Manager
	if projects != "" {
		allowedPaths := parseProjectPaths(projects)
		log.Printf("  Multi-project mode: %d projects configured", len(allowedPaths))
		for _, p := range allowedPaths {
			log.Printf("    - %s", p)
		}

		projectMgr = project.NewManager(&project.Config{
			AllowedPaths: allowedPaths,
			PortMin:      *portMin,
			PortMax:      *portMax,
			MaxInstances: *maxInstances,
			DockerImage:  *dockerImage,
		})
	} else {
		log.Printf("  Single-project mode: %s", *opencodeURL)
	}

	client := tunnel.NewClient(*hubURL, id, authToken, opencodeClient, projectMgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	if err := client.Run(ctx); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}

func parseProjectPaths(input string) []string {
	var paths []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}
