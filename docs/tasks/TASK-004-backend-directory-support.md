# TASK-004: 后端支持 session.create 的 directory 参数

## 问题描述

前端发送 `session.create` 时带了 `directory` 参数，但后端没有正确处理，导致会话创建在错误的项目下。

## 测试结果

```bash
# 发送 session.create 带 directory
ws.send(JSON.stringify({
  type: 'session.create',
  payload: { 
    title: 'Test Chat', 
    directory: '/home/zcy/workspace/projects/Game2048' 
  }
}));

# 返回结果
{ directory: '/home/zcy/workspace/projects/SmartQuant' }  # 错误！应该是 Game2048
```

## 根因分析

### Hub 问题

`hub/internal/server/server.go` 第 284-291 行：
```go
func (c *Client) handleSessionCreate(requestID string, title string) {
    // ...
    data, _ := json.Marshal(map[string]string{"title": title})  // 只传了 title！
```

- `handleSessionCreate` 只接收 `title`，没有 `directory`
- 转发给 Agent 时丢失了 `directory`

### Agent 问题

`agent/internal/tunnel/client.go` 第 259-278 行：
```go
func (c *Client) handleOpenCodeRequest(..., req RequestPayload) {
    if c.projectMgr != nil && req.ProjectPath != "" {  // ProjectPath 为空时使用默认 URL
        url, err := c.projectMgr.GetOpenCodeURL(req.ProjectPath)
        ...
    }
```

- `req.ProjectPath` 从 Hub 传来，但 Hub 没有设置

## 修复方案

### 1. Hub: 修改 SessionPayload 结构体

```go
type SessionPayload struct {
    SessionID string `json:"sessionId,omitempty"`
    Title     string `json:"title,omitempty"`
    Directory string `json:"directory,omitempty"`  // 新增
}
```

### 2. Hub: 修改 handleMessage 的 session.create case

```go
case "session.create":
    var payload SessionPayload
    if err := json.Unmarshal(msg.Payload, &payload); err != nil {
        c.sendError(msg.ID, "Invalid payload format")
        return
    }
    c.handleSessionCreate(msg.ID, payload.Title, payload.Directory)  // 传递 directory
```

### 3. Hub: 修改 handleSessionCreate 函数

```go
func (c *Client) handleSessionCreate(requestID string, title string, directory string) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if agent, ok := c.server.tunnelMgr.GetAnyAgent(); ok {
        data, _ := json.Marshal(map[string]string{
            "title":     title,
            "directory": directory,  // 新增
        })
        // 使用 directory 作为 sessionID 参数来传递项目路径
        c.handleViaAgent(ctx, requestID, agent.ID, "session.create", directory, data)
        return
    }
    // ... 其余代码
}
```

### 4. Hub: 修改 handleViaAgent 使用 projectPath

查看 tunnel.RequestPayload 的定义，它有 `ProjectPath` 字段。需要确保这个字段被正确设置。

```go
func (c *Client) handleViaAgent(ctx context.Context, requestID, agentID, action string, projectPath string, data json.RawMessage) {
    req := &tunnel.RequestPayload{
        SessionID:   "",  // 对于 session.create，sessionID 为空
        Action:      action,
        Data:        data,
        ProjectPath: projectPath,  // 设置项目路径
    }
    // ...
}
```

### 5. Agent: 修改 opencode/client.go 支持 directory

```go
type SessionCreateData struct {
    Title     string `json:"title"`
    Directory string `json:"directory,omitempty"`  // 新增
}

func (c *Client) handleSessionCreate(ctx context.Context, baseURL string, data json.RawMessage, ch chan<- []byte) {
    var createData SessionCreateData
    json.Unmarshal(data, &createData)

    body, _ := json.Marshal(map[string]string{
        "title": createData.Title,
    })
    // ... 创建会话 ...

    // 在响应中添加 directory
    var respData map[string]interface{}
    json.Unmarshal(respBody, &respData)
    if createData.Directory != "" {
        respData["directory"] = createData.Directory
    }
    modifiedResp, _ := json.Marshal(respData)
    ch <- modifiedResp
}
```

## 交付要求

### 需要修改的文件

1. `hub/internal/server/server.go`
2. `agent/internal/opencode/client.go`

### 验收标准

1. `session.create` 带 `directory` 参数时，返回的 `directory` 匹配请求
2. 在该项目下创建的会话使用正确的 OpenCode 实例
3. `go build ./cmd/hub` 和 `go build ./cmd/agent` 成功

### 测试方法

```bash
# 启动 Game2048 项目
ws.send({ type: 'project.start', payload: { path: '/home/.../Game2048' } })

# 创建会话
ws.send({ 
  type: 'session.create', 
  payload: { title: 'Test', directory: '/home/.../Game2048' }
})

# 预期响应
{ directory: '/home/.../Game2048' }  # 应该匹配请求
```
