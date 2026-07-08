#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
config_file="${VMLINUX_CONFIG:-${repo_dir}/configs/local.env}"
if [[ -f "${config_file}" ]]; then
  # shellcheck disable=SC1090
  source "${config_file}"
fi

usage() {
  cat <<EOF
Usage:
  $0 list
  $0 start|stop|restart|status <vm-alias-or-host>
  $0 start-all|stop-all|restart-all|status-all

Config:
  VMLENS_CONFIG=${config_file}

Env override:
  VMLENS_SSH_USER=ubuntu
  VMLENS_SSH_KEY=~/.ssh/id_ed25519_vmlens | agent | none
  VMLENS_LOCAL_BACKEND=127.0.0.1:8080
  VMLENS_REMOTE_BACKEND=127.0.0.1:18080
  VMLENS_VM_INVENTORY=configs/vms.local
EOF
}

action="${1:-}"
vm_host="${2:-}"
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

is_disabled_key() {
  case "${1:-}" in
    ""|"-"|"none"|"agent"|"ssh-agent") return 0 ;;
    *) return 1 ;;
  esac
}

discover_default_key() {
  local candidate
  for candidate in "${HOME}/.ssh/id_ed25519_vmlens" /mnt/c/Users/*/.ssh/id_ed25519_vmlens; do
    if [[ -r "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}

prepare_ssh_key() {
  local requested="${1:-}"
  if is_disabled_key "${requested}"; then
    printf '%s\n' "agent"
    return 0
  fi

  local key_path
  key_path="$(expand_path "${requested}")"
  if [[ ! -r "${key_path}" ]]; then
    if key_path="$(discover_default_key)"; then
      :
    else
      echo "SSH key not found: ${requested}" >&2
      echo "Set VMLENS_SSH_KEY, SSH_KEY, or per-VM ssh_key in ${vm_inventory}" >&2
      return 1
    fi
  fi

  if [[ "${key_path}" == /mnt/c/* ]]; then
    mkdir -p "${key_state_dir}"
    install -m 0600 "${key_path}" "${key_state_dir}/id_ed25519_vmlens"
    key_path="${key_state_dir}/id_ed25519_vmlens"
  fi

  printf '%s\n' "${key_path}"
}

default_ssh_user="${SSH_USER:-${VMLINUX_SSH_USER:-ubuntu}}"
default_ssh_key="${SSH_KEY:-${VMLINUX_SSH_KEY:-~/.ssh/id_ed25519_vmlens}}"
default_local_backend="${LOCAL_BACKEND:-${VMLINUX_LOCAL_BACKEND:-127.0.0.1:8080}}"
default_remote_backend="${REMOTE_BACKEND:-${VMLINUX_REMOTE_BACKEND:-127.0.0.1:18080}}"
state_dir="$(expand_path "${VMLINUX_TUNNEL_STATE_DIR:-${HOME}/.vmlens/tunnels}")"
key_state_dir="$(expand_path "${VMLINUX_KEY_STATE_DIR:-${HOME}/.vmlens/keys}")"
vm_inventory="${VMLINUX_VM_INVENTORY:-${repo_dir}/configs/vms.local}"
if [[ "${vm_inventory}" != /* ]]; then
  vm_inventory="${repo_dir}/${vm_inventory}"
fi
if [[ ! -f "${vm_inventory}" && -f "${repo_dir}/configs/vms.example" ]]; then
  vm_inventory="${repo_dir}/configs/vms.example"
fi

list_vms() {
  if [[ ! -f "${vm_inventory}" ]]; then
    echo "VM inventory not found: ${vm_inventory}" >&2
    return 1
  fi
  printf '%-16s %-16s %-10s %-34s %-22s %-22s\n' "ALIAS" "HOST" "USER" "KEY" "REMOTE_BACKEND" "LOCAL_BACKEND"
  while IFS='|' read -r alias host user key remote local; do
    [[ -z "${alias}" || "${alias}" == \#* ]] && continue
    printf '%-16s %-16s %-10s %-34s %-22s %-22s\n' \
      "${alias}" \
      "${host}" \
      "${user:--}" \
      "${key:--}" \
      "${remote:--}" \
      "${local:--}"
  done <"${vm_inventory}"
}

resolve_vm() {
  local selector="$1"
  vm_alias="${selector}"
  vm_host="${selector}"
  ssh_user="${default_ssh_user}"
  ssh_key="${default_ssh_key}"
  local_backend="${default_local_backend}"
  remote_backend="${default_remote_backend}"

  if [[ -f "${vm_inventory}" ]]; then
    while IFS='|' read -r alias host user key remote local; do
      [[ -z "${alias}" || "${alias}" == \#* ]] && continue
      if [[ "${selector}" == "${alias}" || "${selector}" == "${host}" ]]; then
        vm_alias="${alias}"
        vm_host="${host}"
        [[ -n "${user:-}" && "${user}" != "-" ]] && ssh_user="${user}"
        [[ -n "${key:-}" && "${key}" != "-" ]] && ssh_key="${key}"
        [[ -n "${remote:-}" && "${remote}" != "-" ]] && remote_backend="${remote}"
        [[ -n "${local:-}" && "${local}" != "-" ]] && local_backend="${local}"
        break
      fi
    done <"${vm_inventory}"
  fi
}

run_for_all() {
  local sub_action="$1"
  if [[ ! -f "${vm_inventory}" ]]; then
    echo "VM inventory not found: ${vm_inventory}" >&2
    exit 1
  fi
  local failed=0
  local alias host user key remote local
  while IFS='|' read -r alias host user key remote local; do
    [[ -z "${alias}" || "${alias}" == \#* ]] && continue
    echo "==> ${sub_action} ${alias} (${host})"
    if ! VMLENS_CONFIG="${config_file}" "$0" "${sub_action}" "${alias}"; then
      failed=1
    fi
  done <"${vm_inventory}"
  return "${failed}"
}

case "${action}" in
  list) list_vms; exit $? ;;
  start-all) run_for_all start; exit $? ;;
  stop-all) run_for_all stop; exit $? ;;
  restart-all) run_for_all restart; exit $? ;;
  status-all) run_for_all status; exit $? ;;
esac

if [[ -z "${vm_host}" ]]; then
  usage >&2
  exit 1
fi

resolve_vm "${vm_host}"
ssh_key="$(prepare_ssh_key "${ssh_key}")"
safe_host="${vm_host//[^A-Za-z0-9_.-]/_}"
control_path="${state_dir}/${ssh_user}_${safe_host}.ctl"

ssh_common=(
  ssh
  -o ExitOnForwardFailure=yes
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
  -o StrictHostKeyChecking=accept-new
  -S "${control_path}"
)
if ! is_disabled_key "${ssh_key}"; then
  ssh_common+=(-i "${ssh_key}" -o IdentitiesOnly=yes)
fi

target="${ssh_user}@${vm_host}"

is_running() {
  "${ssh_common[@]}" -O check "${target}" >/dev/null 2>&1
}

start_tunnel() {
  mkdir -p "${state_dir}"
  if is_running; then
    echo "VMLens tunnel already running for ${vm_alias}: ${target}"
    return
  fi
  "${ssh_common[@]}" -M -fN -R "${remote_backend}:${local_backend}" "${target}"
  echo "VMLens tunnel started: ${vm_alias} ${target} ${remote_backend} -> ${local_backend}"
}

stop_tunnel() {
  if is_running; then
    "${ssh_common[@]}" -O exit "${target}" >/dev/null
    echo "VMLens tunnel stopped for ${vm_alias}: ${target}"
  else
    echo "VMLens tunnel is not running for ${vm_alias}: ${target}"
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
