# VMLens

VMLens is an eBPF-powered VM network relationship tracker. Agents register the
VMs on which they run, send heartbeats and metadata-only network flows to a Go
API, and the API stores an aggregated topology in PostgreSQL. The dashboard
renders the topology and refreshes through Server-Sent Events (SSE).

## Privacy boundary

VMLens collects only relationship metadata:

- agent and machine identity;
- interface names, IP addresses and MAC addresses;
- TCP/UDP source and destination ports;
- sent/received byte counters, packets and connection counts;
- direction, first/last seen timestamps and interface name.

VMLens does not capture packet payloads, HTTP bodies, SSH content, database
queries, file contents, request/response bodies, TLS plaintext or command lines.

## Architecture

```text
[customer VM / cloud VM / local VM]
  vmlens-agent
    |-- POST /api/agents/register
    |-- POST /api/agents/heartbeat
    `-- POST /api/flows/ingest
                  |
                  v
      [control-plane API :8080]
           |-- PostgreSQL
           |-- REST graph/stats API
           `-- SSE /api/realtime
                  |
                  v
          [dashboard UI :3000]
```

Registration is automatic. There is no VM-registration screen, required seed
record, or demo VM in the default stack.

## Repository

```text
backend/    control-plane source: Go REST API, services, migrations and SSE hub
agent/      VM agent source: identity detection, mock/eBPF collectors and sender
frontend/   dashboard source: React, TypeScript and topology graph
configs/    local operator config examples for SSH keys, VM list and paths
scripts/    local tunnel and VM agent management scripts
docker-compose.yml
```

See [`docs/project-structure.md`](docs/project-structure.md) for the current
runtime service names and folder conventions.

## Quick start

Requirements:

- Docker Desktop or Docker Engine with Compose;
- ports `3000`, `5432` and `8080` available.

Start the datastore, control-plane API and dashboard:

```bash
cp .env.example .env
docker compose up -d --build
```

Buka:

- dashboard: http://localhost:3000
- API health: http://localhost:8080/health
- live graph JSON: http://localhost:8080/api/graph

The graph starts empty by design. Install the script on a real VM; that VM
registers itself and appears as a node without a page refresh. Relationships
appear when the real collector sends flows.

The dashboard applies `flow.updated` SSE events directly for immediate active
line feedback, then performs a throttled canonical refresh for counters. Traffic
cards split cumulative internal VM bytes from external public bytes, including
sent/received and aggregated-flow counts.

Useful commands:

```bash
docker compose ps
docker compose logs -f control-plane dashboard datastore
docker compose down
docker compose down -v   # also deletes local PostgreSQL test data
```

## Simple cloud VM setup

Use this when the dashboard runs locally, while agents run on cloud VMs that are
reachable over SSH.

Tracking happens inside each cloud VM. The local machine only runs PostgreSQL,
the control-plane API, and the dashboard. Do not install the agent locally
unless you also want to track the local machine as a VM node.

For a copy-paste guide starting from a fresh clone, see
[`instructions/15-from-scratch-local-dashboard-cloud-agent.md`](instructions/15-from-scratch-local-dashboard-cloud-agent.md).

### 1. Start the local dashboard

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/health
```

Open:

```text
http://localhost:3000
```

### 2. Start one tunnel for each cloud VM

On your local machine:

```bash
bash scripts/vmlens-tunnel.sh start <VM_A_PUBLIC_IP>
bash scripts/vmlens-tunnel.sh start <VM_B_PUBLIC_IP>
```

Use an explicit key when needed:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh stop <VM_IP> ~/.vmlens/keys/id_ed25519_vmlens
```

If a VM IP was reused and SSH reports a changed host key:

```bash
bash scripts/vmlens-tunnel.sh forget-host <VM_IP>
```

Or configure the VM list and SSH keys once:

```bash
cp configs/local.env.example configs/local.env
bash scripts/vmlens-tunnel.sh list
bash scripts/vmlens-tunnel.sh start-all
```

`configs/local.env` contains the VM profiles, IP/host, SSH user, and SSH key
path for each VM. `configs/vms.local` is only a legacy fallback.

After this, each cloud VM agent can send observations to the local backend
through:

```text
http://127.0.0.1:18080
```

The tunnel is only a transport path from the cloud VM to the local backend. It
does not track the local machine.

Normal SSH access stays unchanged:

```bash
ssh ubuntu@<VM_PUBLIC_IP>
```

### 3. Install and start the agent on each VM

On each VM:

```bash
sudo apt-get update
sudo apt-get install -y git golang-go clang bpftool libbpf-dev

git clone <repo-url>
cd vmlens-ebpf

BACKEND_URL=http://127.0.0.1:18080 \
AGENT_PUBLIC_IP=<THIS_VM_PUBLIC_IP> \
bash scripts/vmlens-agent.sh start
```

Check:

```bash
bash scripts/vmlens-agent.sh status
bash scripts/vmlens-agent.sh logs
```

### 4. Test communication between cloud VMs

On VM B:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM A:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null http://<VM_B_PUBLIC_IP>:8081/; sleep 0.2; done
```

Check the dashboard:

```text
http://localhost:3000
```

The VM line should animate, byte counters should increase, and
`Request frequency` should show recent request activity.

### 5. Stop

On each VM:

```bash
bash scripts/vmlens-agent.sh stop
```

On your local machine:

```bash
bash scripts/vmlens-tunnel.sh stop <VM_A_PUBLIC_IP>
bash scripts/vmlens-tunnel.sh stop <VM_B_PUBLIC_IP>
docker compose down
```

Notes:

- Open firewall/security group access for test port `8081` between VMs.
- If VMs are in different clouds, private IPs usually cannot communicate
  directly. Use public IPs, VPN, WireGuard, Tailscale, or another routed
  network.
- If the test uses public IPs, set `AGENT_PUBLIC_IP` so VMLens can resolve the
  destination public IP back to the VM node.
- Do not expose this backend to the public internet without TLS/authentication.

## Runtime behavior

### Auto-registration

On startup an agent detects hostname, `/etc/machine-id`, OS, kernel and network
interfaces and calls `POST /api/agents/register`. Backend VM identity priority:

1. `machine_id`;
2. existing `agent_id` mapping;
3. primary MAC address;
4. hostname plus primary private IP.

This prevents a normal agent restart from creating a duplicate VM node.

If an unknown internal node already exists for one of the new VM's IPs, the
backend marks it resolved and rewrites matching relationships to the registered
VM.

### Agent and VM status

- agent online: last registration or heartbeat less than 60 seconds ago;
- agent stale: last heartbeat between 1 and 5 minutes ago;
- agent offline: no heartbeat for more than 5 minutes;
- VM online: last registration, heartbeat, or observed registered VM traffic
  less than 60 seconds ago;
- VM stale: last observed VM activity between 1 and 5 minutes ago;
- VM offline: no observed VM activity for more than 5 minutes;
- retained as an offline inventory node when heartbeats stop.

The backend evaluates state periodically and emits an SSE update when state
changes. `VM_DELETE_AFTER=0` is the default, so offline nodes are retained.
Heartbeat timeout is best-effort: without a cloud-provider deletion webhook the
backend cannot distinguish a deleted VM from a long power/network outage. A
still-running agent automatically registers again after connectivity returns.

The graph doubles as VM inventory: online nodes have a bright green border and
shadow, while stale/offline nodes remain visible but dimmed. A node is removed
only when its VM record is explicitly deleted (for example by a future
OpenStack lifecycle integration).

### Flow aggregation

Flows are aggregated by source VM/IP, destination VM/IP, protocol, destination
port and scope. A transaction-scoped advisory lock prevents concurrent requests
from creating duplicate graph edges. Existing counters are incremented and
`first_seen`/`last_seen` retain the full observed window.

Scopes:

- `internal_same_tenant`;
- `internal_cross_tenant`;
- `unknown_internal`;
- `external_public`;
- `unknown`.

Internal CIDRs are configured with `INTERNAL_CIDRS`.

### Live communication lines

An edge is bright and animated while the backend continues receiving TCP/UDP
activity for that VM relationship. After no new activity is observed for
`FLOW_ACTIVE_WINDOW` (default `3s`), its animation stops and the historical
relationship remains as a dim static line. The browser evaluates the expiry
locally, so a line can stop without waiting for the 10-second polling fallback.

This is transport-level activity, not an HTTP request parser. HTTPS, HTTP/2,
connection pooling and non-HTTP protocols can reuse one connection, so exact
application request boundaries require application tracing such as
OpenTelemetry. See `INSTRUCTION.md` for supported communication and tests.

## REST API

```text
GET  /health
POST /api/agents/register
POST /api/agents/heartbeat
GET  /api/agents
GET  /api/vms
GET  /api/flows
POST /api/flows/ingest
GET  /api/graph
GET  /api/stats/summary
GET  /api/stats/top-talkers
GET  /api/realtime
```

Register a test agent:

```bash
curl -X POST http://localhost:8080/api/agents/register \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id":"agent-manual-test",
    "hostname":"vm-manual-test",
    "machine_id":"manual-machine-001",
    "tenant_id":"tenant-demo",
    "private_ips":["10.10.1.60"],
    "mac_addresses":["52:54:00:aa:01:60"],
    "interfaces":[{"name":"eth0","ip_address":"10.10.1.60","mac_address":"52:54:00:aa:01:60"}],
    "os":"ubuntu",
    "kernel":"6.8.0",
    "agent_version":"0.1.0",
    "environment":"manual-test"
  }'
```

Ingest a flow:

```bash
curl -X POST http://localhost:8080/api/flows/ingest \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id":"agent-manual-test",
    "src_ip":"10.10.1.60",
    "dst_ip":"8.8.8.8",
    "src_port":43120,
    "dst_port":443,
    "protocol":"tcp",
    "direction":"egress",
    "bytes_sent":500000,
    "bytes_received":900000,
    "packets":500,
    "connection_count":2,
    "first_seen":"2026-07-02T10:00:00Z",
    "last_seen":"2026-07-02T10:00:05Z",
    "interface":"eth0"
  }'
```

Graph filters:

```text
/api/graph?vm_id=vm-id
/api/graph?scope=external_public
/api/graph?scope=unknown_internal
/api/graph?protocol=tcp
/api/graph?port=5432
/api/graph?time_range=5m
/api/graph?min_bytes=10000000
/api/graph?status=online
/api/graph?tenant_id=tenant-demo
/api/graph?agent_id=agent-id
```

Edge weight uses total sent plus received bytes:

```text
<100 KiB       weight 1
100 KiB-1 MiB  weight 2
1-10 MiB       weight 3
10-100 MiB     weight 4
>=100 MiB      weight 5
```

## Install a real agent

Build and install from a checked-out repository on a Linux VM:

```bash
sudo apt-get update
sudo apt-get install -y golang-go clang bpftool libbpf-dev
sudo BACKEND_URL=http://BACKEND_IP:8080 MOCK_MODE=false ./scripts/install-agent.sh
sudo journalctl -u vmlens-agent -f
```

The installer builds the Go binary and compiles the eBPF object against the
target VM's own kernel BTF. On successful startup the VM registers immediately;
it appears as a node before the first network edge is observed.

For a VM behind NAT, register both the guest interface and reachable/NAT IP:

```bash
sudo env BACKEND_URL=http://BACKEND_IP:8080 \
  AGENT_PRIVATE_IPS=192.168.1.144,10.20.20.103 \
  bash scripts/install-agent.sh
```

## Test communication between two VMs

Both VM agents must be registered, and the destination address observed by the
source must match an IP registered by the destination VM. Use the same tenant
name on both agents when testing same-tenant topology.

On VM B, start a temporary bounded test server (allow TCP 8081 only between the
two test VMs in the firewall/security group):

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM A, generate three HTTP connections to VM B's private IP:

```bash
cd vmlens-ebpf
bash scripts/test-vm-communication.sh VM_B_PRIVATE_IP 8081
```

The backend resolves `dst_ip` to VM B, emits `flow.updated` over SSE, and the UI
draws an animated A → B line. Reverse the test from VM B to VM A to verify both
directions. Check resolution directly if the line does not appear:

```bash
curl http://BACKEND_IP:8080/api/vms
curl http://BACKEND_IP:8080/api/flows
curl http://BACKEND_IP:8080/api/graph?time_range=5m
```

Uninstall:

```bash
sudo ./scripts/uninstall-agent.sh
```

The backend port must be reachable from the VM. Use TLS and authentication
before exposing an ingest endpoint outside a trusted development network.

## Real eBPF mode

The installed agent runs real mode by default. It requires Linux kernel BTF, clang,
bpftool, libbpf headers and sufficient BPF/kprobe privileges. Build instructions
are in `agent/ebpf/README.md`.

The current eBPF program observes best-effort TCP/IPv4 connect, accept, send and
receive metadata plus UDP send/receive metadata. Byte counters are application
bytes returned by socket functions, not Ethernet wire bytes. Packet counters are
zero when the kernel source cannot provide a defensible value.

Kernel function names and signatures must be validated across the production
kernel support matrix.

## Development

Compile Go services:

```bash
make test
make build
```

Validate frontend in Docker (recommended when the repository is on an NTFS
mount under WSL):

```bash
docker build -t vmlens-frontend-check ./frontend
```

Database migrations are embedded into the backend and applied idempotently at
startup from `backend/internal/db/migrations`.

## MVP limitations

- ingest endpoints have no authentication or TLS yet;
- PostgreSQL flow retention/partitioning is not implemented;
- external ASN, country, provider and reverse-DNS enrichment fields are stored
  but not populated;
- the graph limits a response to the 5,000 most recent aggregate rows;
- real eBPF coverage is IPv4-oriented and requires kernel-level testing;
- SSE broadcasts invalidation events; the frontend refetches authoritative
  graph/state instead of applying fragile partial graph mutations.
