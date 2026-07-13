#!/usr/bin/env bash
set -euo pipefail

[[ "$(uname -s)" == Linux ]] || { echo "VMLens supports Linux only" >&2; exit 1; }
[[ "${EUID}" -eq 0 ]] || { echo "Run with sudo: sudo ./legacy/v1-stack/scripts/install.sh" >&2; exit 1; }

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
make -C "$repo_dir" build
if command -v clang >/dev/null && command -v bpftool >/dev/null; then
  make -C "$repo_dir" ebpf || echo "eBPF build failed; demo mode remains available" >&2
else
  echo "clang/bpftool unavailable; skipping eBPF object build" >&2
fi

install -Dm0755 "$repo_dir/bin/vmlens-agent" /usr/local/bin/vmlens-agent
install -Dm0644 "$repo_dir/config/vmlens.example.yaml" /etc/vmlens/vmlens.yaml
if [[ -f "$repo_dir/internal/ebpf/program.bpf.o" ]]; then
  install -Dm0644 "$repo_dir/internal/ebpf/program.bpf.o" /usr/lib/vmlens/program.bpf.o
  sed -i 's#\./internal/ebpf/program.bpf.o#/usr/lib/vmlens/program.bpf.o#' /etc/vmlens/vmlens.yaml
fi

cat <<'EOF'
VMLens installed.
Real eBPF: sudo vmlens-agent --config /etc/vmlens/vmlens.yaml
Demo mode: vmlens-agent --config /etc/vmlens/vmlens.yaml --demo
Metrics: curl http://localhost:9109/metrics
EOF
