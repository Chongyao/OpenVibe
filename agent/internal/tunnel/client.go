package tunnel

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openvibe/agent/internal/opencode"
	"github.com/openvibe/agent/internal/project"
)

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

type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type RegisterPayload struct {
	AgentID      string   `json:"agentId"`
	Token        string   `json:"token"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

type RegisteredPayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type RequestPayload struct {
	SessionID   string          `json:"sessionId"`
	Action      string          `json:"action"`
	Data        json.RawMessage `json:"data"`
	ProjectPath string          `json:"projectPath,omitempty"`
}

type Client struct {
	hubURL         string
	agentID        string
	token          string
	opencodeClient *opencode.Client
	projectMgr     *project.Manager
	conn           *websocket.Conn
	reconnectDelay time.Duration
	maxReconnect   time.Duration
}

func NewClient(hubURL, agentID, token string, opencodeClient *opencode.Client, projectMgr *project.Manager) *Client {
	return &Client{
		hubURL:         hubURL,
		agentID:        agentID,
		token:          token,
		opencodeClient: opencodeClient,
		projectMgr:     projectMgr,
		reconnectDelay: time.Second,
		maxReconnect:   30 * time.Second,
	}
}

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
				c.reconnectDelay = min(c.reconnectDelay*2, c.maxReconnect)
			}
			continue
		}

		c.reconnectDelay = time.Second
	}
}

func (c *Client) connectAndRun(ctx context.Context) error {
	log.Printf("Connecting to Hub: %s", c.hubURL)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.hubURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	defer conn.Close()

	regPayload, _ := json.Marshal(RegisterPayload{
		AgentID:      c.agentID,
		Token:        c.token,
		Capabilities: []string{"opencode", "multi-project"},
		Version:      "0.2.0",
	})

	if err := conn.WriteJSON(Message{
		Type:    MsgTypeRegister,
		Payload: regPayload,
	}); err != nil {
		return err
	}

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
	c.reconnectDelay = time.Second

	if c.projectMgr != nil {
		c.projectMgr.SyncWithTmux(ctx)
	}

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

	switch req.Action {
	case "project.list":
		c.handleProjectList(msg.ID)
	case "project.start":
		c.handleProjectStart(ctx, msg.ID, req.Data)
	case "project.stop":
		c.handleProjectStop(ctx, msg.ID, req.Data)
	default:
		c.handleOpenCodeRequest(ctx, msg.ID, req)
	}
}

func (c *Client) handleProjectList(requestID string) {
	if c.projectMgr == nil {
		c.sendError(requestID, "project manager not configured")
		return
	}

	projects := c.projectMgr.List()
	payload, _ := json.Marshal(map[string]interface{}{"projects": projects})
	c.conn.WriteJSON(Message{
		Type:    MsgTypeResponse,
		ID:      requestID,
		Payload: payload,
	})
}

func (c *Client) handleProjectStart(ctx context.Context, requestID string, data json.RawMessage) {
	if c.projectMgr == nil {
		c.sendError(requestID, "project manager not configured")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		c.sendError(requestID, "invalid project.start payload")
		return
	}

	inst, err := c.projectMgr.Start(ctx, req.Path)
	if err != nil {
		c.sendError(requestID, err.Error())
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{"project": inst})
	c.conn.WriteJSON(Message{
		Type:    MsgTypeResponse,
		ID:      requestID,
		Payload: payload,
	})
}

func (c *Client) handleProjectStop(ctx context.Context, requestID string, data json.RawMessage) {
	if c.projectMgr == nil {
		c.sendError(requestID, "project manager not configured")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		c.sendError(requestID, "invalid project.stop payload")
		return
	}

	if err := c.projectMgr.Stop(ctx, req.Path); err != nil {
		c.sendError(requestID, err.Error())
		return
	}

	payload, _ := json.Marshal(map[string]bool{"success": true})
	c.conn.WriteJSON(Message{
		Type:    MsgTypeResponse,
		ID:      requestID,
		Payload: payload,
	})
}

func (c *Client) handleOpenCodeRequest(ctx context.Context, requestID string, req RequestPayload) {
	var baseURL string

	if c.projectMgr != nil && req.ProjectPath != "" {
		url, err := c.projectMgr.GetOpenCodeURL(req.ProjectPath)
		if err != nil {
			c.sendError(requestID, err.Error())
			return
		}
		baseURL = url
	}

	var streamCh <-chan []byte
	var err error

	if baseURL != "" {
		streamCh, err = c.opencodeClient.HandleRequestWithURL(ctx, baseURL, req.SessionID, req.Action, req.Data)
	} else {
		streamCh, err = c.opencodeClient.HandleRequest(ctx, req.SessionID, req.Action, req.Data)
	}

	if err != nil {
		c.sendError(requestID, err.Error())
		return
	}

	isStreaming := req.Action == "prompt"

	if isStreaming {
		for chunk := range streamCh {
			c.conn.WriteJSON(Message{
				Type:    MsgTypeStream,
				ID:      requestID,
				Payload: chunk,
			})
		}
		c.conn.WriteJSON(Message{
			Type: MsgTypeStreamEnd,
			ID:   requestID,
		})
	} else {
		var responseData []byte
		for chunk := range streamCh {
			responseData = chunk
		}
		c.conn.WriteJSON(Message{
			Type:    MsgTypeResponse,
			ID:      requestID,
			Payload: responseData,
		})
	}
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
