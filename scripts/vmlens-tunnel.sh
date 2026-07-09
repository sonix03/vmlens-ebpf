#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
config_file="${VMLENS_CONFIG:-${repo_dir}/configs/local.env}"
if [[ -f "${config_file}" ]]; then
  # shellcheck disable=SC1090
  source "${config_file}"
fi

usage() {
  cat <<EOF
Usage:
  $0 list
  $0 show <vm-alias-or-host>
  $0 start|stop|restart|status <vm-alias-or-host> [ssh-key|agent|none]
  $0 start-all|stop-all|restart-all|status-all
  $0 forget-host <vm-alias-or-host>

Config:
  VMLENS_CONFIG=${config_file}

Env override:
  VMLENS_SSH_USER=ubuntu
  VMLENS_SSH_KEY=~/.ssh/id_ed25519_vmlens | agent | none
  VMLENS_LOCAL_BACKEND=127.0.0.1:8080
  VMLENS_REMOTE_BACKEND=127.0.0.1:18080
  VMLENS_VM_PROFILES="testing_a_1 testing_a_2"
  VMLENS_VM_INVENTORY=configs/vms.local  # optional legacy file
EOF
}

action="${1:-}"
vm_host="${2:-}"
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
    echo "SSH key not found: ${requested}" >&2
    echo "Set VMLENS_SSH_KEY, SSH_KEY, or per-VM VMLENS_VM_<PROFILE>_SSH_KEY in ${config_file}" >&2
    return 1
  fi

  if [[ "${key_path}" == /mnt/c/* ]]; then
    mkdir -p "${key_state_dir}"
    install -m 0600 "${key_path}" "${key_state_dir}/id_ed25519_vmlens"
    key_path="${key_state_dir}/id_ed25519_vmlens"
  fi

  printf '%s\n' "${key_path}"
}

default_ssh_user="${SSH_USER:-${VMLENS_SSH_USER:-ubuntu}}"
default_ssh_key="${SSH_KEY:-${VMLENS_SSH_KEY:-}}"
if [[ -z "${default_ssh_key}" ]]; then
  default_ssh_key="$(discover_default_key || true)"
fi
default_ssh_key="${default_ssh_key:-~/.ssh/id_ed25519_vmlens}"
default_local_backend="${LOCAL_BACKEND:-${VMLENS_LOCAL_BACKEND:-127.0.0.1:8080}}"
default_remote_backend="${REMOTE_BACKEND:-${VMLENS_REMOTE_BACKEND:-127.0.0.1:18080}}"
state_dir="$(expand_path "${VMLENS_TUNNEL_STATE_DIR:-${HOME}/.vmlens/tunnels}")"
key_state_dir="$(expand_path "${VMLENS_KEY_STATE_DIR:-${HOME}/.vmlens/keys}")"
vm_profiles="${VMLENS_VM_PROFILES:-}"
vm_inventory="${VMLENS_VM_INVENTORY:-${repo_dir}/configs/vms.local}"
if [[ "${vm_inventory}" != /* ]]; then
  vm_inventory="${repo_dir}/${vm_inventory}"
fi
if [[ -z "${vm_profiles}" && ! -f "${vm_inventory}" && -f "${repo_dir}/configs/vms.example" ]]; then
  vm_inventory="${repo_dir}/configs/vms.example"
fi

profile_env_name() {
  local profile="$1"
  profile="${profile^^}"
  profile="${profile//-/_}"
  printf '%s\n' "${profile}"
}

env_value() {
  local name="$1"
  printf '%s\n' "${!name:-}"
}

profile_value() {
  local profile="$1"
  local field="$2"
  local normalized
  normalized="$(profile_env_name "${profile}")"
  env_value "VMLENS_VM_${normalized}_${field}"
}

has_env_profiles() {
  [[ -n "${vm_profiles// }" ]]
}

load_defaults() {
  vm_alias="${1:-}"
  vm_host="${1:-}"
  ssh_user="${default_ssh_user}"
  ssh_key="${default_ssh_key}"
  local_backend="${default_local_backend}"
  remote_backend="${default_remote_backend}"
}

apply_profile() {
  local profile="$1"
  local alias host user key remote local
  alias="$(profile_value "${profile}" "ALIAS")"
  host="$(profile_value "${profile}" "HOST")"
  user="$(profile_value "${profile}" "SSH_USER")"
  key="$(profile_value "${profile}" "SSH_KEY")"
  remote="$(profile_value "${profile}" "REMOTE_BACKEND")"
  local="$(profile_value "${profile}" "LOCAL_BACKEND")"

  vm_alias="${alias:-${profile}}"
  vm_host="${host:-${vm_alias}}"
  [[ -n "${user:-}" && "${user}" != "-" ]] && ssh_user="${user}"
  [[ -n "${key:-}" && "${key}" != "-" ]] && ssh_key="${key}"
  [[ -n "${remote:-}" && "${remote}" != "-" ]] && remote_backend="${remote}"
  [[ -n "${local:-}" && "${local}" != "-" ]] && local_backend="${local}"
  return 0
}

print_vm_row() {
  local selector="$1"
  resolve_vm "${selector}"
  local display_key
  if is_disabled_key "${ssh_key}"; then
    display_key="agent"
  else
    display_key="$(expand_path "${ssh_key}")"
  fi
  printf '%-16s %-16s %-10s %-58s %-22s %-22s\n' \
    "${vm_alias}" \
    "${vm_host}" \
    "${ssh_user}" \
    "${display_key}" \
    "${remote_backend}" \
    "${local_backend}"
}

for_each_vm() {
  local callback="$1"
  if has_env_profiles; then
    local profile
    for profile in ${vm_profiles}; do
      "${callback}" "${profile}"
    done
    return 0
  fi

  if [[ ! -f "${vm_inventory}" ]]; then
    echo "VM inventory not found: ${vm_inventory}" >&2
    return 1
  fi
  local alias host user key remote local
  while IFS='|' read -r alias host user key remote local; do
    [[ -z "${alias}" || "${alias}" == \#* ]] && continue
    "${callback}" "${alias}"
  done <"${vm_inventory}"
}

list_vms() {
  printf '%-16s %-16s %-10s %-58s %-22s %-22s\n' "ALIAS" "HOST" "USER" "KEY" "REMOTE_BACKEND" "LOCAL_BACKEND"
  for_each_vm print_vm_row
}

resolve_vm() {
  local selector="$1"
  load_defaults "${selector}"

  if has_env_profiles; then
    local profile alias host
    for profile in ${vm_profiles}; do
      alias="$(profile_value "${profile}" "ALIAS")"
      host="$(profile_value "${profile}" "HOST")"
      if [[ "${selector}" == "${profile}" || "${selector}" == "${alias}" || "${selector}" == "${host}" ]]; then
        apply_profile "${profile}"
        return
      fi
    done
    return
  fi

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
  if ! has_env_profiles && [[ ! -f "${vm_inventory}" ]]; then
    echo "VM inventory not found: ${vm_inventory}" >&2
    exit 1
  fi
  local failed=0
  run_one() {
    local selector="$1"
    resolve_vm "${selector}"
    echo "==> ${sub_action} ${vm_alias} (${vm_host})"
    if ! VMLENS_CONFIG="${config_file}" "$0" "${sub_action}" "${vm_alias}"; then
      failed=1
    fi
  }
  for_each_vm run_one
  return "${failed}"
}

forget_known_host() {
  local selector="$1"
  resolve_vm "${selector}"

  local known_hosts_file="${HOME}/.ssh/known_hosts"
  if [[ ! -f "${known_hosts_file}" ]]; then
    echo "known_hosts not found: ${known_hosts_file}"
    return 0
  fi

  local entry seen_entries=" "
  for entry in "${selector}" "${vm_host}" "${vm_alias}"; do
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
    if [[ -z "${vm_host}" ]]; then
      usage >&2
      exit 1
    fi
    printf '%-16s %-16s %-10s %-58s %-22s %-22s\n' "ALIAS" "HOST" "USER" "KEY" "REMOTE_BACKEND" "LOCAL_BACKEND"
    print_vm_row "${vm_host}"
    exit $?
    ;;
  forget-host)
    if [[ -z "${vm_host}" ]]; then
      usage >&2
      exit 1
    fi
    forget_known_host "${vm_host}"
    exit 0
    ;;
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
if [[ -n "${key_arg}" ]]; then
  ssh_key="${key_arg}"
fi
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
