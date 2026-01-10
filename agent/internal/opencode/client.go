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

type Client struct {
	defaultURL string
	httpClient *http.Client
}

func NewClient(defaultURL string) *Client {
	return &Client{
		defaultURL: strings.TrimSuffix(defaultURL, "/"),
		httpClient: &http.Client{},
	}
}

type SessionInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type PromptRequest struct {
	Parts []PromptPart `json:"parts"`
}

type PromptPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type PromptData struct {
	Content string `json:"content"`
}

type SessionCreateData struct {
	Title string `json:"title"`
}

type OpenCodeResponse struct {
	Info  json.RawMessage `json:"info"`
	Parts []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"parts"`
}

func (c *Client) HandleRequest(ctx context.Context, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	return c.HandleRequestWithURL(ctx, c.defaultURL, sessionID, action, data)
}

func (c *Client) HandleRequestWithURL(ctx context.Context, baseURL, sessionID, action string, data json.RawMessage) (<-chan []byte, error) {
	if baseURL == "" {
		baseURL = c.defaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	ch := make(chan []byte, 100)

	go func() {
		defer close(ch)

		switch action {
		case "session.create":
			c.handleSessionCreate(ctx, baseURL, data, ch)
		case "session.list":
			c.handleSessionList(ctx, baseURL, ch)
		case "session.messages":
			c.handleSessionMessages(ctx, baseURL, sessionID, ch)
		case "session.delete":
			c.handleSessionDelete(ctx, baseURL, sessionID, ch)
		case "prompt":
			c.handlePrompt(ctx, baseURL, sessionID, data, ch)
		default:
			errPayload, _ := json.Marshal(map[string]string{"error": "unknown action: " + action})
			ch <- errPayload
		}
	}()

	return ch, nil
}

func (c *Client) handleSessionCreate(ctx context.Context, baseURL string, data json.RawMessage, ch chan<- []byte) {
	var createData SessionCreateData
	json.Unmarshal(data, &createData)

	body, _ := json.Marshal(map[string]string{"title": createData.Title})
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/session", bytes.NewReader(body))
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

func (c *Client) handleSessionList(ctx context.Context, baseURL string, ch chan<- []byte) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/session", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ch <- respBody
}

func (c *Client) handleSessionMessages(ctx context.Context, baseURL, sessionID string, ch chan<- []byte) {
	url := fmt.Sprintf("%s/session/%s/message", baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		ch <- errPayload
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		ch <- errPayload
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		errPayload, _ := json.Marshal(map[string]string{"error": string(errBody)})
		ch <- errPayload
		return
	}

	respBody, _ := io.ReadAll(resp.Body)
	ch <- respBody
}

func (c *Client) handleSessionDelete(ctx context.Context, baseURL, sessionID string, ch chan<- []byte) {
	url := fmt.Sprintf("%s/session/%s", baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		ch <- errPayload
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		ch <- errPayload
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		errPayload, _ := json.Marshal(map[string]string{"error": string(errBody)})
		ch <- errPayload
		return
	}

	successPayload, _ := json.Marshal(map[string]interface{}{"success": true, "sessionId": sessionID})
	ch <- successPayload
}

func (c *Client) handlePrompt(ctx context.Context, baseURL, sessionID string, data json.RawMessage, ch chan<- []byte) {
	var promptData PromptData
	json.Unmarshal(data, &promptData)

	promptReq := PromptRequest{
		Parts: []PromptPart{
			{Type: "text", Text: promptData.Content},
		},
	}

	body, _ := json.Marshal(promptReq)
	url := fmt.Sprintf("%s/session/%s/message", baseURL, sessionID)

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

	for _, part := range ocResp.Parts {
		if part.Type == "text" && part.Text != "" {
			textPayload, _ := json.Marshal(map[string]string{"text": part.Text})
			ch <- textPayload
		}
	}
}

func (c *Client) Health(ctx context.Context) error {
	return c.HealthWithURL(ctx, c.defaultURL)
}

func (c *Client) HealthWithURL(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		baseURL = c.defaultURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/global/health", nil)
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
