# Phase 2: Architecture Refactoring (The Go Core)

> **Status**: In Progress  
> **Goal**: 引入高性能组件，实现断点续传和反向隧道

## 目标

1. **Redis 消息缓冲** - Mosh 风格断点续传，支持移动网络频繁断连
2. **Host Agent** - 运行在用户服务器的 Go 程序，管理 OpenCode 进程
3. **反向隧道** - Agent 主动连接 Hub，穿透 NAT/防火墙

## 架构变更

### Phase 1 (当前)
```
App ←--WS--→ Hub ←--HTTP--→ OpenCode (同机部署)
```

### Phase 2 (目标)
```
App ←--WS--→ Hub ←--WS Tunnel--→ Agent ←--HTTP--→ OpenCode
                ↓
              Redis (消息缓冲)
```

## 核心组件

| 组件 | 目录 | 职责 |
|------|------|------|
| Hub Server | `/hub` | WebSocket 网关 + 消息路由 + Redis 缓冲 |
| Host Agent | `/agent` | 反向隧道客户端 + OpenCode 代理 |
| Message Buffer | `/hub/internal/buffer` | Redis 环形队列实现 |
| Tunnel Manager | `/hub/internal/tunnel` | Agent 连接管理 |

## 消息流

### 发送消息 (App → Agent)
```
1. App 发送 prompt 到 Hub
2. Hub 生成递增 msgID，存入 Redis
3. Hub 通过 tunnel 转发到 Agent
4. Agent 调用 OpenCode API
5. 响应通过 tunnel 返回 Hub
6. Hub 存入 Redis，推送给 App
7. App 确认收到 (ack msgID)
```

### 断点续传 (重连后)
```
1. App 重连，发送 lastAckID = 1000
2. Hub 查询 Redis: msgID > 1000
3. Hub 批量推送缺失消息 [1001, 1002, ...]
4. App 渲染，无缝衔接
```

## 协议设计

### 新增消息类型

```typescript
// App → Hub
interface SyncRequest {
  type: 'sync';
  id: string;
  payload: {
    lastAckID: number;  // 最后确认的消息 ID
    sessionId: string;
  };
}

// Hub → App
interface SyncResponse {
  type: 'sync.batch';
  id: string;
  payload: {
    messages: BufferedMessage[];
    latestID: number;
  };
}

// App → Hub (确认收到)
interface AckMessage {
  type: 'ack';
  id: string;
  payload: {
    msgID: number;
  };
}
```

### Agent 协议

```typescript
// Agent → Hub (注册)
interface AgentRegister {
  type: 'agent.register';
  payload: {
    agentId: string;
    token: string;      // 预共享密钥
    capabilities: string[];
  };
}

// Hub → Agent (转发请求)
interface AgentRequest {
  type: 'agent.request';
  id: string;
  payload: {
    sessionId: string;
    action: 'prompt' | 'session.create' | 'session.list';
    data: any;
  };
}

// Agent → Hub (响应)
interface AgentResponse {
  type: 'agent.response';
  id: string;
  payload: any;
}
```

## Redis 数据结构

```
# 消息缓冲 (每个 session 一个 list)
openvibe:session:{sessionId}:messages -> LIST [msg1, msg2, ...]

# 消息 ID 计数器
openvibe:session:{sessionId}:msgid -> INT (递增)

# Agent 注册信息
openvibe:agent:{agentId} -> HASH { status, lastSeen, capabilities }

# Session 到 Agent 的映射
openvibe:session:{sessionId}:agent -> STRING agentId
```

## 文件结构规划

```
hub/
├── cmd/hub/main.go
├── internal/
│   ├── config/config.go
│   ├── server/server.go      # 现有，需修改
│   ├── proxy/proxy.go        # 现有，Phase 2 弃用
│   ├── buffer/               # 新增
│   │   ├── buffer.go         # 消息缓冲接口
│   │   └── redis.go          # Redis 实现
│   └── tunnel/               # 新增
│       ├── manager.go        # Agent 连接管理
│       └── protocol.go       # 消息协议定义
└── go.mod

agent/
├── cmd/agent/main.go         # 入口
├── internal/
│   ├── config/config.go      # 配置
│   ├── tunnel/tunnel.go      # 隧道客户端
│   └── opencode/client.go    # OpenCode HTTP 客户端
└── go.mod
```

## 实施步骤

### Step 1: 消息缓冲层 (2-3h)
- [ ] 添加 go-redis 依赖
- [ ] 实现 `buffer.Buffer` 接口
- [ ] 实现 Redis 环形队列
- [ ] 集成到 server.go

### Step 2: Hub 隧道管理 (2-3h)
- [ ] 创建 `tunnel.Manager`
- [ ] Agent WebSocket 端点 `/agent`
- [ ] 消息路由逻辑

### Step 3: Host Agent (3-4h)
- [ ] 创建 agent 模块骨架
- [ ] 实现隧道客户端
- [ ] 实现 OpenCode 代理
- [ ] 心跳保活机制

### Step 4: App 适配 (1-2h)
- [ ] 添加 lastAckID 跟踪
- [ ] 实现 sync 协议
- [ ] 处理 sync.batch 消息

### Step 5: 集成测试 (1-2h)
- [ ] 端到端测试脚本
- [ ] 断连重连测试
- [ ] 性能基准测试

## 验收标准

1. **断点续传**: 断网 30s 后重连，消息无丢失
2. **反向隧道**: Agent 在 NAT 后可正常工作
3. **性能**: 消息延迟 < 100ms (不含 AI 响应时间)
4. **可靠性**: Redis 宕机时优雅降级到直连模式

## 依赖

```go
// hub/go.mod 新增
require (
    github.com/redis/go-redis/v9 v9.x.x
)
```

## 相关文档

- [BUFFER.md](./BUFFER.md) - 消息缓冲详细设计
- [TUNNEL.md](./TUNNEL.md) - 隧道协议详细设计
- [AGENT.md](./AGENT.md) - Host Agent 详细设计
