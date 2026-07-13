#!/usr/bin/env bash
set -euo pipefail

iface="${1:-${CAPTURE_INTERFACE:-ens3}}"

echo "== interface =="
ip -brief address show "${iface}" || true

echo
echo "== vmlens service =="
systemctl status vmlens-agent --no-pager --lines=20 || true

echo
echo "== vmlens env =="
sudo sed -n '1,120p' /etc/vmlens/agent.env 2>/dev/null || true

echo
echo "== deepflow service =="
systemctl status deepflow-agent --no-pager --lines=20 2>/dev/null || true

echo
echo "== tc filters =="
sudo tc filter show dev "${iface}" ingress 2>/dev/null || true
sudo tc filter show dev "${iface}" egress 2>/dev/null || true

echo
echo "== bpftool links =="
sudo bpftool link show 2>/dev/null | grep -Ei 'vmlens|deepflow|tc|tcx|trace' || true

echo
echo "== bpftool programs =="
sudo bpftool prog show 2>/dev/null | grep -Ei 'vmlens|deepflow|tc|tcx|trace|kprobe' || true
