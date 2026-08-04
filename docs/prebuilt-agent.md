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

`VERSION` is stamped into the binary, and that is what the agent reports as
`agent_version`. A build with no `VERSION` reports `dev`, which is the honest
answer for a binary that did not come from a release.

The eBPF object is compiled against the build host's kernel BTF, but every
kernel struct is read through `BPF_CORE_READ`, so the object carries CO-RE
relocations and the offsets are resolved against the target VM's own BTF at
load time. One object therefore works across kernel versions. It does not work
across architectures: build each arch on its own machine.

## Build artifacts with GitHub Actions

```bash
git tag v0.1.0
git push origin v0.1.0
```

Release assets:

```text
vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
vmlens-agent-linux-arm64
flow_tracker-linux-arm64.bpf.o
install-agent.sh
SHA256SUMS
```

Each architecture builds on a runner of that architecture, because the eBPF
object cannot be cross-compiled: `struct pt_regs` differs per arch, so an arm64
object built against an x86 `vmlinux.h` would read the wrong registers. The
arm64 job is allowed to fail without holding back the amd64 release, so check
the release actually carries the arm64 assets before telling anyone to use them.

`SHA256SUMS` covers every asset in the release and is written by the publish
job after both architectures are collected.

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
  REQUIRE_CHECKSUM=true \
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

## Checksum verification

Downloaded artifacts are checked against the `SHA256SUMS` published beside them.
The installer derives its URL from the binary URL, so a normal release install
needs no extra flag.

```text
REQUIRE_CHECKSUM=false  default; a missing SHA256SUMS warns and continues
REQUIRE_CHECKSUM=true   refuse to install unless every artifact is verified
```

A checksum that does not match always aborts, in both modes. Local files given
via `AGENT_BINARY_PATH` / `BPF_OBJECT_PATH` are not checked: they are already in
the operator's hands.

## Check

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
