#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  bash scripts/vmlens-stack.sh start [--with-deepflow|--core]
  bash scripts/vmlens-stack.sh stop [--with-deepflow|--core]
  bash scripts/vmlens-stack.sh restart [--with-deepflow|--core]
  bash scripts/vmlens-stack.sh status [--with-deepflow|--core]
  bash scripts/vmlens-stack.sh logs [--with-deepflow|--core] [service]
  bash scripts/vmlens-stack.sh health
  bash scripts/vmlens-stack.sh deepflow-domain
  bash scripts/vmlens-stack.sh grafana-refresh

Default mode is --with-deepflow.
EOF
}

mode="--with-deepflow"
command="${1:-}"
if [[ -z "${command}" || "${command}" == "-h" || "${command}" == "--help" ]]; then
  usage
  exit 0
fi
shift || true

if [[ "${1:-}" == "--with-deepflow" || "${1:-}" == "--core" ]]; then
  mode="$1"
  shift || true
fi

compose_args=(-f docker-compose.yml)
if [[ "${mode}" == "--with-deepflow" ]]; then
  compose_args+=(-f docker-compose.deepflow.yml)
fi

run_compose() {
  (cd "${ROOT_DIR}" && docker compose "${compose_args[@]}" "$@")
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
      curl -fsS http://127.0.0.1:8080/api/deepflow/health
      printf '\n'
    elif [[ "${mode}" == "--with-deepflow" ]]; then
      run_compose exec -T deepflow-clickhouse wget -qO- http://control-plane:8080/health
      printf '\n'
      run_compose exec -T deepflow-clickhouse wget -qO- http://control-plane:8080/api/deepflow/health
      printf '\n'
    else
      echo "VMLens API is not reachable on http://127.0.0.1:8080" >&2
      exit 1
    fi
    ;;
  deepflow-domain)
    bash "${ROOT_DIR}/scripts/deepflow-ensure-domain.sh"
    ;;
  grafana-refresh)
    bash "${ROOT_DIR}/scripts/deepflow-set-grafana-refresh.sh"
    ;;
  *)
    usage
    exit 1
    ;;
esac
