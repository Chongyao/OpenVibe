# Reverse Tunnel Design (Agent ↔ Hub)

> **目标**: 实现 Agent 主动连接 Hub 的反向隧道，穿透 NAT/防火墙

## 设计原理

### 为什么需要反向隧道?

传统模式：Hub 连接 Agent (不可行)
- Agent 在 NAT 后，无公网 IP
- 防火墙阻止入站连接
- 需要配置端口转发

反向隧道：Agent 连接 Hub (可行)
- Agent 主动建立出站 WebSocket
- Hub 通过此连接下发请求
- 无需任何网络配置

### 类似方案

| 工具 | 原理 | 复杂度 |
|------|------|--------|
| ngrok | HTTP 隧道 + 自定义域名 | 高 |
| frp | TCP/UDP 隧道 | 中 |
| **OpenVibe** | WebSocket 隧道 (专用) | 低 |

## 协议设计

### 连接建立

```
Agent                          Hub
  |                             |
  |---- WS Connect /agent ----->|
  |                             |
  |<--- agent.challenge --------|  (可选: 挑战-响应认证)
  |---- agent.response -------->|
  |                             |
  |---- agent.register -------->|  { agentId, token, capabilities }
  |<--- agent.registered -------|  { success, sessionTTL }
  |                             |
  |<--- agent.ping -------------|  (定期心跳)
  |---- agent.pong ------------>|
  |                             |
```

### 消息转发

```
App                    Hub                    Agent
 |                      |                       |
 |--- prompt ---------->|                       |
 |                      |--- agent.request ---->|  { action: 'prompt', ... }
 |                      |                       |
 |                      |<-- agent.response ----|  { stream chunk }
 |<-- stream -----------|                       |
 |                      |                       |
 |                      |<-- agent.response ----|  { stream end }
 |<-- stream.end -------|                       |
 |                      |                       |
```

### 消息格式

```go
// tunnel/protocol.go

package tunnel

// AgentMessage Agent 发送的消息
type AgentMessage struct {
    Type    string          `json:"type"`
    ID      string          `json:"id,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}

// 消息类型常量
const (
    // Agent → Hub
    TypeAgentRegister = "agent.register"
    TypeAgentPong     = "agent.pong"
    TypeAgentResponse = "agent.response"
    TypeAgentStream   = "agent.stream"
    TypeAgentError    = "agent.error"
    
    // Hub → Agent
    TypeAgentChallenge  = "agent.challenge"
    TypeAgentRegistered = "agent.registered"
    TypeAgentPing       = "agent.ping"
    TypeAgentRequest    = "agent.request"
)

// RegisterPayload 注册请求
type RegisterPayload struct {
    AgentID      string   `json:"agentId"`
    Token        string   `json:"token"`
    Capabilities []string `json:"capabilities"` // ["opencode", "pty", "file"]
    Version      string   `json:"version"`
}

// RequestPayload 请求转发
type RequestPayload struct {
    SessionID string          `json:"sessionId"`
    Action    string          `json:"action"` // "prompt", "session.create", etc.
    Data      json.RawMessage `json:"data"`
}

// StreamPayload 流式响应
type StreamPayload struct {
    RequestID string          `json:"requestId"`
    Chunk     json.RawMessage `json:"chunk"`
    Done      bool            `json:"done"`
}
```

## Hub 端实现

### Tunnel Manager

```go
// tunnel/manager.go

package tunnel

import (
    "context"
    "sync"
    "time"
    
    "github.com/gorilla/websocket"
)

type Manager struct {
    agents   map[string]*AgentConn
    mu       sync.RWMutex
    config   *Config
}

type AgentConn struct {
    ID           string
    Conn         *websocket.Conn
    Capabilities []string
    LastSeen     time.Time
    requests     map[string]chan *AgentMessage // 请求 ID -> 响应通道
    mu           sync.RWMutex
}

type Config struct {
    PingInterval time.Duration
    PongTimeout  time.Duration
    AuthToken    string // 预共享密钥
}

func NewManager(cfg *Config) *Manager {
    return &Manager{
        agents: make(map[string]*AgentConn),
        config: cfg,
    }
}

// HandleAgentWebSocket 处理 Agent 连接
func (m *Manager) HandleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    agent := &AgentConn{
        Conn:     conn,
        requests: make(map[string]chan *AgentMessage),
    }
    
    // 等待注册消息
    if err := m.waitForRegister(agent); err != nil {
        conn.Close()
        return
    }
    
    // 注册成功
    m.mu.Lock()
    m.agents[agent.ID] = agent
    m.mu.Unlock()
    
    // 启动读写协程
    go m.readPump(agent)
    go m.pingPump(agent)
}

// Forward 转发请求到 Agent
func (m *Manager) Forward(ctx context.Context, agentID string, req *RequestPayload) (<-chan *AgentMessage, error) {
    m.mu.RLock()
    agent, ok := m.agents[agentID]
    m.mu.RUnlock()
    
    if !ok {
        return nil, ErrAgentNotFound
    }
    
    requestID := generateID()
    responseCh := make(chan *AgentMessage, 10)
    
    agent.mu.Lock()
    agent.requests[requestID] = responseCh
    agent.mu.Unlock()
    
    // 发送请求
    msg := AgentMessage{
        Type: TypeAgentRequest,
        ID:   requestID,
        Payload: mustMarshal(req),
    }
    
    if err := agent.Conn.WriteJSON(msg); err != nil {
        return nil, err
    }
    
    return responseCh, nil
}

// GetAgent 获取可用的 Agent
func (m *Manager) GetAgent(sessionID string) (*AgentConn, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // 简单策略：返回第一个可用的 Agent
    // TODO: 支持 session 亲和性
    for _, agent := range m.agents {
        return agent, true
    }
    return nil, false
}
```

### 路由集成

```go
// cmd/hub/main.go

func main() {
    // ...
    
    tunnelMgr := tunnel.NewManager(&tunnel.Config{
        PingInterval: 30 * time.Second,
        PongTimeout:  10 * time.Second,
        AuthToken:    cfg.AgentToken,
    })
    
    mux.HandleFunc("/agent", tunnelMgr.HandleAgentWebSocket)
    
    // 将 tunnelMgr 传给 server
    wsServer := server.NewServer(cfg, tunnelMgr, buffer)
    
    // ...
}
```

## Agent 端实现

### 隧道客户端

```go
// agent/internal/tunnel/tunnel.go

package tunnel

import (
    "context"
    "time"
    
    "github.com/gorilla/websocket"
)

type Client struct {
    hubURL   string
    agentID  string
    token    string
    conn     *websocket.Conn
    handler  RequestHandler
    
    reconnectDelay time.Duration
    maxReconnect   time.Duration
}

type RequestHandler func(ctx context.Context, req *RequestPayload) (<-chan *StreamPayload, error)

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

func (c *Client) Connect(ctx context.Context) error {
    for {
        if err := c.connectOnce(ctx); err != nil {
            log.Printf("Connection failed: %v, reconnecting in %v", err, c.reconnectDelay)
            
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(c.reconnectDelay):
                // 指数退避
                c.reconnectDelay = min(c.reconnectDelay*2, c.maxReconnect)
            }
            continue
        }
        
        // 连接成功，重置延迟
        c.reconnectDelay = time.Second
        
        // 运行主循环
        if err := c.run(ctx); err != nil {
            log.Printf("Connection lost: %v", err)
        }
    }
}

func (c *Client) connectOnce(ctx context.Context) error {
    conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.hubURL+"/agent", nil)
    if err != nil {
        return err
    }
    c.conn = conn
    
    // 发送注册消息
    return c.conn.WriteJSON(AgentMessage{
        Type: TypeAgentRegister,
        Payload: mustMarshal(RegisterPayload{
            AgentID:      c.agentID,
            Token:        c.token,
            Capabilities: []string{"opencode"},
            Version:      "0.1.0",
        }),
    })
}

func (c *Client) run(ctx context.Context) error {
    for {
        var msg AgentMessage
        if err := c.conn.ReadJSON(&msg); err != nil {
            return err
        }
        
        switch msg.Type {
        case TypeAgentPing:
            c.conn.WriteJSON(AgentMessage{Type: TypeAgentPong})
            
        case TypeAgentRequest:
            go c.handleRequest(ctx, msg)
        }
    }
}

func (c *Client) handleRequest(ctx context.Context, msg AgentMessage) {
    var req RequestPayload
    json.Unmarshal(msg.Payload, &req)
    
    // 调用处理器
    streamCh, err := c.handler(ctx, &req)
    if err != nil {
        c.sendError(msg.ID, err.Error())
        return
    }
    
    // 转发流式响应
    for chunk := range streamCh {
        c.conn.WriteJSON(AgentMessage{
            Type: TypeAgentStream,
            ID:   msg.ID,
            Payload: mustMarshal(chunk),
        })
    }
}
```

## 安全考虑

### 认证方式

| 方式 | 安全性 | 复杂度 | 选择 |
|------|--------|--------|------|
| 预共享密钥 | 中 | 低 | Phase 2 ✓ |
| JWT Token | 高 | 中 | Phase 3 |
| mTLS | 很高 | 高 | 未来 |

### Phase 2 实现

```go
// 简单的预共享密钥认证
func (m *Manager) waitForRegister(agent *AgentConn) error {
    var msg AgentMessage
    if err := agent.Conn.ReadJSON(&msg); err != nil {
        return err
    }
    
    if msg.Type != TypeAgentRegister {
        return ErrInvalidMessage
    }
    
    var payload RegisterPayload
    json.Unmarshal(msg.Payload, &payload)
    
    // 验证 token
    if subtle.ConstantTimeCompare([]byte(payload.Token), []byte(m.config.AuthToken)) != 1 {
        return ErrUnauthorized
    }
    
    agent.ID = payload.AgentID
    agent.Capabilities = payload.Capabilities
    
    // 发送注册成功
    return agent.Conn.WriteJSON(AgentMessage{
        Type: TypeAgentRegistered,
        Payload: mustMarshal(map[string]interface{}{
            "success": true,
        }),
    })
}
```

## 心跳保活

```
Hub                    Agent
 |                       |
 |--- ping (30s) ------->|
 |<-- pong --------------|
 |                       |
 |--- ping (30s) ------->|
 |    (no pong in 10s)   |
 |                       |
 |--- close connection --|
 |                       |
```

## 故障处理

| 场景 | Hub 行为 | Agent 行为 |
|------|----------|------------|
| Agent 断连 | 从列表移除，请求失败返回错误 | 自动重连 |
| Hub 断连 | N/A | 自动重连 |
| 请求超时 | 关闭响应通道 | 取消处理 |
| Agent 无响应 | 心跳超时断连 | N/A |

## 测试计划

1. **连接测试**: 正常连接、认证失败、重连
2. **消息测试**: 请求转发、流式响应、错误处理
3. **故障测试**: 网络断开、心跳超时、Agent 崩溃
4. **并发测试**: 多 Agent、多请求
