#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 start|stop|restart|status|logs"
  echo "Env: BACKEND_URL=http://127.0.0.1:18080 MOCK_MODE=false FLOW_INTERVAL=1s CAPTURE_MODE=tc CAPTURE_INTERFACE=ens3 INSTALL_MODE=auto|prebuilt|build"
  echo "DeepFlow optional: INSTALL_DEEPFLOW_AGENT=true INSTALL_DEEPFLOW_RELAY=true DEEPFLOW_AGENT_CONTROLLER_IPS=<VM_IP> DEEPFLOW_AGENT_CONTROLLER_PORT=30035"
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
    exec sudo --preserve-env=BACKEND_URL,MOCK_MODE,TENANT_ID,AGENT_PRIVATE_IPS,AGENT_PUBLIC_IP,AGENT_IGNORE_IPS,AGENT_ENVIRONMENT,FLOW_INTERVAL,CAPTURE_MODE,CAPTURE_INTERFACE,FLOW_ALLOW_CIDRS,FLOW_DENY_CIDRS,AUTO_DENY_TUNNEL_PEER,SSH_CLIENT,SSH_CONNECTION,INSTALL_MODE,AGENT_BINARY_URL,AGENT_BINARY_PATH,BPF_OBJECT_URL,BPF_OBJECT_PATH,INSTALL_DEEPFLOW_AGENT,DEEPFLOW_AGENT_VERSION,DEEPFLOW_AGENT_URL,DEEPFLOW_AGENT_STATIC_LINK,DEEPFLOW_AGENT_CONTROLLER_IPS,DEEPFLOW_AGENT_CONTROLLER_PORT,DEEPFLOW_AGENT_VTAP_GROUP_ID_REQUEST,DEEPFLOW_AGENT_TEAM_ID,DEEPFLOW_AGENT_OVERRIDE_HOSTNAME,DEEPFLOW_AGENT_UNIQUE_IDENTIFIER,DEEPFLOW_AGENT_LOG_FILE,INSTALL_DEEPFLOW_RELAY,DEEPFLOW_RELAY_BIND,DEEPFLOW_RELAY_CONTROLLER_LISTEN,DEEPFLOW_RELAY_CONTROLLER_TARGET,DEEPFLOW_RELAY_INGESTER_LISTEN,DEEPFLOW_RELAY_INGESTER_TARGET "$0" "$@"
  fi
}

start_agent() {
  need_root start
  export BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:18080}"
  export MOCK_MODE="${MOCK_MODE:-false}"
  export FLOW_INTERVAL="${FLOW_INTERVAL:-1s}"
  export CAPTURE_MODE="${CAPTURE_MODE:-tc}"
  export CAPTURE_INTERFACE="${CAPTURE_INTERFACE:-ens3}"
  export AUTO_DENY_TUNNEL_PEER="${AUTO_DENY_TUNNEL_PEER:-true}"
  "${repo_dir}/scripts/install-agent.sh"
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
