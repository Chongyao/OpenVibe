// Package buffer provides Redis-based message buffering
package buffer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultTTL is how long messages are retained
	DefaultTTL = 5 * time.Minute
	// DefaultMaxCount is maximum messages per session
	DefaultMaxCount = 100
)

// RedisBuffer implements Buffer using Redis sorted sets
type RedisBuffer struct {
	client   *redis.Client
	ttl      time.Duration
	maxCount int64
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	TTL      time.Duration
	MaxCount int64
}

// NewRedisBuffer creates a new Redis-backed buffer
func NewRedisBuffer(cfg RedisConfig) (*RedisBuffer, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}

	maxCount := cfg.MaxCount
	if maxCount == 0 {
		maxCount = DefaultMaxCount
	}

	return &RedisBuffer{
		client:   client,
		ttl:      ttl,
		maxCount: maxCount,
	}, nil
}

func (b *RedisBuffer) keyMessages(sessionID string) string {
	return fmt.Sprintf("openvibe:session:%s:messages", sessionID)
}

func (b *RedisBuffer) keyMsgID(sessionID string) string {
	return fmt.Sprintf("openvibe:session:%s:msgid", sessionID)
}

// Push adds a message to the buffer
func (b *RedisBuffer) Push(ctx context.Context, sessionID string, msg Message) (int64, error) {
	// Get next ID
	id, err := b.client.Incr(ctx, b.keyMsgID(sessionID)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get next id: %w", err)
	}

	msg.ID = id
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add to sorted set
	key := b.keyMessages(sessionID)
	err = b.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(id),
		Member: string(data),
	}).Err()
	if err != nil {
		return 0, fmt.Errorf("failed to push message: %w", err)
	}

	// Set TTL
	b.client.Expire(ctx, key, b.ttl)
	b.client.Expire(ctx, b.keyMsgID(sessionID), b.ttl)

	return id, nil
}

// GetSince retrieves messages after the specified ID
func (b *RedisBuffer) GetSince(ctx context.Context, sessionID string, afterID int64) ([]Message, error) {
	key := b.keyMessages(sessionID)

	// ZRANGEBYSCORE key (afterID +inf
	results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", afterID), // Open interval
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	messages := make([]Message, 0, len(results))
	for _, data := range results {
		var msg Message
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue // Skip corrupted messages
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetLatestID returns the latest message ID
func (b *RedisBuffer) GetLatestID(ctx context.Context, sessionID string) (int64, error) {
	result, err := b.client.Get(ctx, b.keyMsgID(sessionID)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get latest id: %w", err)
	}

	id, _ := strconv.ParseInt(result, 10, 64)
	return id, nil
}

// Trim removes old messages, keeping only the most recent ones
func (b *RedisBuffer) Trim(ctx context.Context, sessionID string) error {
	key := b.keyMessages(sessionID)
	// Keep the latest maxCount messages, remove the rest
	return b.client.ZRemRangeByRank(ctx, key, 0, -b.maxCount-1).Err()
}

// Close closes the Redis connection
func (b *RedisBuffer) Close() error {
	return b.client.Close()
}
