# Message Buffer Design (Mosh-Style Sync)

> **目标**: 实现类似 Mosh 的状态同步机制，容忍移动网络的高延迟和频繁断连

## 设计原理

### 为什么不用 TCP 流?

传统 WebSocket 是 TCP 流模式：
- 断连 = 状态丢失
- 重连 = 重新开始
- 移动网络 = 频繁断连 = 糟糕体验

### Mosh 模式

Mosh (Mobile Shell) 的核心思想：
- **状态同步** 而非流同步
- 服务端缓存最近 N 条消息
- 客户端记住"我看到了哪条"
- 重连时只补发缺失的部分

## 接口设计

```go
// buffer/buffer.go

package buffer

import "context"

// Message 代表一条缓冲的消息
type Message struct {
    ID        int64           `json:"id"`        // 递增 ID
    Type      string          `json:"type"`      // 消息类型
    RequestID string          `json:"requestId"` // 原始请求 ID
    Payload   json.RawMessage `json:"payload"`   // 消息内容
    Timestamp int64           `json:"timestamp"` // Unix 毫秒时间戳
}

// Buffer 消息缓冲接口
type Buffer interface {
    // Push 添加消息到缓冲，返回分配的 ID
    Push(ctx context.Context, sessionID string, msg Message) (int64, error)
    
    // GetSince 获取指定 ID 之后的所有消息
    GetSince(ctx context.Context, sessionID string, afterID int64) ([]Message, error)
    
    // GetLatestID 获取最新的消息 ID
    GetLatestID(ctx context.Context, sessionID string) (int64, error)
    
    // Trim 清理过期消息（保留最近 N 条或 T 时间内的）
    Trim(ctx context.Context, sessionID string) error
    
    // Close 关闭连接
    Close() error
}
```

## Redis 实现

### 数据结构选择

| 方案 | 数据结构 | 优点 | 缺点 |
|------|----------|------|------|
| LIST | LPUSH + LRANGE | 简单 | 按 ID 查找 O(n) |
| ZSET | ZADD + ZRANGEBYSCORE | 按 ID 查找 O(logn) | 内存占用大 |
| STREAM | XADD + XREAD | 原生支持 | 复杂度高 |

**选择**: ZSET (Sorted Set)
- Score = 消息 ID (int64)
- Member = JSON 序列化的消息
- 支持 ZRANGEBYSCORE 高效范围查询

### Redis 命令

```redis
# 推送消息
INCR openvibe:session:{sid}:msgid           # 获取下一个 ID
ZADD openvibe:session:{sid}:messages {id} {json}  # 添加到 ZSET
EXPIRE openvibe:session:{sid}:messages 300   # 5 分钟过期

# 获取 ID > afterID 的消息
ZRANGEBYSCORE openvibe:session:{sid}:messages ({afterID} +inf

# 获取最新 ID
GET openvibe:session:{sid}:msgid

# 清理旧消息（保留最新 100 条）
ZREMRANGEBYRANK openvibe:session:{sid}:messages 0 -101
```

### 实现代码

```go
// buffer/redis.go

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
    DefaultTTL       = 5 * time.Minute  // 消息保留时间
    DefaultMaxCount  = 100              // 最大消息数
)

type RedisBuffer struct {
    client *redis.Client
    ttl    time.Duration
    maxCount int64
}

func NewRedisBuffer(addr string, password string, db int) (*RedisBuffer, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis connection failed: %w", err)
    }
    
    return &RedisBuffer{
        client:   client,
        ttl:      DefaultTTL,
        maxCount: DefaultMaxCount,
    }, nil
}

func (b *RedisBuffer) keyMessages(sessionID string) string {
    return fmt.Sprintf("openvibe:session:%s:messages", sessionID)
}

func (b *RedisBuffer) keyMsgID(sessionID string) string {
    return fmt.Sprintf("openvibe:session:%s:msgid", sessionID)
}

func (b *RedisBuffer) Push(ctx context.Context, sessionID string, msg Message) (int64, error) {
    // 1. 获取下一个 ID
    id, err := b.client.Incr(ctx, b.keyMsgID(sessionID)).Result()
    if err != nil {
        return 0, fmt.Errorf("failed to get next id: %w", err)
    }
    
    msg.ID = id
    msg.Timestamp = time.Now().UnixMilli()
    
    // 2. 序列化消息
    data, err := json.Marshal(msg)
    if err != nil {
        return 0, fmt.Errorf("failed to marshal message: %w", err)
    }
    
    // 3. 添加到 ZSET
    key := b.keyMessages(sessionID)
    err = b.client.ZAdd(ctx, key, redis.Z{
        Score:  float64(id),
        Member: string(data),
    }).Err()
    if err != nil {
        return 0, fmt.Errorf("failed to push message: %w", err)
    }
    
    // 4. 设置过期时间
    b.client.Expire(ctx, key, b.ttl)
    
    return id, nil
}

func (b *RedisBuffer) GetSince(ctx context.Context, sessionID string, afterID int64) ([]Message, error) {
    key := b.keyMessages(sessionID)
    
    // ZRANGEBYSCORE key (afterID +inf
    results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
        Min: fmt.Sprintf("(%d", afterID), // 开区间
        Max: "+inf",
    }).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to get messages: %w", err)
    }
    
    messages := make([]Message, 0, len(results))
    for _, data := range results {
        var msg Message
        if err := json.Unmarshal([]byte(data), &msg); err != nil {
            continue // 跳过损坏的消息
        }
        messages = append(messages, msg)
    }
    
    return messages, nil
}

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

func (b *RedisBuffer) Trim(ctx context.Context, sessionID string) error {
    key := b.keyMessages(sessionID)
    // 保留最新的 maxCount 条，删除其余
    return b.client.ZRemRangeByRank(ctx, key, 0, -b.maxCount-1).Err()
}

func (b *RedisBuffer) Close() error {
    return b.client.Close()
}
```

## 服务端集成

### server.go 修改点

```go
type Server struct {
    config  *config.Config
    proxy   *proxy.OpenCodeProxy
    buffer  buffer.Buffer           // 新增
    clients map[*Client]bool
    mu      sync.RWMutex
}

// handleMessage 添加新的消息类型
func (c *Client) handleMessage(data []byte) {
    switch msg.Type {
    // ... 现有类型 ...
    
    case "sync":
        c.handleSync(msg.ID, payload)
        
    case "ack":
        c.handleAck(msg.ID, payload)
    }
}

// handleSync 处理同步请求
func (c *Client) handleSync(requestID string, payload SyncPayload) {
    messages, err := c.server.buffer.GetSince(ctx, payload.SessionID, payload.LastAckID)
    if err != nil {
        c.sendError(requestID, err.Error())
        return
    }
    
    latestID, _ := c.server.buffer.GetLatestID(ctx, payload.SessionID)
    
    c.sendMessage(ServerMessage{
        Type: "sync.batch",
        ID:   requestID,
        Payload: map[string]interface{}{
            "messages": messages,
            "latestID": latestID,
        },
    })
}
```

## 客户端集成

### useWebSocket.ts 修改点

```typescript
interface UseWebSocketOptions {
  // ... 现有选项 ...
  sessionId?: string;
}

export function useWebSocket(options: UseWebSocketOptions) {
  const lastAckIDRef = useRef<number>(0);
  
  const onConnect = useCallback(() => {
    // 重连时发送 sync 请求
    if (options.sessionId && lastAckIDRef.current > 0) {
      send({
        type: 'sync',
        id: generateId(),
        payload: {
          sessionId: options.sessionId,
          lastAckID: lastAckIDRef.current,
        },
      });
    }
    options.onConnect?.();
  }, [options.sessionId]);
  
  const handleMessage = useCallback((msg: ServerMessage) => {
    if (msg.type === 'sync.batch') {
      // 批量处理缺失的消息
      const { messages, latestID } = msg.payload as SyncBatchPayload;
      messages.forEach(m => options.onMessage?.(m));
      lastAckIDRef.current = latestID;
    } else {
      // 正常消息，更新 ack ID
      if (msg.payload?.msgID) {
        lastAckIDRef.current = msg.payload.msgID;
      }
      options.onMessage?.(msg);
    }
  }, []);
  
  // ...
}
```

## 降级策略

当 Redis 不可用时：

1. **检测**: 连接失败或操作超时
2. **降级**: 禁用缓冲，直接转发
3. **告警**: 记录日志，可选通知
4. **恢复**: 定期重试连接

```go
type FallbackBuffer struct {
    primary   Buffer
    available atomic.Bool
}

func (b *FallbackBuffer) Push(ctx context.Context, sessionID string, msg Message) (int64, error) {
    if !b.available.Load() {
        return 0, nil // 降级：不缓冲
    }
    
    id, err := b.primary.Push(ctx, sessionID, msg)
    if err != nil {
        b.available.Store(false)
        go b.tryRecover()
        return 0, nil
    }
    return id, nil
}
```

## 性能考虑

| 操作 | 时间复杂度 | 预期延迟 |
|------|------------|----------|
| Push | O(log n) | < 1ms |
| GetSince | O(log n + k) | < 5ms |
| Trim | O(log n + m) | < 2ms |

其中 n = 缓冲消息数，k = 返回消息数，m = 删除消息数

## 测试计划

1. **单元测试**: buffer 接口各方法
2. **集成测试**: Redis 连接和操作
3. **压力测试**: 高并发写入
4. **故障测试**: Redis 断连和恢复
