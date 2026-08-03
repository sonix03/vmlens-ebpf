# VMLens eBPF

Real-time VM network relationship tracking with a TC/eBPF agent.

<p align="center">
  <a href="https://github.com/sonix03/vmlens-ebpf/releases/latest">
    <img alt="Release" src="https://img.shields.io/github/v/release/sonix03/vmlens-ebpf?style=for-the-badge&color=3FAB48">
  </a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-TypeScript-61DAFB?style=for-the-badge&logo=react&logoColor=111">
  <img alt="eBPF" src="https://img.shields.io/badge/TC%2FeBPF-network%20tracker-purple?style=for-the-badge">
</p>

VMLens observes VM-to-VM and VM-to-external network relationships, then renders
them as topology edges in a local dashboard. The telemetry source is the
project's own VM agent, which attaches TC/eBPF programs to the VM network
interface, usually `ens3`.

```text
Cloud VM A ─┐
            │  TC/eBPF flow metadata
Cloud VM B ─┼── vmlens-agent ── reverse SSH tunnel ── control-plane ── dashboard
            │
External IP ┘
```

## What VMLens tracks

- VM identity, hostname, interfaces, private/public IPs and agent status.
- TCP/UDP/ICMP source and destination IP/port.
- Sent bytes, received bytes and packet count.
- Connection/request attempt counters.
- Error counters for failed attempts such as refused TCP connections.
- Internal vs external traffic based on registered VM inventory.
- Connectivity probe RTT for idle connection state.
- Application delay, measured from socket timing between a request being sent
  and the first response byte arriving.
- HTTP response status class for plaintext HTTP/1.x, as counters and the most
  recent code.
- Live topology state: green idle connection, yellow slow RTT, red failed path,
  moving dots for active request traffic.

VMLens does not capture HTTP bodies, TLS plaintext, SSH content, database
queries, files, command lines, request/response bodies, URLs, headers, or
cookies.

The single exception to payload reading is the HTTP status line: eBPF inspects
the first bytes of a plaintext TCP payload in kernel space and extracts only the
three status digits as an integer. No payload bytes ever leave the kernel.
Encrypted traffic yields no status at all. See `docs/privacy.md`.

## Quick start

### 1. Start local stack

```bash
bash scripts/vmlens-stack.sh start
```

Equivalent raw compose command:

```bash
docker compose up -d --build
```

Services:

```text
Dashboard  http://localhost:3000
API        http://localhost:8080
Postgres   localhost:5432
```

Check:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
```

### 2. Start reverse tunnel to each VM

Run on local machine:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.ssh/id_ed25519_vmlens
```

Example:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130 ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh start 10.20.20.199 ~/.ssh/id_ed25519_vmlens
```

The tunnel exposes the local API inside the VM:

```text
VM http://127.0.0.1:18080 -> local http://127.0.0.1:8080
```

Status:

```bash
bash scripts/vmlens-tunnel.sh status <VM_IP> ~/.ssh/id_ed25519_vmlens
```

### 3. Install the TC/eBPF agent on each VM

Using release artifacts:

```bash
curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/latest/download/install-agent.sh

chmod +x /tmp/vmlens-install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  FLOW_DEBUG=false \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  CONNECTIVITY_PROBE_ENABLED=true \
  CONNECTIVITY_PROBE_INTERVAL=5s \
  CONNECTIVITY_PROBE_LISTEN_ADDR=0.0.0.0:18081 \
  IGNORE_PORTS=18080,18081,18082 \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh
```

For source-based reinstall from zero, use
`docs/runbooks/from-scratch-source-download.md`. To toggle flow debug after
install:

```bash
bash scripts/vmlens-agent.sh debug-on
sudo journalctl -u vmlens-agent -f | grep flow_debug
bash scripts/vmlens-agent.sh debug-off
```

Check on the VM:

```bash
systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 80 --no-pager
sudo tc filter show dev ens3 ingress
sudo tc filter show dev ens3 egress
```

Expected log signal:

```text
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
TRACER_RUNNING
```

## Test VM-to-VM traffic

Server VM:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

Client VM:

```bash
for i in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 http://<SERVER_VM_IP>:8081/ || true)
  echo "request=$i status=$code"
  sleep 0.2
done
```

Local verification:

```bash
curl http://127.0.0.1:8080/api/stats/summary
curl 'http://127.0.0.1:8080/api/internal/activity?limit=10'
curl 'http://127.0.0.1:8080/api/flows?limit=20'
curl http://127.0.0.1:8080/api/graph
```

UI behavior:

```text
green idle line   = VM reachability/connectivity is healthy
yellow idle line  = RTT is slow
red line/row      = failed attempt, usually refused port or timeout
moving line       = active request/traffic on the same edge
```

## Common operations

```bash
bash scripts/vmlens-stack.sh status
bash scripts/vmlens-stack.sh logs
bash scripts/vmlens-stack.sh restart
bash scripts/vmlens-stack.sh stop
docker compose down -v
```

Tunnel:

```bash
bash scripts/vmlens-tunnel.sh list
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh stop <VM_IP> ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh forget-host <VM_IP>
```

Agent:

```bash
sudo systemctl restart vmlens-agent
sudo systemctl stop vmlens-agent
sudo journalctl -u vmlens-agent -f
```

## Traffic classification

VMLens treats a flow as internal only when the destination IP belongs to a
registered VM in the inventory.

```text
internal_same_tenant   registered source VM -> registered destination VM, same tenant
internal_cross_tenant  registered source VM -> registered destination VM, different tenant
external_private       private destination IP not registered as a VM
external_public        public destination IP
unknown_internal       optional discovery mode for unregistered private IPs
```

Enable discovery mode:

```bash
UNREGISTERED_INTERNAL_SCOPE=unknown_internal docker compose up -d --build
```

## Repository layout

```text
agent/        VM agent and TC/eBPF programs
backend/      Go control-plane API, graph, stats and database migrations
frontend/     React dashboard
scripts/      stack, tunnel, install, release and test helpers
configs/      local tunnel/VM profile examples
deploy/       deployment assets, currently OpenStack cloud-init
docs/         setup guides, runbooks, architecture and privacy notes
```

## API quick reference

```text
GET  /health
GET  /api/agents
GET  /api/vms
GET  /api/graph
GET  /api/flows
GET  /api/stats/summary
GET  /api/internal/activity
GET  /api/realtime
POST /api/agents/register
POST /api/agents/heartbeat
POST /api/flows/ingest
POST /api/connections/probe
```

## Requirements

Local:

- Docker Desktop or Docker Engine with Compose.
- Ports `3000`, `5432`, and `8080` available.
- SSH access to the VMs.

VM:

- Linux amd64.
- Root/sudo access for systemd service and eBPF load.
- Kernel BTF:

```bash
test -r /sys/kernel/btf/vmlinux
```

## Security note

This project currently targets development and controlled lab usage. Do not
expose the local backend publicly without TLS, authentication and ingest
authorization.
