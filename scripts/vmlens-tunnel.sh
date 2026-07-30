#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
config_file="${TUNNEL_CONFIG:-${VMLENS_CONFIG:-}}"
if [[ -n "${config_file}" && -f "${config_file}" ]]; then
  # shellcheck disable=SC1090
  source "${config_file}"
fi

usage() {
  cat <<EOF
Usage:
  bash scripts/vmlens-tunnel.sh list
  bash scripts/vmlens-tunnel.sh show <vm-alias-or-host>
  bash scripts/vmlens-tunnel.sh start|stop|restart|status <vm-alias-or-host> [ssh-key|agent|none]
  bash scripts/vmlens-tunnel.sh start-all|stop-all|restart-all|status-all
  bash scripts/vmlens-tunnel.sh forget-host <vm-alias-or-host>

What it opens:
  remote ${REMOTE_BACKEND:-127.0.0.1:18080} -> local ${LOCAL_BACKEND:-127.0.0.1:8080}

Config:
  VM_INVENTORY=configs/vms.local
  SSH_USER=ubuntu
  SSH_KEY=~/.vmlens/keys/id_ed25519_vmlens | agent | none
  SSH_PROXY_JUMP=
  LOCAL_BACKEND=127.0.0.1:8080
  REMOTE_BACKEND=127.0.0.1:18080
  TUNNEL_CONFIG=<optional-env-file>
EOF
}

action="${1:-}"
selector="${2:-}"
key_arg="${3:-}"

if [[ -z "${action}" ]]; then
  usage >&2
  exit 1
fi

expand_path() {
  local value="${1:-}"
  case "${value}" in
    "~") printf '%s\n' "${HOME}" ;;
    "~/"*) printf '%s/%s\n' "${HOME}" "${value#~/}" ;;
    *) printf '%s\n' "${value}" ;;
  esac
}

is_agent_key() {
  case "${1:-}" in
    ""|"-"|"agent"|"none"|"ssh-agent") return 0 ;;
    *) return 1 ;;
  esac
}

discover_default_key() {
  local candidate
  for candidate in "${HOME}/.vmlens/keys/id_ed25519_vmlens" "${HOME}/.ssh/id_ed25519_vmlens" /mnt/c/Users/*/.ssh/id_ed25519_vmlens; do
    if [[ -r "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}

default_ssh_user="${SSH_USER:-ubuntu}"
default_ssh_key="${SSH_KEY:-}"
default_ssh_proxy_jump="${SSH_PROXY_JUMP:-}"
if [[ -z "${default_ssh_key}" ]]; then
  default_ssh_key="$(discover_default_key || true)"
fi
default_ssh_key="${default_ssh_key:-~/.vmlens/keys/id_ed25519_vmlens}"
default_local_backend="${LOCAL_BACKEND:-127.0.0.1:8080}"
default_remote_backend="${REMOTE_BACKEND:-127.0.0.1:18080}"
state_dir="$(expand_path "${TUNNEL_STATE_DIR:-${HOME}/.vmlens/tunnels}")"
key_state_dir="$(expand_path "${KEY_STATE_DIR:-${HOME}/.vmlens/keys}")"
vm_inventory="${VM_INVENTORY:-${repo_dir}/configs/vms.local}"
if [[ "${vm_inventory}" != /* ]]; then
  vm_inventory="${repo_dir}/${vm_inventory}"
fi
if [[ ! -f "${vm_inventory}" && -f "${repo_dir}/configs/vms.example" ]]; then
  vm_inventory="${repo_dir}/configs/vms.example"
fi

load_defaults() {
  vm_alias="${1:-}"
  vm_host="${1:-}"
  ssh_user="${default_ssh_user}"
  ssh_key="${default_ssh_key}"
  ssh_proxy_jump="${default_ssh_proxy_jump}"
  local_backend="${default_local_backend}"
  remote_backend="${default_remote_backend}"
}

resolve_vm() {
  local query="$1"
  load_defaults "${query}"

  if [[ -f "${vm_inventory}" ]]; then
    local alias host user key remote local proxy_jump role host_type environment owner tenant_id project_id region zone network_id subnet_id public_ip provider_id probe_protocol probe_port capture_interface ignore_ports ignore_ips flow_allow_cidrs flow_deny_cidrs notes
    while IFS='|' read -r alias host user key remote local proxy_jump role host_type environment owner tenant_id project_id region zone network_id subnet_id public_ip provider_id probe_protocol probe_port capture_interface ignore_ports ignore_ips flow_allow_cidrs flow_deny_cidrs notes; do
      [[ -z "${alias}" || "${alias}" == \#* ]] && continue
      if [[ "${query}" == "${alias}" || "${query}" == "${host}" ]]; then
        vm_alias="${alias}"
        vm_host="${host}"
        [[ -n "${user:-}" && "${user}" != "-" ]] && ssh_user="${user}"
        [[ -n "${key:-}" && "${key}" != "-" ]] && ssh_key="${key}"
        [[ -n "${remote:-}" && "${remote}" != "-" ]] && remote_backend="${remote}"
        [[ -n "${local:-}" && "${local}" != "-" ]] && local_backend="${local}"
        [[ -n "${proxy_jump:-}" && "${proxy_jump}" != "-" ]] && ssh_proxy_jump="${proxy_jump}"
        return 0
      fi
    done <"${vm_inventory}"
  fi
  return 0
}

prepare_ssh_key() {
  local requested="${1:-}"
  if is_agent_key "${requested}"; then
    printf '%s\n' "agent"
    return 0
  fi

  local key_path
  key_path="$(expand_path "${requested}")"
  if [[ ! -r "${key_path}" ]]; then
    echo "SSH key not found: ${requested}" >&2
    echo "Set SSH_KEY, pass the key as the third argument, or set per-VM ssh_key in ${vm_inventory}" >&2
    return 1
  fi

  if [[ "${key_path}" == /mnt/c/* ]]; then
    mkdir -p "${key_state_dir}"
    install -m 0600 "${key_path}" "${key_state_dir}/id_ed25519_vmlens"
    key_path="${key_state_dir}/id_ed25519_vmlens"
  fi

  printf '%s\n' "${key_path}"
}

for_each_vm() {
  local callback="$1"
  if [[ ! -f "${vm_inventory}" ]]; then
    echo "VM inventory not found: ${vm_inventory}" >&2
    return 1
  fi

  local alias host user key remote local proxy_jump role host_type environment owner tenant_id project_id region zone network_id subnet_id public_ip provider_id probe_protocol probe_port capture_interface ignore_ports ignore_ips flow_allow_cidrs flow_deny_cidrs notes
  while IFS='|' read -r alias host user key remote local proxy_jump role host_type environment owner tenant_id project_id region zone network_id subnet_id public_ip provider_id probe_protocol probe_port capture_interface ignore_ports ignore_ips flow_allow_cidrs flow_deny_cidrs notes; do
    [[ -z "${alias}" || "${alias}" == \#* ]] && continue
    "${callback}" "${alias}"
  done <"${vm_inventory}"
}

print_vm_row() {
  resolve_vm "$1"
  local display_key
  if is_agent_key "${ssh_key}"; then
    display_key="agent"
  else
    display_key="$(expand_path "${ssh_key}")"
  fi
  printf '%-16s %-16s %-10s %-58s %-18s %-22s %-22s\n' \
    "${vm_alias}" "${vm_host}" "${ssh_user}" "${display_key}" "${ssh_proxy_jump:-direct}" "${remote_backend}" "${local_backend}"
}

list_vms() {
  printf '%-16s %-16s %-10s %-58s %-18s %-22s %-22s\n' "ALIAS" "HOST" "USER" "KEY" "PROXY_JUMP" "REMOTE_BACKEND" "LOCAL_BACKEND"
  for_each_vm print_vm_row
}

run_for_all() {
  local sub_action="$1"
  local failed=0
  run_one() {
    resolve_vm "$1"
    echo "==> ${sub_action} ${vm_alias} (${vm_host})"
    if ! TUNNEL_CONFIG="${config_file}" bash "$0" "${sub_action}" "${vm_alias}"; then
      failed=1
    fi
  }
  for_each_vm run_one
  return "${failed}"
}

forget_known_host() {
  resolve_vm "$1"
  local known_hosts_file="${HOME}/.ssh/known_hosts"
  if [[ ! -f "${known_hosts_file}" ]]; then
    echo "known_hosts not found: ${known_hosts_file}"
    return 0
  fi

  local entry seen_entries=" "
  for entry in "$1" "${vm_host}" "${vm_alias}"; do
    [[ -z "${entry}" ]] && continue
    if [[ "${seen_entries}" == *" ${entry} "* ]]; then
      continue
    fi
    seen_entries="${seen_entries}${entry} "
    ssh-keygen -f "${known_hosts_file}" -R "${entry}" || true
  done

  echo "Removed known_hosts entries for ${vm_alias} (${vm_host}) from ${known_hosts_file}"
}

case "${action}" in
  list) list_vms; exit $? ;;
  show)
    [[ -n "${selector}" ]] || { usage >&2; exit 1; }
    printf '%-16s %-16s %-10s %-58s %-18s %-22s %-22s\n' "ALIAS" "HOST" "USER" "KEY" "PROXY_JUMP" "REMOTE_BACKEND" "LOCAL_BACKEND"
    print_vm_row "${selector}"
    exit $?
    ;;
  forget-host)
    [[ -n "${selector}" ]] || { usage >&2; exit 1; }
    forget_known_host "${selector}"
    exit 0
    ;;
  start-all) run_for_all start; exit $? ;;
  stop-all) run_for_all stop; exit $? ;;
  restart-all) run_for_all restart; exit $? ;;
  status-all) run_for_all status; exit $? ;;
esac

[[ -n "${selector}" ]] || { usage >&2; exit 1; }

resolve_vm "${selector}"
if [[ -n "${key_arg}" ]]; then
  ssh_key="${key_arg}"
fi
ssh_key="$(prepare_ssh_key "${ssh_key}")"

mkdir -p "${state_dir}"
safe_host="${vm_host//[^A-Za-z0-9_.-]/_}"
control_path="${state_dir}/${ssh_user}_${safe_host}.ctl"
pid_path="${state_dir}/${ssh_user}_${safe_host}.pid"
log_path="${state_dir}/${ssh_user}_${safe_host}.log"
target="${ssh_user}@${vm_host}"
uses_proxy=false

ssh_common=(
  ssh
  -o ExitOnForwardFailure=yes
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
  -o StrictHostKeyChecking=accept-new
)
if [[ -z "${ssh_proxy_jump:-}" || "${ssh_proxy_jump}" == "-" ]]; then
  ssh_common+=(-S "${control_path}")
fi
if ! is_agent_key "${ssh_key}"; then
  ssh_common+=(-i "${ssh_key}" -o IdentitiesOnly=yes)
fi
if [[ -n "${ssh_proxy_jump:-}" && "${ssh_proxy_jump}" != "-" ]]; then
  uses_proxy=true
  jump_target="${ssh_proxy_jump}"
  if [[ "${jump_target}" != *@* ]]; then
    jump_target="${ssh_user}@${jump_target}"
  fi
  proxy_command=(ssh -o StrictHostKeyChecking=accept-new)
  if ! is_agent_key "${ssh_key}"; then
    proxy_command+=(-i "${ssh_key}" -o IdentitiesOnly=yes)
  fi
  proxy_command+=(-W %h:%p "${jump_target}")
  printf -v proxy_command_string "%q " "${proxy_command[@]}"
  ssh_common+=(-o "ProxyCommand=${proxy_command_string% }")
fi

find_tunnel_pids() {
  ps -eo pid=,comm=,args= | while read -r pid command args; do
    [[ "${command}" == "ssh" ]] || continue
    case "${args}" in
      *"-R ${remote_backend}:${local_backend}"*" ${target}"*)
        printf '%s\n' "${pid}"
        ;;
    esac
  done
}

pid_is_running() {
  [[ -f "${pid_path}" ]] || return 1
  local pid
  pid="$(cat "${pid_path}" 2>/dev/null || true)"
  [[ -n "${pid}" ]] || return 1
  if kill -0 "${pid}" >/dev/null 2>&1; then
    return 0
  fi
  pid="$(find_tunnel_pids | head -n 1)"
  if [[ -n "${pid}" ]]; then
    printf '%s\n' "${pid}" >"${pid_path}"
    return 0
  fi
  rm -f "${pid_path}"
  return 1
}

is_running() {
  if [[ "${uses_proxy}" == "true" ]]; then
    pid_is_running
    return
  fi
  "${ssh_common[@]}" -O check "${target}" >/dev/null 2>&1
}

start_tunnel() {
  if is_running; then
    echo "Tunnel already running: ${vm_alias} ${target}"
    return 0
  fi
  rm -f "${control_path}" "${pid_path}" "${log_path}"
  if [[ "${uses_proxy}" == "true" ]]; then
    "${ssh_common[@]}" -fN -R "${remote_backend}:${local_backend}" "${target}" >"${log_path}" 2>&1
    sleep 1
    local pid
    pid="$(find_tunnel_pids | head -n 1)"
    if [[ -z "${pid}" ]]; then
      echo "Tunnel failed: ${vm_alias} ${target}" >&2
      sed -n '1,80p' "${log_path}" >&2 || true
      return 1
    fi
    printf '%s\n' "${pid}" >"${pid_path}"
  else
    "${ssh_common[@]}" -M -fN -R "${remote_backend}:${local_backend}" "${target}"
  fi
  echo "Tunnel started: ${vm_alias} ${target}"
  echo "  ${remote_backend} -> ${local_backend}"
}

stop_tunnel() {
  if [[ "${uses_proxy}" == "true" ]]; then
    local pids
    pids="$(find_tunnel_pids)"
    if [[ -n "${pids}" ]]; then
      local pid
      while read -r pid; do
        [[ -n "${pid}" ]] || continue
        kill "${pid}" >/dev/null 2>&1 || true
      done <<<"${pids}"
      sleep 1
      pids="$(find_tunnel_pids)"
      while read -r pid; do
        [[ -n "${pid}" ]] || continue
        kill -9 "${pid}" >/dev/null 2>&1 || true
      done <<<"${pids}"
      rm -f "${pid_path}"
      echo "Tunnel stopped: ${vm_alias} ${target}"
      return 0
    fi
    if pid_is_running; then
      pid="$(cat "${pid_path}")"
      kill "${pid}" >/dev/null 2>&1 || true
      rm -f "${pid_path}"
      echo "Tunnel stopped: ${vm_alias} ${target}"
      return 0
    fi
    rm -f "${pid_path}" "${control_path}"
    echo "Tunnel already stopped: ${vm_alias} ${target}"
    return 0
  fi
  if is_running; then
    "${ssh_common[@]}" -O exit "${target}" >/dev/null
    echo "Tunnel stopped: ${vm_alias} ${target}"
    return 0
  fi
  rm -f "${control_path}"
  echo "Tunnel already stopped: ${vm_alias} ${target}"
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
