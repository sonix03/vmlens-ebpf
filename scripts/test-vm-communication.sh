#!/usr/bin/env bash
set -euo pipefail

peer_ip="${1:-}"
peer_port="${2:-8081}"
if [[ -z "${peer_ip}" ]]; then
  echo "Usage: $0 PEER_VM_PRIVATE_IP [PORT]" >&2
  echo "Start on peer first: python3 -m http.server 8081 --bind 0.0.0.0" >&2
  exit 1
fi

echo "Creating bounded TCP traffic to http://${peer_ip}:${peer_port}/"
for attempt in 1 2 3; do
  curl --fail --silent --show-error --max-time 5 --output /dev/null "http://${peer_ip}:${peer_port}/"
  echo "request ${attempt}/3 completed"
  sleep 1
done
echo "Flow sent. The VM-to-VM edge should appear after backend ingest/SSE refresh."
