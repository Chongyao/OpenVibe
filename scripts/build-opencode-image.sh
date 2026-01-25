#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

docker build -t openvibe/opencode:latest -f "$PROJECT_ROOT/agent/docker/opencode/Dockerfile" "$PROJECT_ROOT"

echo "Successfully built openvibe/opencode:latest"
