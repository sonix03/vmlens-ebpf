# Prebuilt VM Agent

Use this flow when a VM should install release binaries only, without compiling
Go or eBPF programs on the VM.

## Build artifacts locally

Run on a Linux machine with Go, clang with BPF target support, bpftool and
kernel BTF.

```bash
bash scripts/build-agent-release.sh
```

Output:

```text
dist/agent/<version>/vmlens-agent-linux-amd64
dist/agent/<version>/flow_tracker-linux-amd64.bpf.o
dist/agent/<version>/install-agent.sh
dist/agent/<version>/SHA256SUMS
```

## Build artifacts with GitHub Actions

```bash
git tag v0.1.0
git push origin v0.1.0
```

Release assets:

```text
vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
install-agent.sh
SHA256SUMS
```

## Install on a VM

Start the tunnel from local first:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.ssh/id_ed25519_vmlens
```

Run on the VM:

```bash
curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/latest/download/install-agent.sh

chmod +x /tmp/vmlens-install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  CONNECTIVITY_PROBE_ENABLED=true \
  CONNECTIVITY_PROBE_INTERVAL=5s \
  CONNECTIVITY_PROBE_LISTEN_ADDR=0.0.0.0:18081 \
  IGNORE_PORTS=18080,18081,18082 \
  bash /tmp/vmlens-install-agent.sh
```

Check:

```bash
systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 80 --no-pager
sudo tc filter show dev ens3 ingress
sudo tc filter show dev ens3 egress
```

## Runtime requirement

Real eBPF mode requires kernel BTF on the VM:

```bash
test -r /sys/kernel/btf/vmlinux
```

Ubuntu 24.04 images usually have this.

## Capture mode

```text
CAPTURE_MODE=auto    try Traffic Control first, then fallback to kprobe
CAPTURE_MODE=tc      require TC/TCX on CAPTURE_INTERFACE
CAPTURE_MODE=kprobe  use socket-level kprobes only
```

For OpenStack Ubuntu VMs, `ens3` is usually the primary interface:

```bash
CAPTURE_MODE=tc CAPTURE_INTERFACE=ens3
```

The reverse SSH tunnel only carries agent telemetry to the local backend:

```text
BACKEND_URL=http://127.0.0.1:18080
```

Captured application traffic still comes from `CAPTURE_INTERFACE`.

When the installer is run over SSH, it auto-detects the SSH client IP and
appends it to `IGNORE_IPS` and `FLOW_DENY_CIDRS`. This prevents the telemetry
tunnel itself from being counted as VM external traffic. Disable this only when
debugging:

```bash
AUTO_DENY_TUNNEL_PEER=false
```
