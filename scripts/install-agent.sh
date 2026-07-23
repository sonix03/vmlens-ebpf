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
CAPTURE_MODE="${CAPTURE_MODE:-tc}"
CAPTURE_INTERFACE="${CAPTURE_INTERFACE:-ens3}"
FLOW_ALLOW_CIDRS="${FLOW_ALLOW_CIDRS:-}"
FLOW_DENY_CIDRS="${FLOW_DENY_CIDRS:-}"
AUTO_DENY_TUNNEL_PEER="${AUTO_DENY_TUNNEL_PEER:-true}"

# auto: use prebuilt files when URLs/paths are provided, otherwise build locally.
# prebuilt: require prebuilt agent binary and, in real mode, prebuilt eBPF object.
# build: always build from source on the VM.
INSTALL_MODE="${INSTALL_MODE:-auto}"
AGENT_BINARY_URL="${AGENT_BINARY_URL:-}"
AGENT_BINARY_PATH="${AGENT_BINARY_PATH:-}"
BPF_OBJECT_URL="${BPF_OBJECT_URL:-}"
BPF_OBJECT_PATH="${BPF_OBJECT_PATH:-}"

# Optional DeepFlow agent install. This keeps the VM setup in one installer
# without making DeepFlow mandatory for the realtime VMLens agent.
INSTALL_DEEPFLOW_AGENT="${INSTALL_DEEPFLOW_AGENT:-false}"
DEEPFLOW_AGENT_VERSION="${DEEPFLOW_AGENT_VERSION:-v6.6.1}"
DEEPFLOW_AGENT_URL="${DEEPFLOW_AGENT_URL:-}"
DEEPFLOW_AGENT_STATIC_LINK="${DEEPFLOW_AGENT_STATIC_LINK:-false}"
DEEPFLOW_AGENT_CONTROLLER_IPS="${DEEPFLOW_AGENT_CONTROLLER_IPS:-127.0.0.1}"
DEEPFLOW_AGENT_CONTROLLER_PORT="${DEEPFLOW_AGENT_CONTROLLER_PORT:-30035}"
DEEPFLOW_AGENT_VTAP_GROUP_ID_REQUEST="${DEEPFLOW_AGENT_VTAP_GROUP_ID_REQUEST:-}"
DEEPFLOW_AGENT_TEAM_ID="${DEEPFLOW_AGENT_TEAM_ID:-}"
DEEPFLOW_AGENT_OVERRIDE_HOSTNAME="${DEEPFLOW_AGENT_OVERRIDE_HOSTNAME:-}"
DEEPFLOW_AGENT_UNIQUE_IDENTIFIER="${DEEPFLOW_AGENT_UNIQUE_IDENTIFIER:-ip-and-mac}"
DEEPFLOW_AGENT_LOG_FILE="${DEEPFLOW_AGENT_LOG_FILE:-/var/log/deepflow-agent/deepflow-agent.log}"
INSTALL_DEEPFLOW_RELAY="${INSTALL_DEEPFLOW_RELAY:-false}"
DEEPFLOW_RELAY_BIND="${DEEPFLOW_RELAY_BIND:-}"
DEEPFLOW_RELAY_CONTROLLER_LISTEN="${DEEPFLOW_RELAY_CONTROLLER_LISTEN:-30035}"
DEEPFLOW_RELAY_CONTROLLER_TARGET="${DEEPFLOW_RELAY_CONTROLLER_TARGET:-127.0.0.1:30035}"
DEEPFLOW_RELAY_INGESTER_LISTEN="${DEEPFLOW_RELAY_INGESTER_LISTEN:-30033}"
DEEPFLOW_RELAY_INGESTER_TARGET="${DEEPFLOW_RELAY_INGESTER_TARGET:-127.0.0.1:30033}"

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

append_csv_unique() {
  local current="$1"
  local value="$2"
  local part
  [[ -n "${value}" ]] || { printf '%s\n' "${current}"; return; }
  IFS=',' read -r -a parts <<<"${current}"
  for part in "${parts[@]}"; do
    if [[ "${part}" == "${value}" ]]; then
      printf '%s\n' "${current}"
      return
    fi
  done
  if [[ -z "${current}" ]]; then
    printf '%s\n' "${value}"
  else
    printf '%s,%s\n' "${current}" "${value}"
  fi
}

ip_to_cidr() {
  local ip="$1"
  if [[ "${ip}" == *:* ]]; then
    printf '%s/128\n' "${ip}"
  else
    printf '%s/32\n' "${ip}"
  fi
}

ssh_peer_ip() {
  if [[ -n "${SSH_CLIENT:-}" ]]; then
    awk '{print $1; exit}' <<<"${SSH_CLIENT}"
    return
  fi
  if [[ -n "${SSH_CONNECTION:-}" ]]; then
    awk '{print $1; exit}' <<<"${SSH_CONNECTION}"
    return
  fi
  who -m 2>/dev/null | sed -n 's/.*(\([^)]*\)).*/\1/p' | awk '{print $1; exit}'
}

auto_filter_tunnel_peer() {
  [[ "${AUTO_DENY_TUNNEL_PEER}" == "true" ]] || return 0
  local peer_ip
  peer_ip="$(ssh_peer_ip || true)"
  [[ -n "${peer_ip}" ]] || return 0
  case "${peer_ip}" in
    127.*|::1) return 0 ;;
  esac
  AGENT_IGNORE_IPS="$(append_csv_unique "${AGENT_IGNORE_IPS}" "${peer_ip}")"
  FLOW_DENY_CIDRS="$(append_csv_unique "${FLOW_DENY_CIDRS}" "$(ip_to_cidr "${peer_ip}")")"
  echo "VMLens: auto-filtering SSH tunnel peer from captured flows: ${peer_ip}" >&2
}

default_route_interface() {
  ip route show default 2>/dev/null | awk '{print $5; exit}'
}

default_route_ip() {
  ip -4 route get 1.1.1.1 2>/dev/null | awk '
    {
      for (i = 1; i <= NF; i++) {
        if ($i == "src") {
          print $(i + 1)
          exit
        }
      }
    }
  '
}

resolve_capture_interface() {
  case "${CAPTURE_MODE}" in
    tc|auto) ;;
    *) return 0 ;;
  esac
  if [[ -n "${CAPTURE_INTERFACE}" ]] && ip link show "${CAPTURE_INTERFACE}" >/dev/null 2>&1; then
    return 0
  fi
  local fallback
  fallback="$(default_route_interface || true)"
  if [[ -n "${fallback}" ]]; then
    if [[ -n "${CAPTURE_INTERFACE}" ]]; then
      echo "VMLens: capture interface ${CAPTURE_INTERFACE} not found; using default-route interface ${fallback}" >&2
    fi
    CAPTURE_INTERFACE="${fallback}"
    return 0
  fi
  if [[ "${CAPTURE_MODE}" == "tc" ]]; then
    echo "CAPTURE_INTERFACE=${CAPTURE_INTERFACE} not found and no default-route interface was detected" >&2
    exit 1
  fi
}

resolve_deepflow_relay_config() {
  is_true "${INSTALL_DEEPFLOW_RELAY}" || return 0
  if [[ -z "${DEEPFLOW_RELAY_BIND}" ]]; then
    DEEPFLOW_RELAY_BIND="$(default_route_ip || true)"
  fi
  DEEPFLOW_RELAY_BIND="${DEEPFLOW_RELAY_BIND:-0.0.0.0}"

  if is_true "${INSTALL_DEEPFLOW_AGENT}" && [[ "${DEEPFLOW_AGENT_CONTROLLER_IPS}" == "127.0.0.1" && "${DEEPFLOW_RELAY_BIND}" != "0.0.0.0" ]]; then
    DEEPFLOW_AGENT_CONTROLLER_IPS="${DEEPFLOW_RELAY_BIND}"
  fi
}

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

is_true() {
  case "${1:-}" in
    true|TRUE|1|yes|YES|y|Y|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

release_arch() {
  case "$(uname -m)" in
    x86_64) printf '%s\n' amd64 ;;
    aarch64|arm64) printf '%s\n' arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; return 1 ;;
  esac
}

yaml_quote() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "${value}"
}

write_deepflow_agent_config() {
  install -d -m0755 "$(dirname "${DEEPFLOW_AGENT_LOG_FILE}")"

  {
    echo "controller-ips:"
    local ip
    for ip in ${DEEPFLOW_AGENT_CONTROLLER_IPS//,/ }; do
      [[ -n "${ip}" ]] && echo "  - ${ip}"
    done
    echo "controller-port: ${DEEPFLOW_AGENT_CONTROLLER_PORT}"
    echo "log-file: $(yaml_quote "${DEEPFLOW_AGENT_LOG_FILE}")"
    echo "agent-unique-identifier: $(yaml_quote "${DEEPFLOW_AGENT_UNIQUE_IDENTIFIER}")"
    if [[ -n "${DEEPFLOW_AGENT_VTAP_GROUP_ID_REQUEST}" ]]; then
      echo "vtap-group-id-request: $(yaml_quote "${DEEPFLOW_AGENT_VTAP_GROUP_ID_REQUEST}")"
    fi
    if [[ -n "${DEEPFLOW_AGENT_TEAM_ID}" ]]; then
      echo "team-id: $(yaml_quote "${DEEPFLOW_AGENT_TEAM_ID}")"
    fi
    if [[ -n "${DEEPFLOW_AGENT_OVERRIDE_HOSTNAME}" ]]; then
      echo "override-os-hostname: $(yaml_quote "${DEEPFLOW_AGENT_OVERRIDE_HOSTNAME}")"
    fi
  } >/etc/deepflow-agent.yaml

  chmod 0644 /etc/deepflow-agent.yaml
}

install_deepflow_agent() {
  is_true "${INSTALL_DEEPFLOW_AGENT}" || return 0
  command -v tar >/dev/null || { echo "tar is required to install DeepFlow agent" >&2; exit 1; }

  local arch url tmp_dir archive binary static_segment
  arch="$(release_arch)"
  static_segment=""
  if is_true "${DEEPFLOW_AGENT_STATIC_LINK}"; then
    static_segment="/static-link"
  fi
  url="${DEEPFLOW_AGENT_URL:-https://deepflow-ce.oss-cn-beijing.aliyuncs.com/bin/agent/${DEEPFLOW_AGENT_VERSION}/linux${static_segment}/${arch}/deepflow-agent.tar.gz}"

  tmp_dir="$(mktemp -d)"
  archive="${tmp_dir}/deepflow-agent.tar.gz"
  download_file "${url}" "${archive}"
  tar -xzf "${archive}" -C "${tmp_dir}"

  binary="$(find "${tmp_dir}" -type f -name deepflow-agent | head -n 1 || true)"
  [[ -n "${binary}" && -r "${binary}" ]] || { echo "deepflow-agent binary not found in ${archive}" >&2; exit 1; }
  install -Dm0755 "${binary}" /usr/sbin/deepflow-agent
  rm -rf "${tmp_dir}"

  write_deepflow_agent_config

  cat >/etc/systemd/system/deepflow-agent.service <<'EOF'
[Unit]
Description=deepflow-agent.service
After=syslog.target network-online.target
Wants=network-online.target

[Service]
Environment=GOTRACEBACK=single
LimitCORE=1G
ExecStart=/usr/sbin/deepflow-agent
Restart=always
RestartSec=10
LimitNOFILE=1024:4096

[Install]
WantedBy=multi-user.target
EOF
}

install_deepflow_relay() {
  is_true "${INSTALL_DEEPFLOW_RELAY}" || return 0
  command -v python3 >/dev/null || { echo "python3 is required for DeepFlow local relay" >&2; exit 1; }

  cat >/usr/local/bin/vmlens-tcp-relay <<'PY'
#!/usr/bin/env python3
import select
import socket
import sys
import threading


def pipe(left, right):
    sockets = [left, right]
    try:
        while True:
            readable, _, _ = select.select(sockets, [], [], 60)
            if not readable:
                continue
            for src in readable:
                data = src.recv(65536)
                if not data:
                    return
                dst = right if src is left else left
                dst.sendall(data)
    except OSError:
        return
    finally:
        for sock in sockets:
            try:
                sock.close()
            except OSError:
                pass


def handle(client, target_host, target_port):
    try:
        upstream = socket.create_connection((target_host, target_port), timeout=10)
    except OSError:
        client.close()
        return
    pipe(client, upstream)


def main():
    if len(sys.argv) != 4:
        print("usage: vmlens-tcp-relay <bind_ip:listen_port> <target_ip:target_port> <name>", file=sys.stderr)
        return 2
    bind_host, bind_port_text = sys.argv[1].rsplit(":", 1)
    target_host, target_port_text = sys.argv[2].rsplit(":", 1)
    bind_port = int(bind_port_text)
    target_port = int(target_port_text)

    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind((bind_host, bind_port))
    server.listen(128)
    print(f"{sys.argv[3]} relay listening on {bind_host}:{bind_port} -> {target_host}:{target_port}", flush=True)
    while True:
        client, _ = server.accept()
        threading.Thread(target=handle, args=(client, target_host, target_port), daemon=True).start()


if __name__ == "__main__":
    raise SystemExit(main())
PY
  chmod 0755 /usr/local/bin/vmlens-tcp-relay

  cat >/etc/systemd/system/vmlens-deepflow-controller-relay.service <<EOF
[Unit]
Description=VMLens DeepFlow controller relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vmlens-tcp-relay ${DEEPFLOW_RELAY_BIND}:${DEEPFLOW_RELAY_CONTROLLER_LISTEN} ${DEEPFLOW_RELAY_CONTROLLER_TARGET} deepflow-controller
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
EOF

  cat >/etc/systemd/system/vmlens-deepflow-ingester-relay.service <<EOF
[Unit]
Description=VMLens DeepFlow ingester relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vmlens-tcp-relay ${DEEPFLOW_RELAY_BIND}:${DEEPFLOW_RELAY_INGESTER_LISTEN} ${DEEPFLOW_RELAY_INGESTER_TARGET} deepflow-ingester
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
EOF
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
    -I "${build_dir}" -c ebpf/programs/flow_tracker.bpf.c -o "${build_dir}/flow_tracker.bpf.o"
  install -m0644 "${build_dir}/flow_tracker.bpf.o" /usr/lib/vmlens/flow_tracker.bpf.o
}

has_agent_prebuilt() {
  [[ -n "${AGENT_BINARY_URL}" || -n "${AGENT_BINARY_PATH}" ]]
}

has_bpf_prebuilt() {
  [[ -n "${BPF_OBJECT_URL}" || -n "${BPF_OBJECT_PATH}" ]]
}

auto_filter_tunnel_peer
resolve_capture_interface
resolve_deepflow_relay_config

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

install_deepflow_relay
install_deepflow_agent

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
if is_true "${INSTALL_DEEPFLOW_RELAY}"; then
  systemctl enable vmlens-deepflow-controller-relay
  systemctl enable vmlens-deepflow-ingester-relay
  systemctl restart vmlens-deepflow-controller-relay
  systemctl restart vmlens-deepflow-ingester-relay
  echo "DeepFlow relays installed; logs: journalctl -u vmlens-deepflow-controller-relay -u vmlens-deepflow-ingester-relay -f"
fi
if is_true "${INSTALL_DEEPFLOW_AGENT}"; then
  systemctl enable deepflow-agent
  systemctl restart deepflow-agent
  echo "DeepFlow agent installed; config: /etc/deepflow-agent.yaml; logs: journalctl -u deepflow-agent -f"
fi
systemctl enable vmlens-agent
systemctl restart vmlens-agent
echo "VMLens agent installed; logs: journalctl -u vmlens-agent -f"
