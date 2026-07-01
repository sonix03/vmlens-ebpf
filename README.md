# VMLens

**VMLens: eBPF-Based SSH Session and Resource Usage Monitor for Linux VMs**

VMLens is a lightweight, local-first Linux VM observability agent. It connects SSH login metadata to descendant processes and their CPU, memory, disk I/O, and network connection metadata so an administrator can explain resource spikes without recording terminal contents.

Typical questions:

- Which SSH user started the CPU-heavy script?
- Which process caused a memory or disk spike?
- Why did outbound internet usage increase?
- Which commands and process tree belong to an active SSH session?

## Safety and privacy

VMLens is for authorized monitoring of systems you own or administer. It does **not** capture passwords, private keys, keystrokes, terminal input/output, file contents, packet payloads, or decrypted TLS traffic. Command arguments are sanitized before logging. Data remains in local JSON Lines files; remote upload is not implemented. See [privacy details](docs/privacy.md).

## Current v0.1 behavior

- SSH events: follows `/var/log/auth.log`, with `journalctl` fallback.
- Process lifecycle: `/proc` polling fallback is operational; eBPF exec/exit source programs are provided and built separately.
- CPU/RSS/disk: sampled from `/proc/<pid>`.
- Network: TCP connection metadata is mapped from socket inode to PID. Fallback RX/TX values are zero; packet payload is never read.
- Correlation: follows PID ancestry rooted at the accepted `sshd` process. It does not guess based on UID alone.
- Metrics: bound to `127.0.0.1:9435` by default.

The eBPF objects are optional in v0.1 and require BTF, clang, bpftool, and libbpf headers (`make bpf`). The host fallback keeps the agent functional on systems where eBPF cannot be built or loaded. See [architecture](docs/architecture.md) for limitations.

## Install on Ubuntu/Debian over SSH

```bash
git clone https://github.com/YOUR_ORG/vmlens-ebpf.git
cd vmlens-ebpf
sudo ./scripts/install.sh
sudo systemctl enable --now vmlens
sudo journalctl -u vmlens -f
```

Build manually (Go 1.22+):

```bash
make build
sudo make install
sudo systemctl enable --now vmlens
curl http://localhost:9435/metrics
```

Manual run:

```bash
sudo ./bin/vmlens run --config configs/vmlens.yaml
```

## CLI

```text
vmlens run
vmlens ssh sessions
vmlens ssh watch
vmlens ssh inspect <session_id>
vmlens top [--by cpu|memory|network|disk] [--session <session_id>]
vmlens processes
vmlens version
```

Non-root CLI queries may be denied because logs are intentionally mode `0640`. Use `sudo` or grant a dedicated read-only group access.

Logs are written under `/var/log/vmlens/`: `ssh_sessions.log`, `process_events.log`, `resource_events.log`, `network_flows.log`, and `analysis.log`. Each line is independent JSON.

## Test flow

```bash
# SSH into VM
ssh ubuntu@YOUR_VM_IP

# Start VMLens and watch analysis
sudo systemctl enable --now vmlens
sudo vmlens ssh watch

# In another SSH session, generate bounded workloads
bash examples/network-spike.sh
bash examples/cpu-spike.sh

# Inspect attribution
sudo vmlens ssh sessions
sudo vmlens ssh inspect <session_id>
sudo vmlens top --by cpu --session <session_id>
```

The examples are deliberately bounded, but still consume real VM resources. Do not run them on a production VM without capacity review.

## Prometheus and Grafana

```bash
docker compose -f deploy/docker-compose.monitoring.yml up -d
```

On Linux, Compose uses `host-gateway`. If unavailable, replace `host.docker.internal` in `deploy/prometheus.yml` with the host bridge IP, or run Prometheus with host networking. Because VMLens binds loopback by default, a container cannot reach it through the bridge; either change `listen_addr` to a controlled host address with firewall rules or run Prometheus using `network_mode: host` and target `127.0.0.1:9435`.

Remote access should use SSH tunnels:

```bash
ssh -L 9435:localhost:9435 ubuntu@YOUR_VM_IP
ssh -L 3000:localhost:3000 ubuntu@YOUR_VM_IP
```

Default Prometheus labels exclude session IDs, commands, remote IPs, and destination IPs to control cardinality and information exposure.

## Troubleshooting

- Permission denied: run the agent as root; eBPF and `/proc/<pid>` access require privileges.
- Missing kernel headers/BTF: install matching headers; the `/proc` fallback still works.
- eBPF unsupported: use the fallback and inspect service logs for the exact loader/build error.
- Missing `auth.log`: enable journald parsing; Ubuntu with journal-only SSH logging is supported.
- Journald permission: the systemd unit runs as root; manual users need journal access.
- Prometheus cannot scrape: remember VMLens binds loopback and containers have a separate network namespace.
- High cardinality: never add session, IP, or full-command labels to shared Prometheus installations.

More detail is in [troubleshooting](docs/troubleshooting.md). Contributions must preserve the privacy boundary and include tests. Licensed under MIT.
