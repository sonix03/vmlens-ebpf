#!/usr/bin/env bash
set -euo pipefail
[[ "${EUID}" -eq 0 ]] || { echo "Run with sudo" >&2; exit 1; }
systemctl disable --now vmlens 2>/dev/null || true
rm -f /etc/systemd/system/vmlens.service /usr/local/bin/vmlens
systemctl daemon-reload
echo "VMLens removed. Configuration and logs were preserved in /etc/vmlens and /var/log/vmlens."
