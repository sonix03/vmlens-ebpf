#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEEPFLOW_AGENT_LOAD_THRESHOLD="${DEEPFLOW_AGENT_LOAD_THRESHOLD:-10.00}"
DEEPFLOW_AGENT_LOAD_RECOVER="${DEEPFLOW_AGENT_LOAD_RECOVER:-7.00}"
DEEPFLOW_AGENT_LOAD_METRIC="${DEEPFLOW_AGENT_LOAD_METRIC:-load15}"
DEEPFLOW_MYSQL_PASSWORD="${DEEPFLOW_MYSQL_PASSWORD:-deepflow}"

case "${DEEPFLOW_AGENT_LOAD_THRESHOLD}" in
  ""|*[!0-9.]*)
    echo "DEEPFLOW_AGENT_LOAD_THRESHOLD must be numeric" >&2
    exit 1
    ;;
esac

case "${DEEPFLOW_AGENT_LOAD_RECOVER}" in
  ""|*[!0-9.]*)
    echo "DEEPFLOW_AGENT_LOAD_RECOVER must be numeric" >&2
    exit 1
    ;;
esac

case "${DEEPFLOW_AGENT_LOAD_METRIC}" in
  ""|*[!A-Za-z0-9_-]*)
    echo "DEEPFLOW_AGENT_LOAD_METRIC must contain only letters, numbers, underscore, or dash" >&2
    exit 1
    ;;
esac

run_compose() {
  (cd "${ROOT_DIR}" && docker compose -f docker-compose.yml -f docker-compose.deepflow.yml "$@")
}

run_mysql() {
  run_compose exec -T deepflow-mysql mysql -uroot -p"${DEEPFLOW_MYSQL_PASSWORD}" deepflow
}

for _ in $(seq 1 60); do
  if run_mysql -e "SELECT 1" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

run_mysql <<SQL
SET @vmlens_default_group_lcuuid := (
  SELECT lcuuid
  FROM vtap_group
  WHERE name = 'default'
  LIMIT 1
);

INSERT INTO vtap_group_configuration (
  user_id,
  team_id,
  vtap_group_lcuuid,
  system_load_circuit_breaker_threshold,
  system_load_circuit_breaker_recover,
  system_load_circuit_breaker_metric,
  lcuuid
)
SELECT
  1,
  1,
  @vmlens_default_group_lcuuid,
  ${DEEPFLOW_AGENT_LOAD_THRESHOLD},
  ${DEEPFLOW_AGENT_LOAD_RECOVER},
  '${DEEPFLOW_AGENT_LOAD_METRIC}',
  UUID()
WHERE @vmlens_default_group_lcuuid IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM vtap_group_configuration
    WHERE vtap_group_lcuuid = @vmlens_default_group_lcuuid
  );

UPDATE vtap_group_configuration
SET
  system_load_circuit_breaker_threshold = ${DEEPFLOW_AGENT_LOAD_THRESHOLD},
  system_load_circuit_breaker_recover = ${DEEPFLOW_AGENT_LOAD_RECOVER},
  system_load_circuit_breaker_metric = '${DEEPFLOW_AGENT_LOAD_METRIC}'
WHERE vtap_group_lcuuid = @vmlens_default_group_lcuuid;

SELECT
  id,
  vtap_group_lcuuid,
  system_load_circuit_breaker_threshold,
  system_load_circuit_breaker_recover,
  system_load_circuit_breaker_metric
FROM vtap_group_configuration;
SQL

curl -fsS -X PUT 'http://127.0.0.1:30417/v1/caches/?org_id=1&type=vtap' >/dev/null \
  || echo "warning: DeepFlow vtap cache refresh failed" >&2

echo "DeepFlow agent group tuned: load threshold=${DEEPFLOW_AGENT_LOAD_THRESHOLD}, recover=${DEEPFLOW_AGENT_LOAD_RECOVER}, metric=${DEEPFLOW_AGENT_LOAD_METRIC}"
