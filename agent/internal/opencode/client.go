// Package opencode provides HTTP client for OpenCode API
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is an OpenCode HTTP client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OpenCode client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{},
	}
}

// SessionInfo represents a session
type SessionInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// PromptRequest represents a message prompt
type PromptRequest struct {
	Parts []PromptPart `json:"parts"`
}

// PromptPart represents a prompt part
type PromptPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PromptData is the data sent with prompt action
type PromptData struct {
	Content string `json:"content"`
}

// SessionCreateData is the data sent with session.create action
type SessionCreateData struct {
	Title string `json:"title"`
}

// OpenCodeResponse is the response from OpenCode
type OpenCodeResponse struct {
	Info  json.RawMessage `json:"info"`
	Parts []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"parts"`
}

// HandleRequest implements the RequestHandler interface
func (c *Client) HandleRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	ch := make(chan []byte, 100)

	go func() {
		defer close(ch)

		switch action {
		case "session.create":
			c.handleSessionCreate(ctx, data, ch)
		case "session.list":
			c.handleSessionList(ctx, ch)
		case "prompt":
			c.handlePrompt(ctx, sessionID, data, ch)
		default:
			errPayload, _ := json.Marshal(map[string]string{"error": "unknown action: " + action})
			ch <- errPayload
		}
	}()

	return ch, nil
}

func (c *Client) handleSessionCreate(ctx context.Context, data json.RawMessage, ch chan<- []byte) {
	var createData SessionCreateData
	json.Unmarshal(data, &createData)

	body, _ := json.Marshal(map[string]string{"title": createData.Title})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ch <- respBody
}

func (c *Client) handleSessionList(ctx context.Context, ch chan<- []byte) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session", nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ch <- respBody
}

func (c *Client) handlePrompt(ctx context.Context, sessionID string, data json.RawMessage, ch chan<- []byte) {
	var promptData PromptData
	json.Unmarshal(data, &promptData)

	promptReq := PromptRequest{
		Parts: []PromptPart{
			{Type: "text", Text: promptData.Content},
		},
	}

	body, _ := json.Marshal(promptReq)
	url := fmt.Sprintf("%s/session/%s/message", c.baseURL, sessionID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		errPayload, _ := json.Marshal(map[string]string{"error": string(errBody)})
		ch <- errPayload
		return
	}

	var ocResp OpenCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocResp); err != nil {
		return
	}

	// Send text parts as stream chunks
	for _, part := range ocResp.Parts {
		if part.Type == "text" && part.Text != "" {
			textPayload, _ := json.Marshal(map[string]string{"text": part.Text})
			ch <- textPayload
		}
	}
}

// Health checks if OpenCode is reachable
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opencode unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
