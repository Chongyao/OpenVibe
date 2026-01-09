#!/bin/bash
# Deploy OpenVibe frontend and Hub to Huawei Cloud
# Usage: ./scripts/deploy-app.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
REMOTE_HOST="huawei"
REMOTE_DIR="/home/zcy/openvibe"

echo "🚀 OpenVibe Full Deployment"
echo "=========================="

# 1. Build Next.js static export
echo ""
echo "📦 Building Next.js app..."
cd "$PROJECT_DIR/app"

# Add output: 'export' to next.config if not already there
if ! grep -q "output:" next.config.*; then
    echo "Adding static export config..."
    cat > next.config.ts << 'EOF'
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'export',
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
EOF
fi

npm run build

if [ ! -d "out" ]; then
    echo "❌ Build failed - no 'out' directory created"
    exit 1
fi

echo "✅ Frontend built successfully"

# 2. Build Hub binary
echo ""
echo "🔧 Building Hub binary..."
cd "$PROJECT_DIR/hub"
GOOS=linux GOARCH=amd64 go build -o bin/openvibe-hub ./cmd/hub
echo "✅ Hub binary built"

# 3. Create deployment package
echo ""
echo "📤 Preparing deployment package..."
cd "$PROJECT_DIR"
rm -rf /tmp/openvibe-deploy
mkdir -p /tmp/openvibe-deploy/static

# Copy built files
cp hub/bin/openvibe-hub /tmp/openvibe-deploy/
cp -r app/out/* /tmp/openvibe-deploy/static/

# Create systemd service file with static dir
cat > /tmp/openvibe-deploy/openvibe-hub.service << EOF
[Unit]
Description=OpenVibe Hub Server
After=network.target

[Service]
Type=simple
User=zcy
WorkingDirectory=/home/zcy/openvibe
ExecStart=/home/zcy/openvibe/openvibe-hub -port 8080 -opencode http://localhost:4096 -static /home/zcy/openvibe/static
Restart=always
RestartSec=5
Environment=OPENVIBE_TOKEN=

[Install]
WantedBy=multi-user.target
EOF

# 4. Upload to remote server
echo ""
echo "📡 Uploading to $REMOTE_HOST..."

# Stop existing service (ignore errors)
ssh "$REMOTE_HOST" "sudo systemctl stop openvibe-hub 2>/dev/null || true"

# Create remote directory
ssh "$REMOTE_HOST" "mkdir -p $REMOTE_DIR"

# Upload files
rsync -avz --delete /tmp/openvibe-deploy/ "$REMOTE_HOST:$REMOTE_DIR/"

echo "✅ Files uploaded"

# 5. Setup and start service
echo ""
echo "⚙️ Configuring service..."

ssh "$REMOTE_HOST" << 'ENDSSH'
cd /home/zcy/openvibe
chmod +x openvibe-hub
sudo cp openvibe-hub.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable openvibe-hub
sudo systemctl start openvibe-hub
sleep 2
sudo systemctl status openvibe-hub --no-pager
ENDSSH

echo ""
echo "✅ Deployment complete!"
echo ""
echo "🌐 Access the app at: http://121.36.218.61:8080"
echo ""

# 6. Quick health check
echo "🔍 Health check..."
if curl -s http://121.36.218.61:8080/health | grep -q "ok"; then
    echo "✅ Hub is responding"
else
    echo "⚠️ Hub health check failed - check logs with:"
    echo "   ssh huawei 'sudo journalctl -u openvibe-hub -f'"
fi
