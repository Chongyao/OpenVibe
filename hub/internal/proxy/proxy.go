package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenCodeProxy handles communication with OpenCode server
type OpenCodeProxy struct {
	baseURL    string
	httpClient *http.Client
}

// NewOpenCodeProxy creates a new OpenCode proxy
func NewOpenCodeProxy(baseURL string) *OpenCodeProxy {
	return &OpenCodeProxy{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
	}
}

// SessionInfo represents a session
type SessionInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Message represents a chat message
type Message struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CreateSessionRequest represents session creation request
type CreateSessionRequest struct {
	Title string `json:"title,omitempty"`
}

// PromptRequest represents a message prompt request
type PromptRequest struct {
	Parts []PromptPart `json:"parts"`
	Model *ModelInfo   `json:"model,omitempty"`
}

// PromptPart represents a part of the prompt
type PromptPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ModelInfo represents model selection
type ModelInfo struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// StreamCallback is called for each SSE event
type StreamCallback func(eventType string, data []byte) error

// Health checks if OpenCode is reachable
func (p *OpenCodeProxy) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opencode unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opencode unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// ListSessions returns all sessions
func (p *OpenCodeProxy) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/session", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sessions []SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// CreateSession creates a new session
func (p *OpenCodeProxy) CreateSession(ctx context.Context, title string) (*SessionInfo, error) {
	body, _ := json.Marshal(CreateSessionRequest{Title: title})
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/session", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var session SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SendMessage sends a message and streams the response
func (p *OpenCodeProxy) SendMessage(ctx context.Context, sessionID string, content string, callback StreamCallback) error {
	promptReq := PromptRequest{
		Parts: []PromptPart{
			{Type: "text", Text: content},
		},
	}

	body, _ := json.Marshal(promptReq)
	url := fmt.Sprintf("%s/session/%s/message", p.baseURL, sessionID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opencode error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read streaming JSON response
	decoder := json.NewDecoder(resp.Body)
	for {
		var msg json.RawMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := callback("message", msg); err != nil {
			return err
		}
	}

	return nil
}

// SubscribeEvents subscribes to SSE events
func (p *OpenCodeProxy) SubscribeEvents(ctx context.Context, callback StreamCallback) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/event", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{
		Timeout: 0, // No timeout for SSE
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataLines []string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}

		line = strings.TrimSpace(line)

		if line == "" {
			// End of event
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				if err := callback(eventType, []byte(data)); err != nil {
					return err
				}
			}
			eventType = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
		}
	}
}

// GetMessages retrieves message history for a session
func (p *OpenCodeProxy) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	url := fmt.Sprintf("%s/session/%s/message", p.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}
