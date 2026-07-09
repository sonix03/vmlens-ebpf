#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 start|stop|restart|status|logs"
  echo "Env: BACKEND_URL=http://127.0.0.1:18080 MOCK_MODE=false FLOW_INTERVAL=1s INSTALL_MODE=auto|prebuilt|build"
}

action="${1:-}"
if [[ -z "${action}" ]]; then
  usage >&2
  exit 1
fi

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
service_name="vmlens-agent"

need_root() {
  if [[ ${EUID} -ne 0 ]]; then
    exec sudo --preserve-env=BACKEND_URL,MOCK_MODE,TENANT_ID,AGENT_PRIVATE_IPS,AGENT_PUBLIC_IP,AGENT_IGNORE_IPS,AGENT_ENVIRONMENT,FLOW_INTERVAL,INSTALL_MODE,AGENT_BINARY_URL,AGENT_BINARY_PATH,BPF_OBJECT_URL,BPF_OBJECT_PATH "$0" "$@"
  fi
}

start_agent() {
  need_root start
  export BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:18080}"
  export MOCK_MODE="${MOCK_MODE:-false}"
  export FLOW_INTERVAL="${FLOW_INTERVAL:-1s}"
  "${repo_dir}/scripts/install-agent.sh"
  if [[ -f /etc/vmlens/agent.env ]]; then
    if grep -q '^FLOW_INTERVAL=' /etc/vmlens/agent.env; then
      sed -i "s/^FLOW_INTERVAL=.*/FLOW_INTERVAL=${FLOW_INTERVAL}/" /etc/vmlens/agent.env
    else
      echo "FLOW_INTERVAL=${FLOW_INTERVAL}" >>/etc/vmlens/agent.env
    fi
  fi
  systemctl restart "${service_name}"
  systemctl --no-pager --lines=0 status "${service_name}"
}

case "${action}" in
  start) start_agent ;;
  stop)
    need_root stop
    systemctl stop "${service_name}"
    echo "VMLens agent stopped"
    ;;
  restart)
    need_root restart
    systemctl restart "${service_name}"
    systemctl --no-pager --lines=0 status "${service_name}"
    ;;
  status)
    systemctl --no-pager status "${service_name}"
    ;;
  logs)
    journalctl -u "${service_name}" -f
    ;;
  *) usage >&2; exit 1 ;;
esac
