#!/usr/bin/env bash
set -euo pipefail
duration="${1:-35}"
echo "Generating one-core CPU load for ${duration}s"
timeout "$duration" sh -c 'while :; do :; done' || [[ $? -eq 124 ]]
