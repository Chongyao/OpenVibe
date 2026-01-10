package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/openvibe/agent/internal/opencode"
	"github.com/openvibe/agent/internal/tunnel"
)

func main() {
	hubURL := flag.String("hub", "ws://localhost:8080/agent", "Hub WebSocket URL")
	agentID := flag.String("id", "", "Agent ID (defaults to hostname)")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_AGENT_TOKEN env)")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "OpenCode server URL")
	flag.Parse()

	// Get agent ID
	id := *agentID
	if id == "" {
		hostname, _ := os.Hostname()
		id = hostname
	}

	// Get token
	authToken := *token
	if authToken == "" {
		authToken = os.Getenv("OPENVIBE_AGENT_TOKEN")
	}

	log.Printf("OpenVibe Agent starting")
	log.Printf("  Agent ID: %s", id)
	log.Printf("  Hub URL: %s", *hubURL)
	log.Printf("  OpenCode URL: %s", *opencodeURL)

	// Create OpenCode client
	ocClient := opencode.NewClient(*opencodeURL)

	// Create tunnel client
	client := tunnel.NewClient(*hubURL, id, authToken, ocClient)

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Connect and run
	if err := client.Run(ctx); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}
