#!/usr/bin/env bash
set -euo pipefail

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3001}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-deepflow}"
GRAFANA_REFRESH="${GRAFANA_REFRESH:-5s}"
GRAFANA_FROM="${GRAFANA_FROM:-now-1h}"
GRAFANA_TO="${GRAFANA_TO:-now}"

dashboards=(
  "Network_Flow_Log_Cloud|VMLens_Network_Flow_Log_Live|VMLens Live - Network Flow Log"
  "Application_Request_Log_Cloud|VMLens_Request_Log_Live|VMLens Live - Request Log"
  "Network_Cloud_Host|VMLens_Network_Cloud_Host_Live|VMLens Live - Network Cloud Host"
  "Application_Cloud_Host|VMLens_Application_Cloud_Host_Live|VMLens Live - Application Cloud Host"
)

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require curl
require jq

for item in "${dashboards[@]}"; do
  IFS="|" read -r source_uid target_uid target_title <<<"${item}"
  echo "creating/updating VMLens Grafana dashboard: ${target_uid} -> ${GRAFANA_REFRESH}"
  dashboard_json="$(curl -fsS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    "${GRAFANA_URL}/api/dashboards/uid/${source_uid}")"

  payload="$(jq -c \
    --arg uid "${target_uid}" \
    --arg title "${target_title}" \
    --arg refresh "${GRAFANA_REFRESH}" \
    --arg from "${GRAFANA_FROM}" \
    --arg to "${GRAFANA_TO}" \
    '
      .dashboard.id = null
      | .dashboard.uid = $uid
      | .dashboard.title = $title
      | .dashboard.refresh = $refresh
      | .dashboard.time.from = $from
      | .dashboard.time.to = $to
      | {
          dashboard: .dashboard,
          overwrite: true,
          message: "Create VMLens live dashboard"
        }
        + (if .meta.folderUid then {folderUid: .meta.folderUid} else {} end)
    ' <<<"${dashboard_json}")"

  curl -fsS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    -H "Content-Type: application/json" \
    -X POST \
    -d "${payload}" \
    "${GRAFANA_URL}/api/dashboards/db" >/dev/null
done

echo "Grafana dashboard refresh updated."
