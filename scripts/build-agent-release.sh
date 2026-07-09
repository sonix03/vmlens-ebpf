#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
version="${VERSION:-$(git -C "${repo_dir}" describe --tags --always --dirty 2>/dev/null || echo dev)}"
out_dir="${OUT_DIR:-${repo_dir}/dist/agent/${version}}"
target_arches="${TARGET_ARCHES:-amd64}"

command -v go >/dev/null || { echo "go is required" >&2; exit 1; }
command -v clang >/dev/null || { echo "clang is required" >&2; exit 1; }
if ! command -v bpftool >/dev/null 2>&1; then
  bpftool_path="$(find /usr/lib/linux-tools* -name bpftool -type f 2>/dev/null | head -n1 || true)"
  if [[ -n "${bpftool_path}" ]]; then
    export PATH="$(dirname "${bpftool_path}"):${PATH}"
  fi
fi
command -v bpftool >/dev/null || { echo "bpftool is required" >&2; exit 1; }
[[ -r /sys/kernel/btf/vmlinux ]] || { echo "kernel BTF /sys/kernel/btf/vmlinux is required" >&2; exit 1; }

mkdir -p "${out_dir}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

bpftool btf dump file /sys/kernel/btf/vmlinux format c >"${tmp_dir}/vmlinux.h"

for arch in ${target_arches}; do
  case "${arch}" in
    amd64) goarch=amd64; bpf_arch=x86 ;;
    arm64) goarch=arm64; bpf_arch=arm64 ;;
    *) echo "unsupported arch: ${arch}; use amd64 or arm64" >&2; exit 1 ;;
  esac

  echo "building vmlens-agent linux/${goarch}"
  (
    cd "${repo_dir}/agent"
    CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" \
      go build -trimpath -ldflags="-s -w" \
      -o "${out_dir}/vmlens-agent-linux-${arch}" ./cmd/agent
  )

  echo "building flow_tracker eBPF object for ${arch}"
  clang -O2 -g -target bpf -D"__TARGET_ARCH_${bpf_arch}" \
    -I "${tmp_dir}" \
    -c "${repo_dir}/agent/ebpf/flow_tracker.bpf.c" \
    -o "${out_dir}/flow_tracker-linux-${arch}.bpf.o"
done

cp "${repo_dir}/scripts/install-agent.sh" "${out_dir}/install-agent.sh"
chmod 0755 "${out_dir}/install-agent.sh"

(
  cd "${out_dir}"
  sha256sum install-agent.sh vmlens-agent-linux-* flow_tracker-linux-*.bpf.o >SHA256SUMS
)

echo "release artifacts written to ${out_dir}"
ls -lh "${out_dir}"
