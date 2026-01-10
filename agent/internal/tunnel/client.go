// Package tunnel provides WebSocket tunnel client for connecting to Hub
package tunnel

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// Message types
const (
	MsgTypeRegister   = "agent.register"
	MsgTypePong       = "agent.pong"
	MsgTypeResponse   = "agent.response"
	MsgTypeStream     = "agent.stream"
	MsgTypeStreamEnd  = "agent.stream.end"
	MsgTypeError      = "agent.error"
	MsgTypeRegistered = "agent.registered"
	MsgTypePing       = "agent.ping"
	MsgTypeRequest    = "agent.request"
)

// Message represents a tunnel protocol message
type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RegisterPayload is sent to register with Hub
type RegisterPayload struct {
	AgentID      string   `json:"agentId"`
	Token        string   `json:"token"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

// RegisteredPayload is received on registration
type RegisteredPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// RequestPayload is received for client requests
type RequestPayload struct {
	SessionID string          `json:"sessionId"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
}

// RequestHandler handles incoming requests
type RequestHandler interface {
	HandleRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error)
}

// Client is a tunnel client that connects to Hub
type Client struct {
	hubURL  string
	agentID string
	token   string
	handler RequestHandler
	conn    *websocket.Conn

	reconnectDelay time.Duration
	maxReconnect   time.Duration
}

// NewClient creates a new tunnel client
func NewClient(hubURL, agentID, token string, handler RequestHandler) *Client {
	return &Client{
		hubURL:         hubURL,
		agentID:        agentID,
		token:          token,
		handler:        handler,
		reconnectDelay: time.Second,
		maxReconnect:   30 * time.Second,
	}
}

// Run connects to Hub and processes requests
func (c *Client) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.connectAndRun(ctx); err != nil {
			log.Printf("Connection error: %v, reconnecting in %v", err, c.reconnectDelay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.reconnectDelay):
				// Exponential backoff
				c.reconnectDelay = min(c.reconnectDelay*2, c.maxReconnect)
			}
			continue
		}

		// Connection closed normally, reset delay
		c.reconnectDelay = time.Second
	}
}

func (c *Client) connectAndRun(ctx context.Context) error {
	// Connect
	log.Printf("Connecting to Hub: %s", c.hubURL)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.hubURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	defer conn.Close()

	// Register
	regPayload, _ := json.Marshal(RegisterPayload{
		AgentID:      c.agentID,
		Token:        c.token,
		Capabilities: []string{"opencode"},
		Version:      "0.1.0",
	})

	if err := conn.WriteJSON(Message{
		Type:    MsgTypeRegister,
		Payload: regPayload,
	}); err != nil {
		return err
	}

	// Wait for registration response
	var regResp Message
	if err := conn.ReadJSON(&regResp); err != nil {
		return err
	}

	if regResp.Type != MsgTypeRegistered {
		return err
	}

	var registered RegisteredPayload
	json.Unmarshal(regResp.Payload, &registered)
	if !registered.Success {
		log.Printf("Registration failed: %s", registered.Error)
		return err
	}

	log.Printf("Registered with Hub successfully")
	c.reconnectDelay = time.Second // Reset on successful connect

	// Main loop
	return c.readLoop(ctx)
}

func (c *Client) readLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			return err
		}

		switch msg.Type {
		case MsgTypePing:
			c.conn.WriteJSON(Message{Type: MsgTypePong})

		case MsgTypeRequest:
			go c.handleRequest(ctx, msg)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, msg Message) {
	var req RequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendError(msg.ID, "invalid request payload")
		return
	}

	// Call handler
	streamCh, err := c.handler.HandleRequest(ctx, req.SessionID, req.Action, req.Data)
	if err != nil {
		c.sendError(msg.ID, err.Error())
		return
	}

	// Forward stream chunks
	for chunk := range streamCh {
		c.conn.WriteJSON(Message{
			Type:    MsgTypeStream,
			ID:      msg.ID,
			Payload: chunk,
		})
	}

	// Send stream end
	c.conn.WriteJSON(Message{
		Type: MsgTypeStreamEnd,
		ID:   msg.ID,
	})
}

func (c *Client) sendError(requestID, errMsg string) {
	payload, _ := json.Marshal(map[string]string{"error": errMsg})
	c.conn.WriteJSON(Message{
		Type:    MsgTypeError,
		ID:      requestID,
		Payload: payload,
	})
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
