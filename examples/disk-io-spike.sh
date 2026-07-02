#!/usr/bin/env bash
set -euo pipefail
file="$(mktemp /tmp/vmlens-disk-test.XXXXXX)"
trap 'rm -f "$file"' EXIT
echo "Writing a bounded 256 MiB temporary file: $file"
dd if=/dev/zero of="$file" bs=1M count=256 conv=fsync status=progress
