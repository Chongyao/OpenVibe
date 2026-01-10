// Package buffer provides message buffering for Mosh-style sync
package buffer

import (
	"context"
	"encoding/json"
)

// Message represents a buffered message
type Message struct {
	ID        int64           `json:"id"`        // Monotonically increasing ID
	Type      string          `json:"type"`      // Message type (stream, stream.end, etc.)
	RequestID string          `json:"requestId"` // Original request ID
	Payload   json.RawMessage `json:"payload"`   // Message payload
	Timestamp int64           `json:"timestamp"` // Unix milliseconds
}

// Buffer interface for message buffering
type Buffer interface {
	// Push adds a message to the buffer, returns assigned ID
	Push(ctx context.Context, sessionID string, msg Message) (int64, error)

	// GetSince retrieves all messages after the specified ID
	GetSince(ctx context.Context, sessionID string, afterID int64) ([]Message, error)

	// GetLatestID returns the latest message ID for a session
	GetLatestID(ctx context.Context, sessionID string) (int64, error)

	// Trim removes old messages, keeping only recent ones
	Trim(ctx context.Context, sessionID string) error

	// Close releases resources
	Close() error
}

// NoopBuffer is a no-op implementation for when Redis is unavailable
type NoopBuffer struct{}

func NewNoopBuffer() *NoopBuffer {
	return &NoopBuffer{}
}

func (b *NoopBuffer) Push(ctx context.Context, sessionID string, msg Message) (int64, error) {
	return 0, nil
}

func (b *NoopBuffer) GetSince(ctx context.Context, sessionID string, afterID int64) ([]Message, error) {
	return nil, nil
}

func (b *NoopBuffer) GetLatestID(ctx context.Context, sessionID string) (int64, error) {
	return 0, nil
}

func (b *NoopBuffer) Trim(ctx context.Context, sessionID string) error {
	return nil
}

func (b *NoopBuffer) Close() error {
	return nil
}
