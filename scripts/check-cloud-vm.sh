#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <cloud-vm-public-ip-or-host>"
  echo "Env: SSH_USER=ubuntu SSH_KEY=~/.ssh/id_ed25519_vmlens LOCAL_BACKEND=http://127.0.0.1:8080"
}

vm_host="${1:-}"
if [[ -z "${vm_host}" ]]; then
  usage >&2
  exit 1
fi

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
ssh_user="${SSH_USER:-ubuntu}"
local_backend="${LOCAL_BACKEND:-http://127.0.0.1:8080}"
default_ssh_key="${HOME}/.ssh/id_ed25519_vmlens"
windows_ssh_key="/mnt/c/Users/USER/.ssh/id_ed25519_vmlens"
key_state_dir="${VMLENS_KEY_STATE_DIR:-${HOME}/.vmlens/keys}"
ssh_key="${SSH_KEY:-${default_ssh_key}}"

if [[ -z "${SSH_KEY:-}" && ! -r "${default_ssh_key}" && -r "${windows_ssh_key}" ]]; then
  mkdir -p "${key_state_dir}"
  install -m 0600 "${windows_ssh_key}" "${key_state_dir}/id_ed25519_vmlens"
  ssh_key="${key_state_dir}/id_ed25519_vmlens"
fi

target="${ssh_user}@${vm_host}"
ssh_opts=(
  -i "${ssh_key}"
  -o IdentitiesOnly=yes
  -o BatchMode=yes
  -o ConnectTimeout=8
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
  -o StrictHostKeyChecking=accept-new
)

echo "1/4 Checking local backend ${local_backend}/health"
curl --fail --silent --show-error "${local_backend}/health" >/dev/null

echo "2/4 Checking SSH to ${target}"
ssh "${ssh_opts[@]}" "${target}" "hostname >/dev/null"

echo "3/4 Starting reverse tunnel"
bash "${repo_dir}/scripts/vmlens-tunnel.sh" start "${vm_host}"

echo "4/4 Checking backend from cloud VM through tunnel"
ssh "${ssh_opts[@]}" "${target}" "curl --fail --silent --show-error http://127.0.0.1:18080/health >/dev/null"

cat <<EOF
OK: cloud VM can reach the local VMLens backend through the tunnel.

Next, run this inside the cloud VM:

sudo apt-get update
sudo apt-get install -y git golang-go clang bpftool libbpf-dev

git clone <repo-url>
cd vmlens-ebpf

BACKEND_URL=http://127.0.0.1:18080 \\
AGENT_PUBLIC_IP=${vm_host} \\
CAPTURE_MODE=tc \\
CAPTURE_INTERFACE=ens3 \\
bash scripts/vmlens-agent.sh start

Then check locally:

curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
EOF
