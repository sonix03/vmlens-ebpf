#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run with sudo: sudo ./scripts/uninstall-agent.sh" >&2
  exit 1
fi

systemctl disable --now vmlens-agent 2>/dev/null || true
rm -f /etc/systemd/system/vmlens-agent.service
rm -f /usr/local/bin/vmlens-agent
rm -rf /etc/vmlens /usr/lib/vmlens
systemctl daemon-reload
echo "VMLens agent uninstalled"

