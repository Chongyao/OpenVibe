# TASK-002: Hub 添加 project.* 消息处理

## 问题描述

Hub 的 `handleMessage` 函数没有处理 `project.list`、`project.start`、`project.stop` 消息类型，导致前端发送这些消息时收到 "Unknown message type" 错误。

## 技术分析

### 现有处理模式

参考 `session.list` 的处理方式 (第 193-194 行):
```go
case "session.list":
    c.handleSessionList(msg.ID)
```

`handleSessionList` 使用 `handleViaAgent` 转发请求到 Agent:
```go
if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
    c.handleViaAgent(ctx, requestID, agent.ID, "session.list", "", nil)
    return
}
```

### 需要添加的处理

project.* 消息需要类似地转发到 Agent:

```go
case "project.list":
    c.handleProjectList(msg.ID)

case "project.start":
    c.handleProjectAction(msg.ID, "project.start", msg.Payload)

case "project.stop":
    c.handleProjectAction(msg.ID, "project.stop", msg.Payload)
```

## 修复方案

### 1. 在 handleMessage 的 switch 中添加 case (第 189-247 行之间):

```go
case "project.list":
    c.handleProjectList(msg.ID)

case "project.start", "project.stop":
    c.handleProjectAction(msg.ID, msg.Type, msg.Payload)
```

### 2. 添加 handleProjectList 函数:

```go
func (c *Client) handleProjectList(requestID string) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
        c.handleViaAgent(ctx, requestID, agent.ID, "project.list", "", nil)
        return
    }

    c.sendError(requestID, "No agent connected. Please start the OpenVibe agent on your development server.")
}
```

### 3. 添加 handleProjectAction 函数:

```go
func (c *Client) handleProjectAction(requestID string, action string, payload json.RawMessage) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
        c.handleViaAgent(ctx, requestID, agent.ID, action, "", payload)
        return
    }

    c.sendError(requestID, "No agent connected. Please start the OpenVibe agent on your development server.")
}
```

## 交付要求

### 需要修改的文件

- `hub/internal/server/server.go`

### 验收标准

1. `go build ./cmd/hub` 编译成功
2. `go test ./...` 测试通过
3. WebSocket 发送 `project.list` 能收到正确响应

### 修改限制

- 不要修改 Agent 代码
- 不要修改前端代码
- 保持现有的 API 接口不变
- 遵循现有代码风格

## 测试方法

```bash
cd /home/zcy/workspace/projects/OpenVibe/app && node -e "
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8080/ws');

ws.on('open', () => {
  console.log('Connected!');
  ws.send(JSON.stringify({
    type: 'project.list',
    id: 'test-1',
    payload: {}
  }));
});

ws.on('message', (data) => {
  console.log('Received:', JSON.parse(data.toString()));
  ws.close();
});
"
```

预期响应:
```json
{
  "type": "response",
  "id": "test-1",
  "payload": {
    "projects": [...]
  }
}
```
