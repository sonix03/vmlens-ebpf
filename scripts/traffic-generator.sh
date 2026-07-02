#!/usr/bin/env bash
set -euo pipefail
url="${1:-https://speed.hetzner.de/10MB.bin}"
echo "Downloading a bounded test object from $url"
curl --fail --location --max-time 60 --output /dev/null "$url"
