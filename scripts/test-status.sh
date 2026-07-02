#!/usr/bin/env bash
set -euo pipefail

echo "Agent health:"
curl -fsS http://127.0.0.1:9109/healthz

echo
echo "Prometheus health:"
curl -fsS http://127.0.0.1:9090/-/ready

echo
echo "Grafana health:"
curl -fsS http://127.0.0.1:3000/api/health

echo
echo "Prometheus VMLens target:"
curl -fsS http://127.0.0.1:9090/api/v1/targets |
  jq '.data.activeTargets[] | select(.labels.job == "vmlens-agent") | {scrapeUrl, health, lastError}'

echo
echo "Current vmlens_agent_up:"
curl -fsS --get --data-urlencode 'query=vmlens_agent_up' \
  http://127.0.0.1:9090/api/v1/query | jq '.data.result'
