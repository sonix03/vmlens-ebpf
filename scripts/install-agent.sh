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
FLOW_INTERVAL="${FLOW_INTERVAL:-1s}"
CAPTURE_MODE="${CAPTURE_MODE:-auto}"
CAPTURE_INTERFACE="${CAPTURE_INTERFACE:-}"
FLOW_ALLOW_CIDRS="${FLOW_ALLOW_CIDRS:-}"
FLOW_DENY_CIDRS="${FLOW_DENY_CIDRS:-}"

# auto: use prebuilt files when URLs/paths are provided, otherwise build locally.
# prebuilt: require prebuilt agent binary and, in real mode, prebuilt eBPF object.
# build: always build from source on the VM.
INSTALL_MODE="${INSTALL_MODE:-auto}"
AGENT_BINARY_URL="${AGENT_BINARY_URL:-}"
AGENT_BINARY_PATH="${AGENT_BINARY_PATH:-}"
BPF_OBJECT_URL="${BPF_OBJECT_URL:-}"
BPF_OBJECT_PATH="${BPF_OBJECT_PATH:-}"

if [[ -z "${BACKEND_URL}" ]]; then
  echo "BACKEND_URL is required" >&2
  exit 1
fi

export HOME="${HOME:-/root}"
export GOPATH="${GOPATH:-/root/go}"
export GOMODCACHE="${GOMODCACHE:-${GOPATH}/pkg/mod}"
export GOCACHE="${GOCACHE:-/root/.cache/go-build}"
mkdir -p "${GOMODCACHE}" "${GOCACHE}"

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
install -d -m0750 /etc/vmlens /usr/lib/vmlens

download_file() {
  local url="$1"
  local destination="$2"
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --show-error --output "${destination}" "${url}"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -O "${destination}" "${url}"
    return
  fi
  echo "curl or wget is required to download ${url}" >&2
  return 1
}

install_agent_prebuilt() {
  local source="${AGENT_BINARY_PATH}"
  if [[ -n "${AGENT_BINARY_URL}" ]]; then
    source="/tmp/vmlens-agent.prebuilt"
    download_file "${AGENT_BINARY_URL}" "${source}"
  fi
  [[ -n "${source}" ]] || return 1
  [[ -r "${source}" ]] || { echo "agent binary not found: ${source}" >&2; return 1; }
  install -Dm0755 "${source}" /usr/local/bin/vmlens-agent
}

build_agent_from_source() {
  [[ -d "${repo_dir}/agent" ]] || { echo "agent source directory not found: ${repo_dir}/agent" >&2; exit 1; }
  command -v go >/dev/null || { echo "go is required" >&2; exit 1; }
  cd "${repo_dir}/agent"
  go build -trimpath -o /tmp/vmlens-agent ./cmd/agent
  install -Dm0755 /tmp/vmlens-agent /usr/local/bin/vmlens-agent
}

install_bpf_prebuilt() {
  local source="${BPF_OBJECT_PATH}"
  if [[ -n "${BPF_OBJECT_URL}" ]]; then
    source="/tmp/flow_tracker.bpf.o.prebuilt"
    download_file "${BPF_OBJECT_URL}" "${source}"
  fi
  [[ -n "${source}" ]] || return 1
  [[ -r "${source}" ]] || { echo "eBPF object not found: ${source}" >&2; return 1; }
  install -m0644 "${source}" /usr/lib/vmlens/flow_tracker.bpf.o
}

build_bpf_from_source() {
  [[ -d "${repo_dir}/agent" ]] || { echo "agent source directory not found: ${repo_dir}/agent" >&2; exit 1; }
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
  cd "${repo_dir}/agent"
  clang -O2 -g -target bpf -D"__TARGET_ARCH_${bpf_arch}" \
    -I "${build_dir}" -c ebpf/flow_tracker.bpf.c -o "${build_dir}/flow_tracker.bpf.o"
  install -m0644 "${build_dir}/flow_tracker.bpf.o" /usr/lib/vmlens/flow_tracker.bpf.o
}

has_agent_prebuilt() {
  [[ -n "${AGENT_BINARY_URL}" || -n "${AGENT_BINARY_PATH}" ]]
}

has_bpf_prebuilt() {
  [[ -n "${BPF_OBJECT_URL}" || -n "${BPF_OBJECT_PATH}" ]]
}

case "${INSTALL_MODE}" in
  prebuilt)
    install_agent_prebuilt || { echo "prebuilt install requires AGENT_BINARY_URL or AGENT_BINARY_PATH" >&2; exit 1; }
    ;;
  build)
    build_agent_from_source
    ;;
  auto)
    if has_agent_prebuilt; then
      install_agent_prebuilt
    else
      build_agent_from_source
    fi
    ;;
  *)
    echo "unsupported INSTALL_MODE=${INSTALL_MODE}; use auto, prebuilt, or build" >&2
    exit 1
    ;;
esac

if [[ "${MOCK_MODE}" == "false" ]]; then
  [[ -r /sys/kernel/btf/vmlinux ]] || { echo "kernel BTF /sys/kernel/btf/vmlinux is required" >&2; exit 1; }
  case "${INSTALL_MODE}" in
    prebuilt)
      install_bpf_prebuilt || { echo "prebuilt real mode requires BPF_OBJECT_URL or BPF_OBJECT_PATH" >&2; exit 1; }
      ;;
    build)
      build_bpf_from_source
      ;;
    auto)
      if has_bpf_prebuilt; then
        install_bpf_prebuilt
      else
        build_bpf_from_source
      fi
      ;;
  esac
fi

cat >/etc/vmlens/agent.env <<EOF
BACKEND_URL=${BACKEND_URL}
MOCK_MODE=${MOCK_MODE}
BPF_OBJECT=/usr/lib/vmlens/flow_tracker.bpf.o
HEARTBEAT_INTERVAL=20s
FLOW_INTERVAL=${FLOW_INTERVAL}
CAPTURE_MODE=${CAPTURE_MODE}
CAPTURE_INTERFACE=${CAPTURE_INTERFACE}
TENANT_ID=${TENANT_ID}
AGENT_PRIVATE_IPS=${AGENT_PRIVATE_IPS}
AGENT_PUBLIC_IP=${AGENT_PUBLIC_IP}
IGNORE_IPS=${AGENT_IGNORE_IPS}
FLOW_ALLOW_CIDRS=${FLOW_ALLOW_CIDRS}
FLOW_DENY_CIDRS=${FLOW_DENY_CIDRS}
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
AmbientCapabilities=CAP_BPF CAP_PERFMON CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_BPF CAP_PERFMON CAP_NET_ADMIN CAP_NET_RAW CAP_SYS_ADMIN
NoNewPrivileges=false
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable vmlens-agent
systemctl restart vmlens-agent
echo "VMLens agent installed; logs: journalctl -u vmlens-agent -f"
