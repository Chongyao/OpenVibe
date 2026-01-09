# OpenVibe 部署指南

## 系统架构

```
┌─────────────────┐     SSH Tunnel      ┌─────────────────┐
│   Arch 服务器   │◀───────────────────▶│    华为云       │
│  (OpenCode)     │    Port 4096        │    (Hub)        │
│  222.205.46.147 │                     │  121.36.218.61  │
└─────────────────┘                     └─────────────────┘
        │                                       │
   opencode serve                          openvibe-hub
   localhost:4096                          0.0.0.0:8080
                                                │
                                          ┌─────▼─────┐
                                          │  手机 App │
                                          └───────────┘
```

## 部署步骤

### 1. Arch 服务器配置

#### 1.1 安装 OpenCode systemd 服务

```bash
# 复制服务文件
sudo cp scripts/opencode.service /etc/systemd/system/

# 重新加载 systemd
sudo systemctl daemon-reload

# 启用并启动服务
sudo systemctl enable opencode
sudo systemctl start opencode

# 检查状态
sudo systemctl status opencode
```

#### 1.2 安装 SSH 隧道服务

```bash
# 复制服务文件
sudo cp scripts/openvibe-tunnel.service /etc/systemd/system/

# 重新加载 systemd
sudo systemctl daemon-reload

# 启用并启动服务
sudo systemctl enable openvibe-tunnel
sudo systemctl start openvibe-tunnel

# 检查状态
sudo systemctl status openvibe-tunnel
```

#### 1.3 验证 OpenCode 运行

```bash
# 检查端口
ss -tlnp | grep 4096

# 测试 API
curl http://localhost:4096/global/health
```

### 2. 华为云服务器配置

#### 2.1 一键部署（推荐）

在 Arch 服务器上运行：

```bash
cd /home/zcy/workspace/projects/OpenVibe
./scripts/deploy-hub.sh
```

#### 2.2 手动部署

```bash
# 1. 编译 Hub（在 Arch 上）
cd hub
GOOS=linux GOARCH=amd64 go build -o bin/openvibe-hub ./cmd/hub

# 2. 上传到华为云
scp bin/openvibe-hub huawei:/home/zcy/openvibe/

# 3. 在华为云上配置 systemd
scp scripts/openvibe-hub.service huawei:/tmp/
ssh huawei "sudo cp /tmp/openvibe-hub.service /etc/systemd/system/"
ssh huawei "sudo systemctl daemon-reload"
ssh huawei "sudo systemctl enable openvibe-hub"
ssh huawei "sudo systemctl start openvibe-hub"
```

### 3. 验证部署

#### 3.1 检查服务状态

```bash
# Arch 服务器
sudo systemctl status opencode
sudo systemctl status openvibe-tunnel

# 华为云
ssh huawei "sudo systemctl status openvibe-hub"
```

#### 3.2 检查端口

```bash
# 华为云上检查 Hub 端口
ssh huawei "ss -tlnp | grep 8080"

# 华为云上检查隧道端口
ssh huawei "ss -tlnp | grep 4096"
```

#### 3.3 端到端测试

```bash
# 测试 Hub 健康检查
curl http://121.36.218.61:8080/health

# 运行完整测试
./scripts/test-connection.sh "你好"
```

## 服务管理

### 常用命令

```bash
# 启动服务
sudo systemctl start opencode
sudo systemctl start openvibe-tunnel

# 停止服务
sudo systemctl stop opencode
sudo systemctl stop openvibe-tunnel

# 重启服务
sudo systemctl restart opencode
sudo systemctl restart openvibe-tunnel

# 查看日志
sudo journalctl -u opencode -f
sudo journalctl -u openvibe-tunnel -f
```

### 华为云服务管理

```bash
ssh huawei "sudo systemctl start openvibe-hub"
ssh huawei "sudo systemctl stop openvibe-hub"
ssh huawei "sudo systemctl restart openvibe-hub"
ssh huawei "sudo journalctl -u openvibe-hub -f"
```

## 安全配置

### Token 认证

编辑华为云上的服务文件，设置 Token：

```bash
ssh huawei "sudo vi /etc/systemd/system/openvibe-hub.service"
```

修改 `Environment=OPENVIBE_TOKEN=your-secure-token-here` 为你的实际 Token。

然后重启服务：

```bash
ssh huawei "sudo systemctl daemon-reload && sudo systemctl restart openvibe-hub"
```

### 防火墙

确保以下端口开放：

| 服务器 | 端口 | 用途 |
|--------|------|------|
| 华为云 | 8080 | Hub WebSocket |
| 华为云 | 22 | SSH |

## 故障排除

### SSH 隧道断开

```bash
# 检查隧道状态
sudo systemctl status openvibe-tunnel

# 查看日志
sudo journalctl -u openvibe-tunnel -n 50

# 手动测试隧道
ssh -N -R 4096:localhost:4096 huawei -v
```

### OpenCode 无响应

```bash
# 检查进程
ps aux | grep opencode

# 检查端口
ss -tlnp | grep 4096

# 重启服务
sudo systemctl restart opencode
```

### Hub 连接失败

```bash
# 检查 Hub 日志
ssh huawei "sudo journalctl -u openvibe-hub -n 50"

# 测试 OpenCode 连接（从华为云）
ssh huawei "curl http://localhost:4096/global/health"
```
