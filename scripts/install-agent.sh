#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run with sudo: sudo BACKEND_URL=http://backend:8080 ./scripts/install-agent.sh" >&2
  exit 1
fi

BACKEND_URL="${BACKEND_URL:-${1:-}}"
MOCK_MODE="${MOCK_MODE:-false}"
if [[ -z "${BACKEND_URL}" ]]; then
  echo "BACKEND_URL is required" >&2
  exit 1
fi

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "${repo_dir}/agent"
go build -trimpath -o /tmp/vmlens-agent ./cmd/agent
install -Dm0755 /tmp/vmlens-agent /usr/local/bin/vmlens-agent
install -d -m0750 /etc/vmlens /usr/lib/vmlens

if [[ -f ebpf/flow_tracker.bpf.o ]]; then
  install -m0644 ebpf/flow_tracker.bpf.o /usr/lib/vmlens/flow_tracker.bpf.o
fi

cat >/etc/vmlens/agent.env <<EOF
BACKEND_URL=${BACKEND_URL}
MOCK_MODE=${MOCK_MODE}
BPF_OBJECT=/usr/lib/vmlens/flow_tracker.bpf.o
HEARTBEAT_INTERVAL=20s
AGENT_ENVIRONMENT=external-vm
EOF
chmod 0640 /etc/vmlens/agent.env

cat >/etc/systemd/system/vmlens-agent.service <<'EOF'
[Unit]
Description=VMLens network relationship agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/vmlens/agent.env
ExecStart=/usr/local/bin/vmlens-agent
Restart=always
RestartSec=5
User=root
NoNewPrivileges=false
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now vmlens-agent
echo "VMLens agent installed; logs: journalctl -u vmlens-agent -f"

