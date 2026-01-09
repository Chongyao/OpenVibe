package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openvibe/hub/internal/config"
	"github.com/openvibe/hub/internal/proxy"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024 // 1MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Server handles WebSocket connections
type Server struct {
	config  *config.Config
	proxy   *proxy.OpenCodeProxy
	clients map[*Client]bool
	mu      sync.RWMutex
}

// Client represents a connected WebSocket client
type Client struct {
	server    *Server
	conn      *websocket.Conn
	send      chan []byte
	sessionID string
}

// ClientMessage represents incoming WebSocket message
type ClientMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

// PromptPayload represents prompt message payload
type PromptPayload struct {
	SessionID string `json:"sessionId"`
	Content   string `json:"content"`
}

// SessionPayload represents session-related payload
type SessionPayload struct {
	SessionID string `json:"sessionId,omitempty"`
	Title     string `json:"title,omitempty"`
}

// ServerMessage represents outgoing WebSocket message
type ServerMessage struct {
	Type    string      `json:"type"`
	ID      string      `json:"id,omitempty"`
	Payload interface{} `json:"payload"`
}

// NewServer creates a new WebSocket server
func NewServer(cfg *config.Config, p *proxy.OpenCodeProxy) *Server {
	return &Server{
		config:  cfg,
		proxy:   p,
		clients: make(map[*Client]bool),
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Token authentication
	if s.config.Token != "" {
		token := r.URL.Query().Get("token")
		if token != s.config.Token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		server: s,
		conn:   conn,
		send:   make(chan []byte, 256),
	}

	s.mu.Lock()
	s.clients[client] = true
	s.mu.Unlock()

	log.Printf("Client connected: %s", conn.RemoteAddr())

	// Start read and write pumps
	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.server.mu.Lock()
		delete(c.server.clients, c)
		c.server.mu.Unlock()
		c.conn.Close()
		log.Printf("Client disconnected: %s", c.conn.RemoteAddr())
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError(msg.ID, "Invalid message format")
		return
	}

	switch msg.Type {
	case "ping":
		c.sendMessage(ServerMessage{Type: "pong", ID: msg.ID, Payload: nil})

	case "session.list":
		c.handleSessionList(msg.ID)

	case "session.create":
		var payload SessionPayload
		json.Unmarshal(msg.Payload, &payload)
		c.handleSessionCreate(msg.ID, payload.Title)

	case "prompt":
		var payload PromptPayload
		json.Unmarshal(msg.Payload, &payload)
		c.handlePrompt(msg.ID, payload)

	default:
		c.sendError(msg.ID, "Unknown message type: "+msg.Type)
	}
}

func (c *Client) handleSessionList(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessions, err := c.server.proxy.ListSessions(ctx)
	if err != nil {
		c.sendError(requestID, "Failed to list sessions: "+err.Error())
		return
	}

	c.sendMessage(ServerMessage{
		Type:    "response",
		ID:      requestID,
		Payload: sessions,
	})
}

func (c *Client) handleSessionCreate(requestID string, title string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := c.server.proxy.CreateSession(ctx, title)
	if err != nil {
		c.sendError(requestID, "Failed to create session: "+err.Error())
		return
	}

	c.sessionID = session.ID
	c.sendMessage(ServerMessage{
		Type:    "response",
		ID:      requestID,
		Payload: session,
	})
}

func (c *Client) handlePrompt(requestID string, payload PromptPayload) {
	sessionID := payload.SessionID
	if sessionID == "" {
		sessionID = c.sessionID
	}
	if sessionID == "" {
		c.sendError(requestID, "No session ID provided")
		return
	}

	ctx := context.Background() // No timeout for streaming

	// Stream response back to client
	err := c.server.proxy.SendMessage(ctx, sessionID, payload.Content, func(eventType string, data []byte) error {
		c.sendMessage(ServerMessage{
			Type:    "stream",
			ID:      requestID,
			Payload: json.RawMessage(data),
		})
		return nil
	})

	if err != nil {
		c.sendError(requestID, "Failed to send message: "+err.Error())
		return
	}

	// Send completion
	c.sendMessage(ServerMessage{
		Type:    "stream.end",
		ID:      requestID,
		Payload: nil,
	})
}

func (c *Client) sendMessage(msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("Client send buffer full, dropping message")
	}
}

func (c *Client) sendError(requestID string, errMsg string) {
	c.sendMessage(ServerMessage{
		Type: "error",
		ID:   requestID,
		Payload: map[string]string{
			"error": errMsg,
		},
	})
}
