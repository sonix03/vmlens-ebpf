# Prebuilt VM Agent

Use this flow when VMs should install binaries only, without compiling Go or
eBPF programs on the VM.

## Build artifacts locally

Run on a Linux machine with Go, clang with BPF target support, libbpf-dev,
bpftool, and kernel BTF:

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
  bash scripts/vmlens-agent.sh start
```

## Install prebuilt artifacts on a VM without git clone

Use this OpenStack Customization Script:

```text
configuration/openstack-vmlens-prebuilt-cloud-init.yaml
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
