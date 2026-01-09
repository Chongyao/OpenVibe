# OpenVibe 架构设计

## 1. 系统概览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           OpenVibe System                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────┐  │
│  │   Mobile     │      │    Cloud     │      │    Dev Server        │  │
│  │   App        │◀────▶│    Hub       │◀────▶│    (Arch Linux)      │  │
│  │              │      │              │      │                      │  │
│  │  - Next.js   │      │  - Go        │      │  - opencode serve    │  │
│  │  - Capacitor │      │  - WebSocket │      │  - HTTP API :4096    │  │
│  │  - iOS/Droid │      │  - Auth      │      │  - SSE streaming     │  │
│  └──────────────┘      └──────────────┘      └──────────────────────┘  │
│         │                     │                        │                │
│    Capacitor             Port 8080               Port 4096             │
│    Native Shell          Public IP              localhost              │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## 2. 组件职责

### 2.1 Mobile App (`/app`)

**技术栈**: Next.js 14 + TypeScript + TailwindCSS + Capacitor

**职责**:
- 用户界面渲染（聊天界面）
- WebSocket 连接管理
- 消息发送/接收
- 会话历史本地存储
- 断线重连逻辑

**关键模块**:
```
app/src/
├── components/          # UI 组件
│   ├── ChatView.tsx     # 聊天主界面
│   ├── MessageBubble.tsx # 消息气泡
│   ├── InputBar.tsx     # 输入栏
│   └── SessionList.tsx  # 会话列表
├── hooks/               # React Hooks
│   ├── useWebSocket.ts  # WebSocket 管理
│   ├── useSession.ts    # 会话状态
│   └── useMessages.ts   # 消息管理
├── lib/                 # 工具库
│   ├── ws-client.ts     # WebSocket 客户端
│   ├── storage.ts       # 本地存储
│   └── api.ts           # API 类型定义
└── app/                 # Next.js 路由
    ├── page.tsx         # 主页面
    └── layout.tsx       # 布局
```

### 2.2 Cloud Hub (`/hub`)

**技术栈**: Go 1.22 + gorilla/websocket

**职责**:
- 接收 App 的 WebSocket 连接
- Token 认证
- 转发请求到 OpenCode API
- 将 SSE 响应转换为 WebSocket 消息
- 连接池管理

**关键模块**:
```
hub/
├── cmd/hub/
│   └── main.go          # 入口
├── internal/
│   ├── server/
│   │   └── server.go    # WebSocket 服务器
│   ├── proxy/
│   │   └── proxy.go     # OpenCode API 代理
│   ├── auth/
│   │   └── auth.go      # Token 认证
│   └── config/
│       └── config.go    # 配置管理
└── go.mod
```

### 2.3 Dev Server (OpenCode)

**技术**: 官方 `opencode serve` 命令

**职责**:
- 运行 AI Agent
- 提供 HTTP API
- SSE 事件流
- 会话管理

**API 端点** (由 OpenCode 提供):
| 端点 | 方法 | 用途 |
|------|------|------|
| `/session` | GET | 列出会话 |
| `/session` | POST | 创建会话 |
| `/session/:id/message` | POST | 发送消息 |
| `/session/:id/message` | GET | 获取消息历史 |
| `/event` | GET | SSE 事件订阅 |

## 3. 通信协议

### 3.1 App ↔ Hub (WebSocket)

**连接**: `ws://hub-ip:8080/ws?token=xxx`

**消息格式**:
```typescript
// Client → Server
interface ClientMessage {
  type: 'prompt' | 'session.create' | 'session.list' | 'ping';
  payload: {
    sessionId?: string;
    content?: string;
    model?: { providerId: string; modelId: string };
  };
  id: string; // 用于匹配响应
}

// Server → Client
interface ServerMessage {
  type: 'response' | 'stream' | 'error' | 'pong' | 'event';
  payload: any;
  id?: string; // 对应请求 ID
}
```

### 3.2 Hub ↔ OpenCode (HTTP + SSE)

**请求**: 标准 HTTP POST/GET

**响应**: SSE 流式输出

```
Hub 收到 App 消息
    ↓
POST /session/:id/message
    ↓
OpenCode 返回 SSE 流
    ↓
Hub 逐条转发给 App (WebSocket)
```

## 4. 认证机制 (MVP 阶段)

### 4.1 Token 认证

简单的静态 Token 认证，后续可升级。

```
1. 服务器生成 Token (首次运行时)
2. 用户在 App 中输入 Token
3. App 连接时携带 Token
4. Hub 验证 Token
```

**Token 存储**:
- Hub: 环境变量 `OPENVIBE_TOKEN`
- App: 本地存储 (SecureStorage)

### 4.2 TLS (可选)

如果有域名，使用 Let's Encrypt 配置 HTTPS。
MVP 阶段先用 HTTP。

## 5. 数据流

### 5.1 发送消息

```
User Input
    │
    ▼
┌─────────┐  WebSocket   ┌─────────┐   HTTP POST   ┌─────────────┐
│   App   │─────────────▶│   Hub   │──────────────▶│  OpenCode   │
└─────────┘              └─────────┘               └─────────────┘
```

### 5.2 接收回复 (流式)

```
┌─────────────┐   SSE Stream   ┌─────────┐  WebSocket  ┌─────────┐
│  OpenCode   │───────────────▶│   Hub   │────────────▶│   App   │
└─────────────┘                └─────────┘             └─────────┘
    │                              │                       │
    │  event: message.part         │  {type: "stream"}     │
    │  data: {"content": "Hello"}  │                       │
    │                              │                       │
    │  event: message.part         │  {type: "stream"}     │
    │  data: {"content": " World"} │                       │
```

## 6. 错误处理

### 6.1 连接错误

| 场景 | App 行为 |
|------|----------|
| Hub 不可达 | 显示错误，3秒后重试 |
| Token 无效 | 提示重新输入 Token |
| OpenCode 离线 | Hub 返回 503，App 显示服务不可用 |

### 6.2 重连策略

```typescript
const RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 30000];
// 指数退避，最大 30 秒
```

## 7. 部署架构

```
┌─────────────────────────────────────────────────────────┐
│                    华为云 (121.36.218.61)               │
│  ┌─────────────────────────────────────────────────┐   │
│  │  Hub (Go Binary)                                 │   │
│  │  - 监听 0.0.0.0:8080                            │   │
│  │  - systemd 管理                                  │   │
│  │  - 日志: /var/log/openvibe/hub.log             │   │
│  └─────────────────────────────────────────────────┘   │
│                         │                               │
│                         │ HTTP (内网穿透/SSH隧道)       │
│                         ▼                               │
└─────────────────────────────────────────────────────────┘
                          │
                          │ SSH Tunnel (localhost:4096)
                          ▼
┌─────────────────────────────────────────────────────────┐
│                    Arch 服务器                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │  opencode serve                                  │   │
│  │  - 监听 127.0.0.1:4096                          │   │
│  │  - systemd 管理                                  │   │
│  │  - 工作目录: /home/zcy/workspace                │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 7.1 SSH 隧道 (Arch → 华为云)

```bash
# 在 Arch 服务器上建立反向隧道
ssh -R 4096:localhost:4096 huawei -N
```

这样 Hub 可以通过 `localhost:4096` 访问 OpenCode。

## 8. 未来扩展点

### 8.1 E2EE (Phase 2)
- 增加 Go Agent 在 Arch 端
- X25519 密钥交换
- AES-256-GCM 加密
- Hub 变成完全盲转

### 8.2 多 Host 支持
- 一个 App 连接多台开发机
- Hub 维护 Host 路由表

### 8.3 Macro Deck
- 动态按钮配置
- 常用命令快捷键

### 8.4 文件预览
- 图片/PDF 传输
- Web 应用隧道预览
