# 🌌 OpenVibe

**Mobile Vibe Coding Terminal** - 用手机和 AI 一起编程

> Code at the speed of thought, secure by design.

---

## 📖 项目简介

OpenVibe 是一个移动端 AI 编程终端，让你可以：
- 📱 在手机上用自然语言和 AI 对话编程
- 🖥️ 远程控制你的开发服务器
- 🔒 安全地访问你的代码（TLS 加密）

## 🏗️ 架构

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   手机 App   │────▶│  云端 Hub    │────▶│  开发服务器  │
│  (Next.js)   │◀────│  (Go Proxy)  │◀────│  (OpenCode)  │
└──────────────┘     └──────────────┘     └──────────────┘
    Capacitor          WebSocket            HTTP API
    iOS/Android        Token 认证           SSE 流式响应
```

| 组件 | 技术栈 | 目录 |
|------|--------|------|
| 手机 App | Next.js 14, TypeScript, Capacitor | `/app` |
| 云端 Hub | Go 1.22, gorilla/websocket | `/hub` |
| 开发服务器 | OpenCode (官方 `opencode serve`) | - |

---

## 🏁 验收节点 (Checkpoints)

### CP1: 链路打通 ⚡
**目标**: 证明整条技术链路是通的

| 验收项 | 标准 |
|--------|------|
| Hub 运行 | 华为云 8080 端口可访问 |
| OpenCode 运行 | Arch 服务器 4096 端口监听 |
| 端到端测试 | 发消息能收到 AI 回复 |

**验收动作**:
```bash
./scripts/test-connection.sh "你好"
# 预期: 收到 AI 回复
```

---

### CP2: 网页可用 🌐
**目标**: 浏览器可以正常使用

| 验收项 | 标准 |
|--------|------|
| 访问网页 | http://121.36.218.61:8080 能打开 |
| 发送消息 | 输入文字，点击发送 |
| 流式回复 | 看到打字机效果 |
| 多轮对话 | 连续对话，上下文保持 |

**验收动作**:
1. 浏览器打开 `http://121.36.218.61:8080`
2. 输入「帮我写一个 Python hello world」
3. 观察流式回复
4. 继续对话，验证上下文

---

### CP3: 手机可用 📱
**目标**: 手机浏览器体验良好

| 验收项 | 标准 |
|--------|------|
| iPhone Safari | 界面正常，能对话 |
| Android Chrome | 界面正常，能对话 |
| 输入法 | 弹出时界面不错乱 |
| 触摸 | 滚动、点击响应正常 |

**验收动作**:
1. 手机打开 `http://121.36.218.61:8080`
2. 完成一次完整对话
3. 测试输入法弹出/收起

---

### CP4: App 完成 📲
**目标**: 可安装的原生 App

| 验收项 | 标准 |
|--------|------|
| iOS 安装 | TestFlight 或开发模式安装成功 |
| Android 安装 | APK 安装成功 |
| 对话功能 | 完整可用 |
| 历史记录 | 重启 App 后保留 |
| 断网重连 | 网络恢复后自动重连 |

**验收动作**:
1. 安装 App
2. 配置服务器地址
3. 完成对话测试
4. 杀掉 App 重开，检查历史
5. 开关飞行模式，测试重连

---

## 📅 开发时间线

```
Week 1
├── Day 1-2: 项目骨架 + 基础代码
├── Day 3: 【CP1 验收】链路打通
├── Day 4-6: 网页 UI 开发
└── Day 7: 【CP2 验收】网页可用

Week 2
├── Day 8-9: 手机适配
├── Day 10: 【CP3 验收】手机可用
├── Day 11-13: Capacitor 打包
└── Day 14: 【CP4 验收】App 完成
```

---

## 🚀 快速开始

### 前置条件

- Arch 服务器: 安装 OpenCode (`opencode --version`)
- 华为云: 8080 端口开放
- 开发机: Node.js 18+, Go 1.22+

### 启动服务

```bash
# 1. Arch 服务器 - 启动 OpenCode
sudo systemctl start opencode

# 2. 华为云 - 启动 Hub
cd hub && ./openvibe-hub

# 3. 访问
open http://121.36.218.61:8080
```

---

## 📁 项目结构

```
OpenVibe/
├── app/                 # Next.js 手机 App
│   ├── src/
│   └── capacitor.config.ts
├── hub/                 # Go 中转服务
│   ├── cmd/hub/
│   └── internal/
├── docs/                # 文档
│   ├── ARCHITECTURE.md
│   └── API.md
├── scripts/             # 部署和测试脚本
├── init.md              # 原始需求文档
├── AGENTS.md            # AI 开发指南
└── README.md            # 本文件
```

---

## 📜 License

MIT

---

## 🔗 相关文档

- [架构设计](./docs/ARCHITECTURE.md)
- [API 文档](./docs/API.md)
- [部署指南](./docs/DEPLOYMENT.md)
- [AI 开发指南](./AGENTS.md)
