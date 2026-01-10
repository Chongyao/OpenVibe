// Package tunnel provides WebSocket reverse tunnel management
package tunnel

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Errors
var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrAgentOffline  = errors.New("agent offline")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrTimeout       = errors.New("request timeout")
)

// Constants for WebSocket handling
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Config holds tunnel manager configuration
type Config struct {
	AgentToken   string        // Pre-shared secret for agent auth
	PingInterval time.Duration // How often to ping agents
	PongTimeout  time.Duration // How long to wait for pong
}

// Manager manages agent connections
type Manager struct {
	config *Config
	agents map[string]*Agent
	mu     sync.RWMutex
}

// Agent represents a connected agent
type Agent struct {
	ID           string
	Conn         *websocket.Conn
	Capabilities []string
	LastSeen     time.Time
	send         chan []byte
	requests     map[string]chan *Message // requestID -> response channel
	mu           sync.RWMutex
}

// NewManager creates a new tunnel manager
func NewManager(cfg *Config) *Manager {
	if cfg.PingInterval == 0 {
		cfg.PingInterval = pingPeriod
	}
	if cfg.PongTimeout == 0 {
		cfg.PongTimeout = pongWait
	}
	return &Manager{
		config: cfg,
		agents: make(map[string]*Agent),
	}
}

// HandleAgentWebSocket handles agent WebSocket connections
func (m *Manager) HandleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Agent WebSocket upgrade error: %v", err)
		return
	}

	// Wait for register message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Agent read register error: %v", err)
		conn.Close()
		return
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Agent invalid register message: %v", err)
		conn.Close()
		return
	}

	if msg.Type != MsgTypeRegister {
		log.Printf("Agent expected register, got: %s", msg.Type)
		conn.Close()
		return
	}

	var payload RegisterPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("Agent invalid register payload: %v", err)
		conn.Close()
		return
	}

	// Validate token
	if m.config.AgentToken != "" {
		if subtle.ConstantTimeCompare([]byte(payload.Token), []byte(m.config.AgentToken)) != 1 {
			log.Printf("Agent unauthorized: %s", payload.AgentID)
			conn.WriteJSON(Message{
				Type:    MsgTypeRegistered,
				Payload: MustMarshal(RegisteredPayload{Success: false, Error: "unauthorized"}),
			})
			conn.Close()
			return
		}
	}

	agent := &Agent{
		ID:           payload.AgentID,
		Conn:         conn,
		Capabilities: payload.Capabilities,
		LastSeen:     time.Now(),
		send:         make(chan []byte, 256),
		requests:     make(map[string]chan *Message),
	}

	// Register agent
	m.mu.Lock()
	// Close existing connection if any
	if existing, ok := m.agents[agent.ID]; ok {
		existing.Conn.Close()
	}
	m.agents[agent.ID] = agent
	m.mu.Unlock()

	log.Printf("Agent registered: %s from %s", agent.ID, conn.RemoteAddr())

	// Send success response
	conn.WriteJSON(Message{
		Type:    MsgTypeRegistered,
		Payload: MustMarshal(RegisteredPayload{Success: true}),
	})

	// Configure connection
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		agent.mu.Lock()
		agent.LastSeen = time.Now()
		agent.mu.Unlock()
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start pumps
	go m.writePump(agent)
	m.readPump(agent)
}

func (m *Manager) readPump(agent *Agent) {
	defer func() {
		m.mu.Lock()
		delete(m.agents, agent.ID)
		m.mu.Unlock()
		agent.Conn.Close()
		close(agent.send)
		log.Printf("Agent disconnected: %s", agent.ID)
	}()

	for {
		_, data, err := agent.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Agent read error: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Agent invalid message: %v", err)
			continue
		}

		m.handleAgentMessage(agent, &msg)
	}
}

func (m *Manager) writePump(agent *Agent) {
	ticker := time.NewTicker(m.config.PingInterval)
	defer func() {
		ticker.Stop()
		agent.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-agent.send:
			agent.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				agent.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := agent.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			agent.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := agent.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (m *Manager) handleAgentMessage(agent *Agent, msg *Message) {
	switch msg.Type {
	case MsgTypePong:
		agent.mu.Lock()
		agent.LastSeen = time.Now()
		agent.mu.Unlock()

	case MsgTypeResponse, MsgTypeStream, MsgTypeStreamEnd, MsgTypeError:
		// Route to waiting request
		if msg.ID != "" {
			agent.mu.RLock()
			ch, ok := agent.requests[msg.ID]
			agent.mu.RUnlock()
			if ok {
				select {
				case ch <- msg:
				default:
					log.Printf("Agent response channel full for request: %s", msg.ID)
				}
			}
		}
	}
}

// Forward sends a request to an agent and returns a channel for responses
func (m *Manager) Forward(ctx context.Context, agentID string, requestID string, req *RequestPayload) (<-chan *Message, error) {
	m.mu.RLock()
	agent, ok := m.agents[agentID]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrAgentNotFound
	}

	responseCh := make(chan *Message, 100)

	agent.mu.Lock()
	agent.requests[requestID] = responseCh
	agent.mu.Unlock()

	// Send request
	msg := Message{
		Type:    MsgTypeRequest,
		ID:      requestID,
		Payload: MustMarshal(req),
	}

	data, _ := json.Marshal(msg)
	select {
	case agent.send <- data:
	default:
		agent.mu.Lock()
		delete(agent.requests, requestID)
		agent.mu.Unlock()
		close(responseCh)
		return nil, errors.New("agent send buffer full")
	}

	// Cleanup when context done
	go func() {
		<-ctx.Done()
		agent.mu.Lock()
		delete(agent.requests, requestID)
		agent.mu.Unlock()
		close(responseCh)
	}()

	return responseCh, nil
}

// GetAgent returns an agent by ID
func (m *Manager) GetAgent(agentID string) (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[agentID]
	return agent, ok
}

// GetAnyAgent returns any available agent
func (m *Manager) GetAnyAgent() (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, agent := range m.agents {
		return agent, true
	}
	return nil, false
}

// ListAgents returns all connected agent IDs
func (m *Manager) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids
}
