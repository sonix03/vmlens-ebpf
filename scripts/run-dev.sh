#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_dir"
exec go run ./cmd/vmlens-agent --config config/vmlens.example.yaml --demo "$@"
