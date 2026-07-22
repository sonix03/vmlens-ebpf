# VMLens eBPF

<p align="center">
  <strong>Real-time VM network relationship tracking with eBPF</strong>
</p>

<p align="center">
  <a href="https://github.com/sonix03/vmlens-ebpf/releases/tag/v2.7">
    <img alt="Release" src="https://img.shields.io/badge/release-v2.7-3FAB48?style=for-the-badge">
  </a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-TypeScript-61DAFB?style=for-the-badge&logo=react&logoColor=111">
  <img alt="eBPF" src="https://img.shields.io/badge/eBPF-network%20telemetry-purple?style=for-the-badge">
</p>

VMLens observes VM-to-VM and VM-to-external network relationships, then shows
them as live topology lines in a local dashboard.

DeepFlow is packaged as an optional local compose overlay for L4/L7 telemetry.
In that mode, VMLens reads DeepFlow raw rows, filters them by VM inventory,
deduplicates tap side duplicates, and renders VM-centric topology edges. See
[docs/deepflow-integration.md](./docs/deepflow-integration.md).

It is designed for development and lab environments where the dashboard runs on
your laptop, while lightweight agents run inside cloud VMs.

```text
Cloud VM A ─┐
            │  eBPF metadata
Cloud VM B ─┼── vmlens-agent ── reverse SSH tunnel ── local control-plane ── dashboard
            │
External IP ┘
```

## What it tracks

VMLens tracks relationship metadata only:

- VM identity, hostname, OS, kernel, interfaces, IPs and MAC addresses;
- TCP/UDP source and destination IP/port;
- sent and received bytes;
- connection/request frequency approximation;
- internal vs external traffic;
- online, stale and offline VM state;
- live active communication lines in the UI.

VMLens does not capture packet payloads, HTTP bodies, TLS plaintext, SSH
content, database queries, files, command lines, or request/response bodies.

## Traffic classification

VMLens classifies a flow as internal only when the destination IP belongs to a
registered VM in the VMLens inventory. This avoids counting an untracked private
cloud VM as internal just because it uses a `10.x`, `172.16.x` or `192.168.x`
address.

Default scopes:

- `internal_same_tenant`: source and destination are registered VMs in the same tenant;
- `internal_cross_tenant`: source and destination are registered VMs in different tenants;
- `external_private`: destination is a private IP but not a registered VM;
- `external_public`: destination is a public IP;
- `unknown_internal`: optional discovery mode for unregistered private IPs.

To enable the older discovery behavior:

```bash
UNREGISTERED_INTERNAL_SCOPE=unknown_internal docker compose up -d --build
```

## Current tested flow

The latest tested end-to-end flow uses prebuilt release assets:

```text
vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
install-agent.sh
SHA256SUMS
```

That means a VM can install the agent without:

```text
git clone
go build
clang build
bpftool build step
```

Tested with:

```text
local dashboard: Docker Compose
VM 10.20.20.130: Ubuntu, agent active
VM 10.20.20.199: Ubuntu, agent active
traffic: 10.20.20.130 -> 10.20.20.199:8081
result: 20/20 HTTP 200, internal flows and request counters increased
```

Full copy-paste E2E notes are in [SONI.txt](./SONI.txt).

For external private service VMs across zones, use the dedicated flow in
[docs/external-multizone-tracking.md](./docs/external-multizone-tracking.md).
That setup captures tracked internal VMs through Traffic Control on `ens3` while
keeping unregistered private service VMs counted as `external_private`.

## Architecture

```text
┌──────────────────────────┐
│ Cloud VM                 │
│                          │
│  vmlens-agent            │
│  ├─ register VM          │
│  ├─ heartbeat            │
│  └─ send flow metadata   │
└────────────┬─────────────┘
             │
             │ reverse SSH tunnel
             ▼
┌──────────────────────────┐
│ Local control-plane      │
│ Go API :8080             │
│ PostgreSQL               │
│ SSE realtime events      │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│ Dashboard :3000          │
│ VM graph                 │
│ live relationship lines  │
│ traffic/request metrics  │
└──────────────────────────┘
```

## Quick start

### 1. Start local dashboard

Recommended full local stack with DeepFlow:

```bash
bash scripts/vmlens-stack.sh start
```

This starts:

```text
VMLens dashboard     http://localhost:3000
VMLens API           http://localhost:8080
DeepFlow Grafana     http://localhost:3001
DeepFlow ClickHouse  http://localhost:8123
```

The packaged DeepFlow stack starts the central DeepFlow services. DeepFlow VM
agents still need to be installed on the VMs you want DeepFlow to observe. The
VMLens agent remains the realtime source for live topology lines.

Core VMLens only, without DeepFlow:

```bash
bash scripts/vmlens-stack.sh start --core
```

Raw Docker Compose equivalent for the full stack:

```bash
docker compose -f docker-compose.yml -f docker-compose.deepflow.yml up -d --build
```

Check:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/stats/summary
```

Open:

```text
http://localhost:3000
```

On a clean database, agents and VMs should be empty:

```text
[]
[]
```

### 2. Start one tunnel per VM

Run on local:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
```

Example:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh start 10.20.20.199 ~/.vmlens/keys/id_ed25519_vmlens
```

Check:

```bash
bash scripts/vmlens-tunnel.sh status <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
```

Expected:

```text
running
```

The VM agent will use this backend URL:

```text
http://127.0.0.1:18080
```

That port exists inside the VM because the local machine created a reverse SSH
tunnel.

The tunnel is only for agent telemetry. VM network traffic is captured from the
VM interface, normally `ens3`, through Traffic Control.

### 3. Install the VM agent from release

Run on each cloud VM:

```bash
curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/install-agent.sh

chmod +x /tmp/vmlens-install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh
```

Check:

```bash
systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 50 --no-pager
```

Expected:

```text
active
registered agent=...
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
```

When the install command is run over SSH, the installer automatically adds the
SSH tunnel peer IP to the deny filter so backend tunnel traffic is not counted
as external VM traffic.

### 4. Verify local dashboard sees the VMs

Run on local:

```bash
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/stats/summary
```

Expected:

```text
agents: online
VMs: online
```

## Test VM-to-VM traffic

Server VM:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

Client VM:

```bash
python3 -c "import time, urllib.request
for i in range(1, 21):
    r = urllib.request.urlopen('http://10.20.20.199:8081/', timeout=5)
    print(f'request={i:02d} status={r.status}')
    r.read(128)
    time.sleep(0.2)
"
```

Local verification:

```bash
curl http://127.0.0.1:8080/api/stats/summary
curl 'http://127.0.0.1:8080/api/internal/activity?limit=10'
curl http://127.0.0.1:8080/api/graph
```

Expected activity:

```text
testing-a-2 (10.20.20.130) -> testing-a-1 (10.20.20.199):8081 tcp
```

## Common operations

### Stop local dashboard

```bash
docker compose down
```

Stop and delete local database:

```bash
docker compose down -v
```

### Stop VM agent

Run on the VM:

```bash
sudo systemctl stop vmlens-agent
```

### Stop tunnel

Run on local:

```bash
bash scripts/vmlens-tunnel.sh stop <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
```

### Reset SSH known host

Use this when OpenStack reuses an IP and SSH says the host key changed:

```bash
bash scripts/vmlens-tunnel.sh forget-host <VM_IP>
```

## Supported communication tests

VMLens observes TCP/UDP transport metadata, so it can show relationships for
many development-style communications:

| Communication | Example |
| --- | --- |
| HTTP service | `curl`, Python `http.server`, frontend/backend traffic |
| TCP service | `nc`, Redis, PostgreSQL, RabbitMQ, app-to-app calls |
| UDP service | DNS, UDP test traffic |
| SSH/SCP | admin or file transfer sessions |
| External traffic | package download, git clone, API calls |
| Kubernetes to service VM | pod/node traffic to Redis/Postgres/RabbitMQ VM |

For exact application-level request latency, status code, path, tenant, and
trace context, combine this with application logs, OpenTelemetry, Prometheus or
a proxy.

## Status model

```text
online  = recent register/heartbeat/activity
stale   = no recent heartbeat/activity for a short period
offline = no heartbeat/activity past the offline threshold
```

Offline nodes are retained as inventory/history instead of being removed
immediately.

## Repository layout

```text
agent/        VM agent source and eBPF collector
  ebpf/       eBPF programs and fallback headers
  internal/   capture, identity, telemetry, transport and lifecycle packages
backend/      Go control-plane API and migrations
frontend/     React dashboard
scripts/      tunnel, install, release and agent helpers
configs/      local tunnel/VM profile examples
configuration/OpenStack cloud-init examples
instructions/ communication test commands
docs/         architecture, privacy and prebuilt agent notes
legacy/       older v1 prototype stack kept for reference
SONI.txt      tested E2E command flow from zero
```

## API quick reference

```text
GET  /health
GET  /api/agents
GET  /api/vms
GET  /api/graph
GET  /api/stats/summary
GET  /api/internal/activity
GET  /api/realtime
POST /api/agents/register
POST /api/agents/heartbeat
POST /api/flows/ingest
```

## Requirements

Local:

- Docker Desktop or Docker Engine with Compose;
- ports `3000`, `5432`, and `8080` available;
- SSH access to the VMs.

VM:

- Linux amd64;
- root/sudo access for systemd service and eBPF load;
- kernel BTF available:

```bash
test -r /sys/kernel/btf/vmlinux
```

## Security note

This project currently targets development and controlled lab usage.

Do not expose the local backend publicly without adding TLS, authentication and
ingest authorization.

## More docs

- [SONI.txt](./SONI.txt) — tested E2E flow from zero.
- [SETUP.md](./SETUP.md) — setup notes.
- [CONFIGURATION.md](./CONFIGURATION.md) — OpenStack/user-data configuration.
- [INSTRUCTION.md](./INSTRUCTION.md) — communication test catalog.
- [docs/prebuilt-agent.md](./docs/prebuilt-agent.md) — release artifact flow.
- [docs/privacy.md](./docs/privacy.md) — privacy boundary.
