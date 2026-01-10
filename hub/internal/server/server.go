package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openvibe/hub/internal/buffer"
	"github.com/openvibe/hub/internal/config"
	"github.com/openvibe/hub/internal/proxy"
	"github.com/openvibe/hub/internal/tunnel"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	sessionIDPattern = regexp.MustCompile(`^ses_[a-zA-Z0-9]+$`)
)

type Server struct {
	config    *config.Config
	proxy     *proxy.OpenCodeProxy
	buffer    buffer.Buffer
	tunnelMgr *tunnel.Manager
	clients   map[*Client]bool
	mu        sync.RWMutex
}

type Client struct {
	server    *Server
	conn      *websocket.Conn
	send      chan []byte
	sessionID string
	lastAckID int64 // For Mosh-style sync
}

type ClientMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

type PromptPayload struct {
	SessionID string `json:"sessionId"`
	Content   string `json:"content"`
}

type SessionPayload struct {
	SessionID string `json:"sessionId,omitempty"`
	Title     string `json:"title,omitempty"`
	Directory string `json:"directory,omitempty"`
}

type SyncPayload struct {
	SessionID string `json:"sessionId"`
	LastAckID int64  `json:"lastAckId"`
}

type ServerMessage struct {
	Type    string      `json:"type"`
	ID      string      `json:"id,omitempty"`
	MsgID   int64       `json:"msgId,omitempty"` // Buffer message ID
	Payload interface{} `json:"payload"`
}

func NewServer(cfg *config.Config, p *proxy.OpenCodeProxy, buf buffer.Buffer, tm *tunnel.Manager) *Server {
	return &Server{
		config:    cfg,
		proxy:     p,
		buffer:    buf,
		tunnelMgr: tm,
		clients:   make(map[*Client]bool),
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.config.Token != "" {
		token := r.URL.Query().Get("token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.config.Token)) != 1 {
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
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError(msg.ID, "Invalid payload format")
			return
		}
		c.handleSessionCreate(msg.ID, payload.Title, payload.Directory)

	case "prompt":
		var payload PromptPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError(msg.ID, "Invalid payload format")
			return
		}
		c.handlePrompt(msg.ID, payload)

	case "sync":
		var payload SyncPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError(msg.ID, "Invalid payload format")
			return
		}
		c.handleSync(msg.ID, payload)

	case "ack":
		// Client acknowledging receipt of message
		var payload struct {
			MsgID int64 `json:"msgId"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			c.lastAckID = payload.MsgID
		}

	case "session.messages":
		var payload SessionPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError(msg.ID, "Invalid payload format")
			return
		}
		c.handleSessionMessages(msg.ID, payload.SessionID)

	case "session.delete":
		var payload SessionPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			c.sendError(msg.ID, "Invalid payload format")
			return
		}
		c.handleSessionDelete(msg.ID, payload.SessionID)

	case "project.list":
		c.handleProjectList(msg.ID)

	case "project.start", "project.stop":
		c.handleProjectAction(msg.ID, msg.Type, msg.Payload)

	default:
		c.sendError(msg.ID, "Unknown message type: "+msg.Type)
	}
}

func (c *Client) handleSessionList(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		c.handleViaAgent(ctx, requestID, agent.ID, "session.list", "", nil)
		return
	}

	// Check if direct mode is available
	if err := c.server.proxy.Health(ctx); err != nil {
		c.sendError(requestID, "No agent connected and OpenCode is not available. Please start an agent or ensure OpenCode is running locally.")
		return
	}

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

func (c *Client) handleSessionCreate(requestID string, title string, directory string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		data, _ := json.Marshal(map[string]string{"title": title, "directory": directory})
		c.handleViaAgent(ctx, requestID, agent.ID, "session.create", directory, data)
		return
	}

	// Check if direct mode is available
	if err := c.server.proxy.Health(ctx); err != nil {
		c.sendError(requestID, "No agent connected. Please start the OpenVibe agent on your development server.")
		return
	}

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

func (c *Client) handleSessionMessages(requestID string, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if sessionID == "" {
		sessionID = c.sessionID
	}
	if sessionID == "" {
		c.sendError(requestID, "No session ID provided")
		return
	}

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		data, _ := json.Marshal(map[string]string{"sessionId": sessionID})
		c.handleViaAgent(ctx, requestID, agent.ID, "session.messages", "", data)
		return
	}

	c.sendError(requestID, "No agent connected")
}

func (c *Client) handleSessionDelete(requestID string, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if sessionID == "" {
		c.sendError(requestID, "No session ID provided")
		return
	}

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		data, _ := json.Marshal(map[string]string{"sessionId": sessionID})
		c.handleViaAgent(ctx, requestID, agent.ID, "session.delete", "", data)
		return
	}

	c.sendError(requestID, "No agent connected")
}

func (c *Client) handleProjectList(requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		c.handleViaAgent(ctx, requestID, agent.ID, "project.list", "", nil)
		return
	}

	c.sendError(requestID, "No agent connected. Please start the OpenVibe agent on your development server.")
}

func (c *Client) handleProjectAction(requestID string, action string, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		c.handleViaAgent(ctx, requestID, agent.ID, action, "", payload)
		return
	}

	c.sendError(requestID, "No agent connected. Please start the OpenVibe agent on your development server.")
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

	if !sessionIDPattern.MatchString(sessionID) {
		c.sendError(requestID, "Invalid session ID format")
		return
	}

	ctx := context.Background()

	// Try agent first, fallback to direct
	if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
		data, _ := json.Marshal(map[string]string{"content": payload.Content})
		c.handleViaAgentStream(ctx, requestID, agent.ID, sessionID, "prompt", data)
		return
	}

	// Direct mode (fallback)
	err := c.server.proxy.SendMessage(ctx, sessionID, payload.Content, func(eventType string, data []byte) error {
		// Buffer the message
		bufMsg := buffer.Message{
			Type:      "stream",
			RequestID: requestID,
			Payload:   data,
		}
		msgID, _ := c.server.buffer.Push(ctx, sessionID, bufMsg)

		c.sendMessage(ServerMessage{
			Type:    "stream",
			ID:      requestID,
			MsgID:   msgID,
			Payload: json.RawMessage(data),
		})
		return nil
	})

	if err != nil {
		c.sendError(requestID, "Failed to send message: "+err.Error())
		return
	}

	// Buffer and send stream end
	bufMsg := buffer.Message{
		Type:      "stream.end",
		RequestID: requestID,
	}
	msgID, _ := c.server.buffer.Push(ctx, sessionID, bufMsg)

	c.sendMessage(ServerMessage{
		Type:    "stream.end",
		ID:      requestID,
		MsgID:   msgID,
		Payload: nil,
	})
}

func (c *Client) handleSync(requestID string, payload SyncPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessionID := payload.SessionID
	if sessionID == "" {
		sessionID = c.sessionID
	}

	// Get messages since lastAckID
	messages, err := c.server.buffer.GetSince(ctx, sessionID, payload.LastAckID)
	if err != nil {
		c.sendError(requestID, "Failed to sync: "+err.Error())
		return
	}

	latestID, _ := c.server.buffer.GetLatestID(ctx, sessionID)

	c.sendMessage(ServerMessage{
		Type: "sync.batch",
		ID:   requestID,
		Payload: map[string]interface{}{
			"messages": messages,
			"latestId": latestID,
		},
	})
}

func (c *Client) handleViaAgent(ctx context.Context, requestID, agentID, action string, projectPath string, data json.RawMessage) {
	sessionID := c.sessionID
	if data != nil {
		var dataMap map[string]interface{}
		if json.Unmarshal(data, &dataMap) == nil {
			if sid, ok := dataMap["sessionId"].(string); ok && sid != "" {
				sessionID = sid
			}
		}
	}

	req := &tunnel.RequestPayload{
		SessionID:   sessionID,
		Action:      action,
		Data:        data,
		ProjectPath: projectPath,
	}

	respCh, err := c.server.tunnelMgr.Forward(ctx, agentID, requestID, req)
	if err != nil {
		c.sendError(requestID, "Agent forward failed: "+err.Error())
		return
	}

	select {
	case msg := <-respCh:
		if msg != nil {
			switch msg.Type {
			case tunnel.MsgTypeResponse:
				c.sendMessage(ServerMessage{
					Type:    "response",
					ID:      requestID,
					Payload: json.RawMessage(msg.Payload),
				})
			case tunnel.MsgTypeStream:
				c.sendMessage(ServerMessage{
					Type:    "response",
					ID:      requestID,
					Payload: json.RawMessage(msg.Payload),
				})
			case tunnel.MsgTypeError:
				c.sendMessage(ServerMessage{
					Type:    "error",
					ID:      requestID,
					Payload: json.RawMessage(msg.Payload),
				})
			default:
				c.sendMessage(ServerMessage{
					Type:    "response",
					ID:      requestID,
					Payload: json.RawMessage(msg.Payload),
				})
			}
		}
	case <-ctx.Done():
		c.sendError(requestID, "Request timeout")
	}
}

func (c *Client) handleViaAgentStream(ctx context.Context, requestID, agentID, sessionID, action string, data json.RawMessage) {
	req := &tunnel.RequestPayload{
		SessionID: sessionID,
		Action:    action,
		Data:      data,
	}

	respCh, err := c.server.tunnelMgr.Forward(ctx, agentID, requestID, req)
	if err != nil {
		c.sendError(requestID, "Agent forward failed: "+err.Error())
		return
	}

	// Stream responses
	for msg := range respCh {
		if msg == nil {
			continue
		}

		switch msg.Type {
		case tunnel.MsgTypeStream:
			// Buffer the message
			bufMsg := buffer.Message{
				Type:      "stream",
				RequestID: requestID,
				Payload:   msg.Payload,
			}
			msgID, _ := c.server.buffer.Push(ctx, sessionID, bufMsg)

			c.sendMessage(ServerMessage{
				Type:    "stream",
				ID:      requestID,
				MsgID:   msgID,
				Payload: json.RawMessage(msg.Payload),
			})

		case tunnel.MsgTypeStreamEnd:
			// Buffer stream end
			bufMsg := buffer.Message{
				Type:      "stream.end",
				RequestID: requestID,
			}
			msgID, _ := c.server.buffer.Push(ctx, sessionID, bufMsg)

			c.sendMessage(ServerMessage{
				Type:    "stream.end",
				ID:      requestID,
				MsgID:   msgID,
				Payload: nil,
			})

		case tunnel.MsgTypeError:
			c.sendMessage(ServerMessage{
				Type:    "error",
				ID:      requestID,
				Payload: json.RawMessage(msg.Payload),
			})
		}
	}
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
