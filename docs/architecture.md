# Architecture

An accepted SSH connection creates an `sshd` child, which launches a login shell and descendant processes. VMLens parses the accepted-login record, stores that `sshd` PID as a session root, and assigns later descendants by walking cached PPID relationships.

```text
sshd log ──> SSH collector ───────────────┐
                                          v
exec/exit ─> eBPF or /proc fallback ─> correlator ─> JSONL ─> CLI
                                               │
/proc stat/status/io ─> resource sampler ──────┤
socket metadata ──────> network collector ─────┤
                                               ├─> rule analyzer
                                               └─> Prometheus exporter ─> Grafana
```

The active VM-side eBPF program lives under `agent/ebpf/programs/`, with
fallback headers under `agent/ebpf/include/`. Older prototype CO-RE programs
were moved to `legacy/v1-stack/bpf/` for reference.

CPU is computed from deltas in process scheduler ticks. RSS comes from `/proc/<pid>/stat`; cumulative storage bytes come from `/proc/<pid>/io`. TCP sockets are joined to process file descriptors by inode. This gives correct ownership metadata but no byte-accurate network accounting; RX/TX remain zero in fallback mode.

All collectors emit typed events. The agent attaches a session ID, writes category-specific JSONL, updates low-cardinality metrics, and evaluates threshold rules. It has no remote transport.
