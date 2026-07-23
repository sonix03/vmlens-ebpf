# Prebuilt VM Agent

Use this flow when VMs should install binaries only, without compiling Go or
eBPF programs on the VM.

## Build artifacts locally

Run on a Linux machine with Go, clang with BPF target support, and libbpf-dev.
If bpftool plus kernel BTF is unavailable, the build script uses the fallback
vmlinux header stored in this repo.

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

Push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow uploads release assets:

```text
vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
install-agent.sh
SHA256SUMS
```

## Install prebuilt artifacts on a VM with repo cloned

```bash
sudo env \
  INSTALL_MODE=prebuilt \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  bash scripts/vmlens-agent.sh start
```

## Install VMLens + DeepFlow agent from prebuilt release

Use this when the local stack is started with DeepFlow and the VM should send:

- realtime topology metadata to VMLens;
- L4/L7 telemetry to the integrated DeepFlow server.

Start the tunnel from local first:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
```

Then run on the VM:

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
  INSTALL_DEEPFLOW_AGENT=true \
  INSTALL_DEEPFLOW_RELAY=true \
  DEEPFLOW_AGENT_VERSION=v6.6.1 \
  bash /tmp/vmlens-install-agent.sh
```

The DeepFlow relay exposes the tunneled controller/ingester ports on the VM
interface. This lets the DeepFlow agent connect through the same SSH tunnel
without exposing local laptop ports publicly.

Check:

```bash
systemctl is-active vmlens-agent
systemctl is-active deepflow-agent
systemctl is-active vmlens-deepflow-controller-relay
systemctl is-active vmlens-deepflow-ingester-relay
sudo journalctl -u deepflow-agent -n 80 --no-pager
```

Recommended VM size for the combined VMLens + DeepFlow agent is at least 1 GB
RAM. Very small 512 MB lab VMs can work with swap, but the DeepFlow agent may
hit memory circuit breakers under load.

## Install prebuilt artifacts on a VM without git clone

Use this OpenStack Customization Script:

```text
deploy/openstack/openstack-vmlens-prebuilt-cloud-init.yaml
```

The VM only installs:

```text
ca-certificates
curl
```

It does not install Go, clang, libbpf-dev, or bpftool.

## Runtime requirement

Real eBPF mode still requires kernel BTF on the VM:

```bash
test -r /sys/kernel/btf/vmlinux
```

Ubuntu 24.04 images usually have this.

## Capture mode

```text
CAPTURE_MODE=auto    try Traffic Control first, then fallback to kprobe
CAPTURE_MODE=tc      require TCX/Traffic Control on CAPTURE_INTERFACE
CAPTURE_MODE=kprobe  use socket-level kprobes only
```

For OpenStack Ubuntu 24.04 VMs, `ens3` is usually the primary interface:

```bash
CAPTURE_MODE=tc CAPTURE_INTERFACE=ens3
```

The reverse SSH tunnel still uses `BACKEND_URL=http://127.0.0.1:18080`.
That loopback address is only the telemetry path from the agent to the local
backend. Captured application traffic comes from `CAPTURE_INTERFACE`.

When `scripts/install-agent.sh` or `scripts/vmlens-agent.sh start` is run over
SSH, the installer auto-detects the SSH client IP and appends it to
`IGNORE_IPS` and `FLOW_DENY_CIDRS`. This prevents the backend tunnel itself
from being counted as VM external traffic. Disable this only when debugging:

```bash
AUTO_DENY_TUNNEL_PEER=false
```

If DeepFlow or another eBPF tool owns incompatible TC hooks on the same
interface, use:

```bash
CAPTURE_MODE=kprobe
```
