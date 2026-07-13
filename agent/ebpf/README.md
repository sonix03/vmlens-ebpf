# eBPF collector

`programs/flow_tracker.bpf.c` emits socket metadata only. It does not copy
packet payloads, HTTP bodies, SSH content, database queries, command lines, or
file contents.

Build on the target Linux VM:

```bash
sudo apt-get install -y clang bpftool libbpf-dev
bpftool btf dump file /sys/kernel/btf/vmlinux format c > /tmp/vmlinux.h
clang -O2 -g -target bpf -D__TARGET_ARCH_x86 -I /tmp -c agent/ebpf/programs/flow_tracker.bpf.c -o /tmp/flow_tracker.bpf.o
```

Then run the agent with `MOCK_MODE=false`, `BPF_OBJECT` pointing to the object,
and root or appropriate `CAP_BPF`, `CAP_PERFMON`, and memlock privileges.
Kernel symbol compatibility must be tested for each supported kernel family.
