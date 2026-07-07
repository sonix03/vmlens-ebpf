#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 start|stop|restart|status <vm-ip-or-host>"
  echo "Env: SSH_USER=ubuntu SSH_KEY=~/.ssh/id_ed25519_vmlens LOCAL_BACKEND=127.0.0.1:8080 REMOTE_BACKEND=127.0.0.1:18080"
}

action="${1:-}"
vm_host="${2:-}"
if [[ -z "${action}" || -z "${vm_host}" ]]; then
  usage >&2
  exit 1
fi

ssh_user="${SSH_USER:-ubuntu}"
local_backend="${LOCAL_BACKEND:-127.0.0.1:8080}"
remote_backend="${REMOTE_BACKEND:-127.0.0.1:18080}"
state_dir="${VMLENS_TUNNEL_STATE_DIR:-${HOME}/.vmlens/tunnels}"
key_state_dir="${VMLENS_KEY_STATE_DIR:-${HOME}/.vmlens/keys}"
default_ssh_key="${HOME}/.ssh/id_ed25519_vmlens"
windows_ssh_key="/mnt/c/Users/USER/.ssh/id_ed25519_vmlens"
ssh_key="${SSH_KEY:-${default_ssh_key}}"
if [[ -z "${SSH_KEY:-}" && ! -r "${default_ssh_key}" && -r "${windows_ssh_key}" ]]; then
  mkdir -p "${key_state_dir}"
  install -m 0600 "${windows_ssh_key}" "${key_state_dir}/id_ed25519_vmlens"
  ssh_key="${key_state_dir}/id_ed25519_vmlens"
fi
safe_host="${vm_host//[^A-Za-z0-9_.-]/_}"
control_path="${state_dir}/${ssh_user}_${safe_host}.ctl"

ssh_common=(
  ssh
  -i "${ssh_key}"
  -o IdentitiesOnly=yes
  -o ExitOnForwardFailure=yes
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
  -S "${control_path}"
)

target="${ssh_user}@${vm_host}"

is_running() {
  "${ssh_common[@]}" -O check "${target}" >/dev/null 2>&1
}

start_tunnel() {
  mkdir -p "${state_dir}"
  if is_running; then
    echo "VMLens tunnel already running for ${target}"
    return
  fi
  "${ssh_common[@]}" -M -fN -R "${remote_backend}:${local_backend}" "${target}"
  echo "VMLens tunnel started: ${target} ${remote_backend} -> ${local_backend}"
}

stop_tunnel() {
  if is_running; then
    "${ssh_common[@]}" -O exit "${target}" >/dev/null
    echo "VMLens tunnel stopped for ${target}"
  else
    echo "VMLens tunnel is not running for ${target}"
  fi
}

case "${action}" in
  start) start_tunnel ;;
  stop) stop_tunnel ;;
  restart) stop_tunnel; start_tunnel ;;
  status)
    if is_running; then
      echo "running"
    else
      echo "stopped"
      exit 1
    fi
    ;;
  *) usage >&2; exit 1 ;;
esac
