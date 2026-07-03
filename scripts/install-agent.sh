#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run with sudo: sudo BACKEND_URL=http://backend:8080 ./scripts/install-agent.sh" >&2
  exit 1
fi

BACKEND_URL="${BACKEND_URL:-${1:-}}"
MOCK_MODE="${MOCK_MODE:-false}"
TENANT_ID="${TENANT_ID:-}"
AGENT_PRIVATE_IPS="${AGENT_PRIVATE_IPS:-}"
AGENT_PUBLIC_IP="${AGENT_PUBLIC_IP:-}"
AGENT_IGNORE_IPS="${AGENT_IGNORE_IPS:-}"
AGENT_ENVIRONMENT="${AGENT_ENVIRONMENT:-external-vm}"
if [[ -z "${BACKEND_URL}" ]]; then
  echo "BACKEND_URL is required" >&2
  exit 1
fi

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
command -v go >/dev/null || { echo "go is required" >&2; exit 1; }
cd "${repo_dir}/agent"
go build -trimpath -o /tmp/vmlens-agent ./cmd/agent
install -Dm0755 /tmp/vmlens-agent /usr/local/bin/vmlens-agent
install -d -m0750 /etc/vmlens /usr/lib/vmlens

if [[ "${MOCK_MODE}" == "false" ]]; then
  command -v clang >/dev/null || { echo "clang is required for real eBPF mode" >&2; exit 1; }
  command -v bpftool >/dev/null || { echo "bpftool is required for real eBPF mode" >&2; exit 1; }
  [[ -r /sys/kernel/btf/vmlinux ]] || { echo "kernel BTF /sys/kernel/btf/vmlinux is required" >&2; exit 1; }
  build_dir="$(mktemp -d)"
  trap 'rm -rf "${build_dir}" /tmp/vmlens-agent' EXIT
  bpftool btf dump file /sys/kernel/btf/vmlinux format c >"${build_dir}/vmlinux.h"
  arch="$(uname -m)"
  case "${arch}" in
    x86_64) bpf_arch=x86 ;;
    aarch64|arm64) bpf_arch=arm64 ;;
    *) echo "unsupported eBPF architecture: ${arch}" >&2; exit 1 ;;
  esac
  clang -O2 -g -target bpf -D"__TARGET_ARCH_${bpf_arch}" \
    -I "${build_dir}" -c ebpf/flow_tracker.bpf.c -o "${build_dir}/flow_tracker.bpf.o"
  install -m0644 "${build_dir}/flow_tracker.bpf.o" /usr/lib/vmlens/flow_tracker.bpf.o
fi

cat >/etc/vmlens/agent.env <<EOF
BACKEND_URL=${BACKEND_URL}
MOCK_MODE=${MOCK_MODE}
BPF_OBJECT=/usr/lib/vmlens/flow_tracker.bpf.o
HEARTBEAT_INTERVAL=20s
TENANT_ID=${TENANT_ID}
AGENT_PRIVATE_IPS=${AGENT_PRIVATE_IPS}
AGENT_PUBLIC_IP=${AGENT_PUBLIC_IP}
IGNORE_IPS=${AGENT_IGNORE_IPS}
AGENT_ENVIRONMENT=${AGENT_ENVIRONMENT}
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
