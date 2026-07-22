#!/usr/bin/env bash
set -euo pipefail

DEEPFLOW_HOST="${DEEPFLOW_HOST:-127.0.0.1}"
DEEPFLOW_API_PORT="${DEEPFLOW_API_PORT:-30417}"
DEEPFLOW_DOMAIN_NAME="${DEEPFLOW_DOMAIN_NAME:-legacy-host}"
DEEPFLOW_CTL_PATH="${DEEPFLOW_CTL_PATH:-${TMPDIR:-/tmp}/deepflow-ctl}"
DEEPFLOW_CTL_URL="${DEEPFLOW_CTL_URL:-https://deepflow-ce.oss-cn-beijing.aliyuncs.com/bin/ctl/v6.6/linux/amd64/deepflow-ctl}"

if [[ ! -x "${DEEPFLOW_CTL_PATH}" ]]; then
  mkdir -p "$(dirname "${DEEPFLOW_CTL_PATH}")"
  curl -fsSL -o "${DEEPFLOW_CTL_PATH}" "${DEEPFLOW_CTL_URL}"
  chmod +x "${DEEPFLOW_CTL_PATH}"
fi

if "${DEEPFLOW_CTL_PATH}" -i "${DEEPFLOW_HOST}" --api-port "${DEEPFLOW_API_PORT}" domain list "${DEEPFLOW_DOMAIN_NAME}" 2>/dev/null | grep -q "${DEEPFLOW_DOMAIN_NAME}"; then
  echo "DeepFlow domain already exists: ${DEEPFLOW_DOMAIN_NAME}"
  exit 0
fi

tmpfile="$(mktemp)"
trap 'rm -f "${tmpfile}"' EXIT

cat > "${tmpfile}" <<EOF
name: ${DEEPFLOW_DOMAIN_NAME}
type: agent_sync
config:
  region_uuid: ffffffff-ffff-ffff-ffff-ffffffffffff
  controller_ip: ${DEEPFLOW_HOST}
EOF

"${DEEPFLOW_CTL_PATH}" -i "${DEEPFLOW_HOST}" --api-port "${DEEPFLOW_API_PORT}" domain create -f "${tmpfile}"
echo "DeepFlow domain created: ${DEEPFLOW_DOMAIN_NAME}"
