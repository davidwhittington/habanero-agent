#!/usr/bin/env bash
set -euo pipefail
echo "Installing Habanero Agent..."
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="latest"
URL="https://github.com/davidwhittington/habanero-agent/releases/download/${VERSION}/habanero-agent-${OS}-${ARCH}"
curl -fsSL "$URL" -o /usr/local/bin/habanero-agent
chmod +x /usr/local/bin/habanero-agent
mkdir -p /etc/habanero
if [ ! -f /etc/habanero/agent.yml ]; then
  curl -fsSL "https://raw.githubusercontent.com/davidwhittington/habanero-agent/main/configs/agent.example.yml" -o /etc/habanero/agent.yml
fi
curl -fsSL "https://raw.githubusercontent.com/davidwhittington/habanero-agent/main/systemd/habanero-agent.service" -o /etc/systemd/system/habanero-agent.service
systemctl daemon-reload
echo "Installed. Edit /etc/habanero/agent.yml then: systemctl enable --now habanero-agent"
