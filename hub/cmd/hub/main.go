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

	"github.com/openvibe/hub/internal/config"
	"github.com/openvibe/hub/internal/proxy"
	"github.com/openvibe/hub/internal/server"
)

func main() {
	port := flag.String("port", "8080", "Port to listen on")
	opencodeURL := flag.String("opencode", "http://localhost:4096", "OpenCode server URL")
	token := flag.String("token", "", "Authentication token (or use OPENVIBE_TOKEN env)")
	staticDir := flag.String("static", "", "Static files directory (Next.js out)")
	flag.Parse()

	cfg := config.New()
	cfg.Port = *port
	cfg.OpenCodeURL = *opencodeURL

	if *token != "" {
		cfg.Token = *token
	} else if envToken := os.Getenv("OPENVIBE_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}

	if cfg.Token == "" {
		log.Println("WARNING: No authentication token set. Use --token or OPENVIBE_TOKEN env var.")
	}

	opencodeProxy := proxy.NewOpenCodeProxy(cfg.OpenCodeURL)
	wsServer := server.NewServer(cfg, opencodeProxy)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.HandleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	if *staticDir != "" {
		log.Printf("Serving static files from: %s", *staticDir)
		staticRoot, err := filepath.Abs(*staticDir)
		if err != nil {
			log.Fatalf("Invalid static directory: %v", err)
		}

		fs := http.FileServer(http.Dir(staticRoot))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/health") {
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
