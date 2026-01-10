package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/openvibe/hub/internal/buffer"
	"github.com/openvibe/hub/internal/config"
	"github.com/openvibe/hub/internal/proxy"
	"github.com/openvibe/hub/internal/server"
	"github.com/openvibe/hub/internal/tunnel"
)

func main() {
	port := flag.String("port", "8080", "Port to listen on")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "OpenCode server URL")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_TOKEN env)")
	staticDir := flag.String("static", "", "Static files directory (Next.js out)")

	// Phase 2 flags
	agentToken := flag.String("agent-token", "", "Agent authentication token (or use OPENVIBE_AGENT_TOKEN env)")
	redisAddr := flag.String("redis", "", "Redis address (e.g., localhost:6379)")
	redisPass := flag.String("redis-pass", "", "Redis password (or use REDIS_PASSWORD env)")
	redisDB := flag.Int("redis-db", 0, "Redis database number")

	flag.Parse()

	cfg := config.New()
	cfg.Port = *port
	cfg.OpenCodeURL = *opencodeURL

	// Token configuration
	if *token != "" {
		cfg.Token = *token
	} else if envToken := os.Getenv("OPENVIBE_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	// Agent token configuration
	if *agentToken != "" {
		cfg.AgentToken = *agentToken
	} else if envToken := os.Getenv("OPENVIBE_AGENT_TOKEN"); envToken != "" {
		cfg.AgentToken = envToken
	}

	// Redis configuration
	cfg.RedisAddr = *redisAddr
	if *redisPass != "" {
		cfg.RedisPass = *redisPass
	} else if envPass := os.Getenv("REDIS_PASSWORD"); envPass != "" {
		cfg.RedisPass = envPass
	}
	cfg.RedisDB = *redisDB

	if cfg.Token == "" {
		log.Println("WARNING: No authentication token set. Use --token or OPENVIBE_TOKEN env var.")
	}

	// Initialize buffer (Redis or Noop)
	var msgBuffer buffer.Buffer
	if cfg.RedisAddr != "" {
		log.Printf("Connecting to Redis: %s", cfg.RedisAddr)
		rb, err := buffer.NewRedisBuffer(buffer.RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPass,
			DB:       cfg.RedisDB,
		})
		if err != nil {
			log.Printf("WARNING: Redis connection failed: %v, running without message buffer", err)
			msgBuffer = buffer.NewNoopBuffer()
		} else {
			log.Printf("Redis connected successfully")
			msgBuffer = rb
		}
	} else {
		log.Println("Running without Redis (no message buffering)")
		msgBuffer = buffer.NewNoopBuffer()
	}
	defer msgBuffer.Close()

	// Initialize tunnel manager
	tunnelMgr := tunnel.NewManager(&tunnel.Config{
		AgentToken: cfg.AgentToken,
	})

	// Initialize OpenCode proxy (fallback for direct mode)
	opencodeProxy := proxy.NewOpenCodeProxy(cfg.OpenCodeURL)

	// Initialize server
	wsServer := server.NewServer(cfg, opencodeProxy, msgBuffer, tunnelMgr)

	mux := http.NewServeMux()

	// WebSocket endpoints
	mux.HandleFunc("/ws", wsServer.HandleWebSocket)
	mux.HandleFunc("/agent", tunnelMgr.HandleAgentWebSocket)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Agents endpoint (list connected agents)
	mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := tunnelMgr.ListAgents()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if len(agents) == 0 {
			w.Write([]byte(`{"agents":[]}`))
		} else {
			w.Write([]byte(`{"agents":["` + strings.Join(agents, `","`) + `"]}`))
		}
	})

	if *staticDir != "" {
		log.Printf("Serving static files from: %s", *staticDir)
		staticRoot, err := filepath.Abs(*staticDir)
		if err != nil {
			log.Fatalf("Invalid static directory: %v", err)
		}

		fs := http.FileServer(http.Dir(staticRoot))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/ws") ||
				strings.HasPrefix(r.URL.Path, "/agent") ||
				strings.HasPrefix(r.URL.Path, "/health") ||
				strings.HasPrefix(r.URL.Path, "/agents") {
				return
			}

			requestPath := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(requestPath, "..") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if requestPath == "/" || requestPath == "." {
				requestPath = "/index.html"
			}

			fullPath := filepath.Join(staticRoot, requestPath)
			resolvedPath, err := filepath.Abs(fullPath)
			if err != nil || !strings.HasPrefix(resolvedPath, staticRoot) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(staticRoot, "index.html"))
				return
			}

			fs.ServeHTTP(w, r)
		})
	}

	addr := "0.0.0.0:" + cfg.Port
	log.Printf("OpenVibe Hub starting on %s", addr)
	log.Printf("OpenCode backend: %s", cfg.OpenCodeURL)
	if cfg.AgentToken != "" {
		log.Printf("Agent authentication: enabled")
	}
	if *staticDir != "" {
		log.Printf("Static files: %s", *staticDir)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

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
