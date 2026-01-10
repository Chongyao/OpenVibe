// Package tunnel provides WebSocket reverse tunnel for Agent connections
package tunnel

import (
	"encoding/json"
)

// Message types for Agent ↔ Hub communication
const (
	// Agent → Hub
	MsgTypeRegister  = "agent.register"
	MsgTypePong      = "agent.pong"
	MsgTypeResponse  = "agent.response"
	MsgTypeStream    = "agent.stream"
	MsgTypeStreamEnd = "agent.stream.end"
	MsgTypeError     = "agent.error"

	// Hub → Agent
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

// RegisterPayload is sent by Agent to register with Hub
type RegisterPayload struct {
	AgentID      string   `json:"agentId"`
	Token        string   `json:"token"`
	Capabilities []string `json:"capabilities"` // ["opencode", "pty", "file"]
	Version      string   `json:"version"`
}

// RegisteredPayload is sent by Hub to confirm registration
type RegisteredPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// RequestPayload is sent by Hub to forward a client request
type RequestPayload struct {
	SessionID   string          `json:"sessionId"`
	Action      string          `json:"action"` // "prompt", "session.create", "session.list"
	Data        json.RawMessage `json:"data"`
	ProjectPath string          `json:"projectPath,omitempty"`
}

// StreamPayload is sent by Agent for streaming responses
type StreamPayload struct {
	RequestID string          `json:"requestId"`
	Text      string          `json:"text,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// ErrorPayload is sent for error responses
type ErrorPayload struct {
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}

// MustMarshal marshals v to JSON, panics on error
func MustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
