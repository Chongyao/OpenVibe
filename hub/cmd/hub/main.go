package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/openvibe/hub/internal/config"
	"github.com/openvibe/hub/internal/proxy"
	"github.com/openvibe/hub/internal/server"
)

func main() {
	// Parse command line flags
	port := flag.String("port", "8080", "Port to listen on")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "OpenCode server URL")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_TOKEN env)")
	flag.Parse()

	// Load configuration
	cfg := config.New()
	cfg.Port = *port
	cfg.OpenCodeURL = *opencodeURL

	// Token from flag or environment
	if *token != "" {
		cfg.Token = *token
	} else if envToken := os.Getenv("OPENVIBE_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	if cfg.Token == "" {
		log.Println("WARNING: No authentication token set. Use --token or OPENVIBE_TOKEN env var.")
	}

	// Create OpenCode proxy
	opencodeProxy := proxy.NewOpenCodeProxy(cfg.OpenCodeURL)

	// Create WebSocket server
	wsServer := server.NewServer(cfg, opencodeProxy)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.HandleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start server
	addr := "0.0.0.0:" + cfg.Port
	log.Printf("OpenVibe Hub starting on %s", addr)
	log.Printf("OpenCode backend: %s", cfg.OpenCodeURL)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
