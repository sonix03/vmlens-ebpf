#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/vmlens-stack.sh start
  bash scripts/vmlens-stack.sh stop
  bash scripts/vmlens-stack.sh restart
  bash scripts/vmlens-stack.sh status
  bash scripts/vmlens-stack.sh logs [service]
  bash scripts/vmlens-stack.sh health

Core services:
  datastore
  control-plane
  dashboard
EOF
}

command="${1:-}"
if [[ -z "${command}" || "${command}" == "-h" || "${command}" == "--help" ]]; then
  usage
  exit 0
fi
shift || true

run_compose() {
  (cd "${ROOT_DIR}" && docker compose -f docker-compose.yml "$@")
}

case "${command}" in
  start)
    run_compose up -d --build
    ;;
  stop)
    run_compose down
    ;;
  restart)
    run_compose down
    run_compose up -d --build
    ;;
  status)
    run_compose ps
    ;;
  logs)
    if [[ $# -gt 0 ]]; then
      run_compose logs -f "$@"
    else
      run_compose logs -f
    fi
    ;;
  health)
    if curl -fsS http://127.0.0.1:8080/health; then
      printf '\n'
    else
      echo "VMLens API is not reachable on http://127.0.0.1:8080" >&2
      exit 1
    fi
    ;;
  *)
    usage
    exit 1
    ;;
esac
