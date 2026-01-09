# 🌌 OpenVibe: The Mobile Vibe Deck
**产品与技术架构白皮书 (v3.0)**

---

## 0. 执行摘要 (Executive Summary)

### 0.1 产品愿景
**OpenVibe** 是一个专为 AI 时代打造的移动端编程终端。它旨在让开发者脱离笨重的桌面 IDE，通过自然语言与强大的 AI Agent (OpenCode/Claude) 协作，利用移动设备实现“心流式编程” (Vibe Coding)。

### 0.2 核心目标
1.  **体验革命**: 解决传统移动端 SSH“打字难、连接脆、无预览”的痛点，提供类似 IM 聊天般的流畅体验。
2.  **算力解放**: 实现 **BYOS (Bring Your Own Server)**，让手机成为轻量级指挥官，远程驾驭用户自有的高性能工作站（Arch Linux/4090 GPU）。
3.  **零信任安全**: 建立商业级的安全壁垒，平台方（OpenVibe SaaS）无法解密用户的代码或指令。

### 0.3 关键技术支柱
* **Blind Relay (盲台中转)**: 基于 Go 的高性能 WebSocket 中转服务，仅转发加密数据包。
* **E2EE (端到端加密)**: 结合 ECDH 密钥交换与 AES-256-GCM，确保通信绝对隐私。
* **Mosh-Style Sync (状态同步)**: 摒弃传统 TCP 流，采用状态同步机制，容忍移动网络高延迟和频繁切换。
* **Contextual UI (情境界面)**: 动态宏按钮替代全键盘输入，智能折叠长代码与报错信息。

---

## 1. 产品定义与交互策略 (Product & UX)

本章节汲取了 Replit (成功点：一键运行)、Termius (成功点：辅助按键) 和传统 SSH (失败点：连接脆弱、全屏红字) 的经验教训。

### 1.1 命名与定位
* **名称**: **OpenVibe** (代号: Mobile Vibe Deck / MVD)
* **Slogan**: Code at the speed of thought, secure by design.
* **定位**: 介于“聊天软件”与“终端模拟器”之间的新物种。

### 1.2 解决的历史痛点
1.  **“拇指痛点” (The Fat Finger)**: 手机输入复杂 shell 命令极其痛苦。
    * *OpenVibe 方案*: **Contextual Macro Deck (动态宏键盘)**。根据 Agent 状态（空闲、询问、运行中、报错）自动切换底部按钮（如 `[Deploy]`, `[Reject]`, `[Auto Fix]`）。
2.  **“连接焦虑” (Connection Anxiety)**: 进电梯/切 WiFi 导致 SSH 断连。
    * *OpenVibe 方案*: **Shadow State Sync**。服务端缓存状态，断网重连后像看视频一样“快进”补全内容，永不掉线。
3.  **“盲人摸象” (Blind Coding)**: 无法预览生成的网页或图片。
    * *OpenVibe 方案*: **Artifact Cards**。文件变动自动触发预览卡片，点击即看。

### 1.3 视觉风格：Cyberpunk Console
* **配色**: 深空灰 (`#09090b`) 背景，霓虹绿 (`#00ff9d`) 主色，赛博红 (`#ff0055`) 警告色。
* **沉浸感**: 无滚动条 PWA 设计，全屏显示，触觉反馈（Haptics）模拟打字机震动。

---

## 2. 系统架构 (System Architecture)

采用 **Hub-and-Spoke (中枢-辐射)** 拓扑结构，确保 SaaS 商业模式的可行性与用户数据主权。

### 2.1 架构拓扑图

```mermaid
graph TD
    subgraph Mobile_Client [📱 客户端层 (The Deck)]
        NextJS[Next.js PWA]
        Capacitor[Native Shell]
        UI_Render[Markdown/ANSI Renderer]
        Crypto_App[WebCrypto Decryption]
    end

    subgraph Cloud_Hub [☁️ 云信令层 (The Blind Hub)]
        Relay[Go Relay Server]
        Redis[Session Buffer (Ring Queue)]
        Auth[Subscription DB]
    end

    subgraph User_Edge [🖥️ 边缘计算层 (The Host)]
        Agent[OpenVibe Agent (Go Binary)]
        Crypto_Edge[Go Crypto Encryption]
        Core[OpenCode / Claude CLI]
        FS_Watch[File System Watcher]
    end

    %% Data Flow
    NextJS <-->|"Encrypted Sync Stream"| Relay
    Relay <-->|"Reverse Tunnel"| Agent
    Agent <-->|"PTY Control"| Core
```

### 2.2 核心组件职责

1.  **The Deck (App)**: 负责密钥管理、UI 渲染、断点续传逻辑。
2.  **The Blind Hub (Server)**:
    * **职责**: 仅仅是一个“不知疲倦的哑巴邮差”。
    * **特性**: 不解密数据，只验证 Session ID 和订阅状态。维护 Redis 环形缓冲区以支持 Mosh 风格重连。
3.  **The Host Agent (Edge)**:
    * **职责**: 运行在用户 Arch Linux 上的 Go 程序。
    * **特性**: **开源 (Open Source)** 以建立信任。负责反向穿透 NAT，管理 OpenCode 进程，监控文件变化。

---

## 3. 安全架构：零信任体系 (Security Architecture)

这是商业化成功的基石。必须保证“即使 OpenVibe 公司倒闭或被黑，用户的代码依然安全”。

### 3.1 信任锚点：物理握手
* **机制**: **Air-Gapped Pairing (物理隔绝配对)**。
* **流程**:
    1.  电脑端 Agent 启动，生成一次性公钥，屏幕显示 QR 码。
    2.  手机 App 扫描 QR 码（不经过网络），获取电脑公钥。
    3.  此后所有通过网络的通信，均使用该公钥派生的密钥加密。

### 3.2 传输协议：E2EE (端到端加密)
* **算法组合**:
    * **密钥交换**: X25519 (ECDH)。
    * **负载加密**: AES-256-GCM。
    * **防重放**: 每个数据包包含递增 Nonce。
* **数据流**: `App (加密) -> Cloud (盲转) -> Agent (解密) -> Shell`。

---

## 4. 关键功能规格 (Detailed Specifications)

### 4.1 智能流与上下文 (Smart Stream)
* **混合渲染**: 将 ANSI 转义序列（终端颜色）转换为 HTML span，同时支持 Markdown 渲染。
* **错误折叠**: 使用正则匹配 Python/C++ Traceback，将其折叠为红色卡片 `[ 🚨 Error: Segmentation Fault ]`，避免刷屏。
* **Project Context**: 侧边栏文件树。长按文件 -> "Add to Context"，下次发指令自动携带该文件内容。

### 4.2 产物预览管线 (Artifact Pipeline)
1.  **图片/PDF**:
    * Agent 监控到新文件 -> 读取二进制 -> 分块加密 -> 发送。
    * App 接收 -> 内存解密 -> 组装为 Blob URL -> `<img>` 标签展示。
    * *注*: App 关闭后内存释放，不在手机留痕。
2.  **Web 应用 (Tunneling)**:
    * Agent 检测到端口监听 (e.g., :3000) -> 通知 Cloud。
    * Cloud 开启临时鉴权隧道 -> App 内嵌 WebView 访问。

### 4.3 Mosh 风格重连机制
* **服务端**: Redis 记录最近 5 分钟的数据包队列 `[Msg_1001, Msg_1002, ... Msg_1050]`。
* **客户端**: 维护 `Last_Ack_ID = 1000`。
* **重连**:
    * App: "我回来了，我上次看到 1000"。
    * Server: "给，这是 1001-1050"。
    * App: 瞬间渲染 50 条消息，无缝衔接。

---

## 5. 实施路线图 (Implementation Roadmap)

### Phase 1: 原型机 (The Prototype)
* **目标**: 验证核心链路。
* **技术**: Next.js (App) + Node.js (Server & Agent)。
* **限制**: 无 E2EE，仅 TLS；无 Mosh 缓存；仅支持局域网或简单 FRP。
* **交付物**: 能在手机网页上打字，控制电脑终端，看到回显。

### Phase 2: 架构重构 (The Go Core)
* **目标**: 引入高性能组件。
* **动作**:
    * 用 **Go** 重写 Agent 和 Relay Server。
    * 引入 **Redis** 实现消息缓冲。
    * 实现 **WebSocket 反向隧道** (彻底抛弃 FRP)。

### Phase 3: 安全堡垒 (Zero Trust)
* **目标**: 商业化准备。
* **动作**:
    * 实现 ECDH + AES-GCM 加密层。
    * 开发 QR 码握手流程。
    * **开源 Host Agent 代码**。

### Phase 4: 移动端打磨 (Mobile Polish)
* **目标**: 上架 App Store。
* **动作**:
    * 使用 **Capacitor** 打包 iOS/Android 应用。
    * 实现 **Contextual Macro Deck** (动态按钮)。
    * 适配 iOS 安全区域与触觉反馈。