# OpenVibe 部署指南

## 系统架构

```
┌─────────────────┐     WebSocket       ┌─────────────────┐
│   Arch 服务器   │◀───────────────────▶│    华为云       │
│   (Agent)       │                     │    (Hub)        │
│  222.205.46.147 │                     │  121.36.218.61  │
└─────────────────┘                     └─────────────────┘
        │                                       │
   Docker 容器                             openvibe-hub
   openvibe-opencode                       0.0.0.0:8080
   (多项目支持)                                 │
                                         ┌─────▼─────┐
                                         │  手机 App │
                                         └───────────┘
```

## 部署步骤

### 1. Arch 服务器配置 (Agent + Docker)

#### 1.1 前置条件

```bash
# 确保 Docker 已安装并运行
docker --version
sudo systemctl enable docker
sudo systemctl start docker

# 确保当前用户有 docker 权限
sudo usermod -aG docker $USER
```

#### 1.2 构建 OpenCode Docker 镜像

```bash
cd /home/zcy/workspace/projects/OpenVibe

# 构建镜像
./scripts/build-opencode-image.sh

# 验证镜像
docker images | grep openvibe/opencode
```

#### 1.3 编译并启动 Agent

```bash
cd agent
go build -o bin/agent ./cmd/agent

# 启动 Agent (Docker 模式)
./bin/agent \
  --hub ws://121.36.218.61:8080/agent \
  --token your-secure-token \
  --projects "/home/zcy/workspace/projects/OpenVibe,/home/zcy/workspace/projects/SmartQuant" \
  --docker-image openvibe/opencode:latest
```

#### 1.4 安装 Agent systemd 服务

```bash
# 复制服务文件
sudo cp scripts/openvibe-agent.service /etc/systemd/system/

# 重新加载 systemd
sudo systemctl daemon-reload

# 启用并启动服务
sudo systemctl enable openvibe-agent
sudo systemctl start openvibe-agent

# 检查状态
sudo systemctl status openvibe-agent
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
sudo systemctl status openvibe-agent

# 检查 Docker 容器
docker ps | grep openvibe-opencode

# 华为云
ssh huawei "sudo systemctl status openvibe-hub"
```

#### 3.2 检查端口

```bash
# 华为云上检查 Hub 端口
ssh huawei "ss -tlnp | grep 8080"

# 本地检查 OpenCode 容器端口
ss -tlnp | grep 409
```

#### 3.3 端到端测试

```bash
# 测试 Hub 健康检查
curl http://121.36.218.61:8080/health

# 测试 OpenCode 容器健康检查
curl http://localhost:4096/global/health
```

## Docker 容器管理

### 容器命名规则

容器名称格式: `openvibe-opencode-{project-name}`

例如:
- `openvibe-opencode-OpenVibe`
- `openvibe-opencode-SmartQuant`

### 手动容器操作

```bash
# 查看所有 OpenCode 容器
docker ps -a --filter name=openvibe-opencode-

# 查看容器日志
docker logs openvibe-opencode-OpenVibe

# 停止容器
docker stop openvibe-opencode-OpenVibe

# 删除容器
docker rm openvibe-opencode-OpenVibe

# 手动启动容器测试
docker run -d --name test-opencode \
  --network host \
  -v /path/to/project:/project \
  -w /project \
  openvibe/opencode:latest \
  --port 4096
```

## 服务管理

### 常用命令

```bash
# 启动服务
sudo systemctl start openvibe-agent

# 停止服务
sudo systemctl stop openvibe-agent

# 重启服务
sudo systemctl restart openvibe-agent

# 查看日志
sudo journalctl -u openvibe-agent -f
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

### Docker 容器问题

```bash
# 检查容器状态
docker ps -a --filter name=openvibe-opencode-

# 查看容器日志
docker logs --tail 50 openvibe-opencode-OpenVibe

# 检查容器健康
curl http://localhost:4096/global/health

# 重启容器
docker restart openvibe-opencode-OpenVibe
```

### Agent 连接问题

```bash
# 检查 Agent 日志
sudo journalctl -u openvibe-agent -n 50

# 检查 WebSocket 连接
curl http://121.36.218.61:8080/health
```

### Hub 连接失败

```bash
# 检查 Hub 日志
ssh huawei "sudo journalctl -u openvibe-hub -n 50"

# 测试 Hub 健康
ssh huawei "curl http://localhost:8080/health"
```
