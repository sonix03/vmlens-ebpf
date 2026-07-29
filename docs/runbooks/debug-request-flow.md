# Debug Request Flow End-to-End

Runbook ini menjelaskan alur lengkap saat user mengirim request dari satu VM ke
VM lain dan bagaimana VMLens melacaknya.

Contoh lab:

| VM | IP | Peran |
| --- | --- | --- |
| testing-a-1 | `10.20.20.130` | client/server |
| testing-a-2 | `10.20.20.199` | client/server |
| testing-a-3 | `10.20.20.220` | client/server |

Endpoint lokal:

| Service | URL | Kode |
| --- | --- | --- |
| Dashboard | `http://localhost:3000` | `frontend/` |
| Backend API | `http://localhost:8080` | `backend/cmd/api/main.go` |
| Postgres | `localhost:5432` | `docker-compose.yml` |
| Agent backend tunnel inside VM | `http://127.0.0.1:18080` | `scripts/vmlens-tunnel.sh` |
| Probe listener inside VM | `0.0.0.0:18081` | `agent/internal/probe/probe.go` |
| Test HTTP server | `0.0.0.0:8081` | manual `python3 -m http.server` |

## 1. Local stack starts

Command:

```bash
docker compose up -d --build
```

What happens:

```text
docker-compose.yml
    ↓
datastore    = PostgreSQL
control-plane = backend API :8080
dashboard     = frontend :3000
```

Code path:

| Step | File |
| --- | --- |
| backend process starts | `backend/cmd/api/main.go` |
| config loads env | `backend/internal/config/config.go` |
| DB migrations run | `backend/internal/db/db.go` + `backend/internal/db/migrations/*.sql` |
| HTTP routes mount | `backend/internal/api/routes.go` |
| handlers serve API | `backend/internal/api/handlers.go` |

Debug:

```bash
docker compose ps
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/api/agents
curl -sS http://127.0.0.1:8080/api/vms
curl -I http://localhost:3000
```

Expected:

```text
backend health ok
dashboard HTTP 200
agents/vms can be empty before agent install
```

If this fails:

| Symptom | Check |
| --- | --- |
| `localhost:3000` not open | `docker compose logs dashboard --tail=80` |
| backend 502 from dashboard | `docker compose logs control-plane --tail=120` |
| DB unavailable | `docker compose logs datastore --tail=120` |

## 2. Local opens reverse SSH tunnel to each VM

Command:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130 ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh start 10.20.20.199 ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh start 10.20.20.220 ~/.ssh/id_ed25519_vmlens
```

What happens:

```text
VM 127.0.0.1:18080
    ↓ reverse SSH tunnel
local 127.0.0.1:8080
```

The agent inside the VM talks to `http://127.0.0.1:18080`, but the request
actually lands on local backend `http://127.0.0.1:8080`.

Code path:

| Step | File |
| --- | --- |
| tunnel command | `scripts/vmlens-tunnel.sh` |
| local tunnel state | `~/.vmlens/tunnels/` |
| backend target | `docker-compose.yml` port `8080:8080` |

Debug from local:

```bash
bash scripts/vmlens-tunnel.sh status 10.20.20.130 ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh status 10.20.20.199 ~/.ssh/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh status 10.20.20.220 ~/.ssh/id_ed25519_vmlens
```

Debug from inside each VM:

```bash
curl -sS http://127.0.0.1:18080/health
```

Expected:

```text
{"status":"ok","database":"ok",...}
```

If this fails:

| Symptom | Meaning |
| --- | --- |
| `curl 127.0.0.1:18080` refused inside VM | reverse tunnel is not active |
| SSH host identification changed | recreate VM host key entry with `ssh-keygen -R <ip>` |
| `.130` direct timeout | use proxy/jump through reachable VM if network requires it |

## 3. Agent installs on VM

Command from VM:

```bash
sudo env \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  FLOW_INTERVAL=1s \
  FLOW_DEBUG=true \
  CONNECTIVITY_PROBE_ENABLED=true \
  CONNECTIVITY_PROBE_INTERVAL=5s \
  CONNECTIVITY_PROBE_LISTEN_ADDR=0.0.0.0:18081 \
  bash scripts/install-agent.sh
```

What happens:

```text
install-agent.sh
    ↓
build/copy /usr/local/bin/vmlens-agent
build/copy /usr/lib/vmlens/flow_tracker.bpf.o
write /etc/vmlens/agent.env
write systemd service
start vmlens-agent
```

Code path:

| Step | File |
| --- | --- |
| installer | `scripts/install-agent.sh` |
| systemd unit source | `configs/systemd/vmlens.service` |
| agent entrypoint | `agent/cmd/agent/main.go` |
| config env | `agent/internal/config/config.go` |
| identity discovery | `agent/internal/identity/identity.go` |
| heartbeat loop | `agent/internal/lifecycle/heartbeat.go` |
| backend HTTP client | `agent/internal/exporter/backend.go` |
| flow aggregate/debug | `agent/internal/flow/store.go` + `agent/internal/flow/debug.go` |
| probe listener/client | `agent/internal/probe/probe.go` |
| eBPF collector | `agent/internal/features/traffic/packet/ebpf_collector.go` |
| eBPF C program | `agent/internal/features/traffic/packet/flow_tracker.bpf.c` |

Debug from VM:

```bash
systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 120 --no-pager
sudo journalctl -u vmlens-agent -f | grep flow_debug
cat /etc/vmlens/agent.env
ss -ltnp 'sport = :18081'
```

Expected logs:

```text
registered agent=... vm=...
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
```

Toggle `FLOW_DEBUG` after install:

```bash
bash scripts/vmlens-agent.sh debug-status
bash scripts/vmlens-agent.sh debug-on
sudo journalctl -u vmlens-agent -f | grep flow_debug
bash scripts/vmlens-agent.sh debug-off
```

If this fails:

| Symptom | Check |
| --- | --- |
| agent cannot register | `curl http://127.0.0.1:18080/health` from VM |
| eBPF object missing | `/usr/lib/vmlens/flow_tracker.bpf.o` |
| TC attach failed | `CAPTURE_INTERFACE`, kernel support, `sudo tc filter show dev ens3 ingress` |
| no probe listener | `CONNECTIVITY_PROBE_LISTEN_ADDR`, journal logs |

## 4. Agent registers and heartbeats

Agent sends:

```text
POST /api/agents/register
POST /api/agents/heartbeat
```

Agent code:

| Payload | File |
| --- | --- |
| registration shape | `agent/internal/exporter/registration_payload.go` |
| heartbeat shape | `agent/internal/exporter/flow_payload.go` |
| sender methods | `agent/internal/exporter/backend.go` |
| heartbeat loop | `agent/internal/lifecycle/heartbeat.go` |

Backend code:

| Endpoint | File |
| --- | --- |
| route | `backend/internal/api/routes.go` |
| handler | `backend/internal/api/handlers.go` |
| service | `backend/internal/service/agent_service.go` |
| tables | `backend/internal/db/migrations/001_init.sql` |

DB tables:

```text
agents
vms
vm_interfaces
```

Debug from local:

```bash
curl -sS http://127.0.0.1:8080/api/agents | jq
curl -sS http://127.0.0.1:8080/api/vms | jq
```

Expected:

```text
agent status = online
vm status = online
private_ip = VM IP
```

## 5. eBPF attaches to VM network path

Current eBPF source:

| Concern | Current file |
| --- | --- |
| eBPF entry sections | `agent/internal/features/traffic/packet/flow_tracker.bpf.c` |
| TC packet parsing | `agent/internal/features/traffic/packet/parser.bpf.h` |
| socket/kprobe capture | `agent/internal/features/protocols/transport/tcp/connection/socket_capture.bpf.h` |
| network parser | `agent/internal/features/traffic/packet/network_parser.bpf.h` |
| transport parser | `agent/internal/features/traffic/packet/transport_parser.bpf.h` |
| flow event struct | `agent/internal/shared/bpf/flow_event.h` |
| byte/packet helpers | `agent/internal/features/traffic/bytes/` + `agent/internal/features/traffic/packets/` |
| port/classification helpers | `agent/internal/features/classification/ports.bpf.h` |

Current userspace collector:

| Concern | Current file |
| --- | --- |
| load object | `agent/internal/features/traffic/packet/ebpf_collector.go` |
| attach TCX ingress/egress | `agent/internal/features/traffic/packet/ebpf_collector.go` |
| attach kprobe fallback | `agent/internal/features/traffic/packet/ebpf_collector.go` |
| read ring buffer | `agent/internal/features/traffic/packet/ebpf_collector.go` |
| convert raw event to telemetry | `agent/internal/features/traffic/packet/ebpf_collector.go` |

Debug from VM:

```bash
sudo bpftool prog show | grep -Ei 'vmlens|tc|trace' || true
sudo bpftool link show | grep -Ei 'vmlens|tcx|tc' || true
sudo tc filter show dev ens3 ingress || true
sudo tc filter show dev ens3 egress || true
sudo journalctl -u vmlens-agent -n 120 --no-pager
```

Expected:

```text
TCX ingress/egress attached, or kprobe fallback attached in auto mode
```

## 6. Connectivity probe keeps idle line alive

Agent does two probe jobs:

```text
listen on 0.0.0.0:18081
probe known peer targets from /api/connections/targets
```

Code path:

| Step | File |
| --- | --- |
| probe logic | `agent/internal/probe/probe.go` |
| target fetch | `agent/internal/exporter/backend.go` |
| backend target query | `backend/internal/service/connection_service.go` |
| probe ingest | `backend/internal/service/connection_service.go` |
| DB table | `backend/internal/db/migrations/007_connection_probes.sql` |
| graph edge merge | `backend/internal/service/graph_service.go` |

Backend endpoints:

```text
GET  /api/connections/targets?agent_id=<agent>
POST /api/connections/probe
```

Debug local:

```bash
curl -sS 'http://127.0.0.1:8080/api/graph?time_range=5m' \
  | jq '[.edges[] | select(.kind == "reachability") | {source,target,dst_port,reachable,avg_rtt_ms,last_observed_at}]'
```

Expected:

```text
green idle line
kind = reachability
dst_port = 18081
request_count = 0
```

Important:

```text
Probe is not user traffic.
Probe must not increment request_count or bytes of real request traffic.
```

## 7. Start test HTTP server on target VM

Run on target VM, example `testing-a-2`:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
ss -ltnp 'sport = :8081'
```

Expected:

```text
LISTEN 0.0.0.0:8081 python3
```

If this is not listening, request test will become red/failed.

## 8. Send request from source VM

Run on source VM, example `testing-a-1 -> testing-a-2`:

```bash
for i in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 http://10.20.20.199:8081/ || true)
  echo "$HOSTNAME -> 10.20.20.199:8081 request=$i status=$code"
  sleep 0.2
done
```

Expected:

```text
status=200
green moving line in dashboard
Connection Flow / Internal Activity row appears
L4 Flow row appears
```

## 9. What happens after request is sent

Full data path:

```text
curl on source VM
    ↓
kernel network stack
    ↓
TC/eBPF program sees packet on ens3
    ↓
ring buffer event
    ↓
agent/internal/features/traffic/packet/ebpf_collector.go
    ↓
features/classification + features/traffic/bytes + features/traffic/direction
    ↓
agent/internal/flow/store.go
    ↓
agent/internal/flow/debug.go when FLOW_DEBUG=true
    ↓
agent/internal/exporter/backend.go
    ↓
POST http://127.0.0.1:18080/api/flows/ingest
    ↓
reverse SSH tunnel
    ↓
local backend http://127.0.0.1:8080/api/flows/ingest
    ↓
backend/internal/api/handlers.go
    ↓
backend/internal/service/flow_service.go
    ↓
network_flows + flow_observations
    ↓
/api/internal/activity
/api/flows
/api/graph
    ↓
frontend/src/api/client.ts
    ↓
frontend/src/App.tsx
    ↓
frontend/src/components/GraphView.tsx
frontend/src/components/InternalActivityTable.tsx
frontend/src/components/FlowTelemetryTable.tsx
```

## 10. Backend verification after request

Run local:

```bash
curl -sS 'http://127.0.0.1:8080/api/flows?limit=20' \
  | jq 'map({observed_at,src_ip,dst_ip,protocol,src_port,dst_port,connection_count,request_count,error_count,bytes_sent,bytes_received})'
```

```bash
curl -sS 'http://127.0.0.1:8080/api/internal/activity?limit=20&time_range=5m' \
  | jq 'map({observed_at,source_name,source_ip,destination_name,destination_ip,protocol,service_port,connection_count,request_count,error_count})'
```

```bash
curl -sS 'http://127.0.0.1:8080/api/graph?time_range=5m' \
  | jq '{
    nodes: (.nodes|length),
    edges: (.edges|length),
    sample: [.edges[] | {source,target,kind,protocol,dst_port,active,failed,reachable,request_count,error_count,avg_rtt_ms,last_observed_at}][0:20]
  }'
```

Expected:

```text
api/flows contains tcp dst_port=8081
api/internal/activity contains VM-to-VM row
api/graph contains active edge
```

## 11. Frontend verification

Open:

```text
http://localhost:3000
```

UI files:

| UI area | File |
| --- | --- |
| page state and polling | `frontend/src/App.tsx` |
| graph drawing | `frontend/src/components/GraphView.tsx` |
| selected VM detail | `frontend/src/components/NodeDetailsPanel.tsx` |
| selected edge detail | `frontend/src/components/EdgeDetailsPanel.tsx` |
| internal log table | `frontend/src/components/InternalActivityTable.tsx` |
| L4/request tables | `frontend/src/components/FlowTelemetryTable.tsx` |
| API client | `frontend/src/api/client.ts` |
| frontend DTOs | `frontend/src/types/*.ts` |

Visual expectation:

| Signal | Expected UI |
| --- | --- |
| successful request | green line moves briefly |
| idle connectivity probe succeeds | same line stays green but stops moving |
| RTT above threshold | yellow/degraded row or edge |
| closed/refused port | red failed row/edge |

## 12. Green, yellow, red quick tests

Full test catalog lives in:

```text
testing/vm-network-traffic.md
```

Minimal green:

```bash
curl -s -o /dev/null -w "status=%{http_code} total=%{time_total}\n" http://10.20.20.199:8081/
```

Minimal yellow from source VM:

```bash
set -e
trap "sudo tc qdisc del dev ens3 root 2>/dev/null || true" EXIT
sudo tc qdisc replace dev ens3 root netem delay 180ms
curl -s -o /dev/null -w "status=%{http_code} total=%{time_total}\n" http://10.20.20.199:8081/
```

Remove delay:

```bash
sudo tc qdisc del dev ens3 root 2>/dev/null || true
```

Minimal red closed port:

```bash
curl -s -o /dev/null -w "status=%{http_code} total=%{time_total}\n" --connect-timeout 1 --max-time 2 http://10.20.20.199:65534/ || true
```

Minimal red service down:

```bash
sudo fuser -k 8081/tcp 2>/dev/null || true
curl -s -o /dev/null -w "status=%{http_code} total=%{time_total}\n" --connect-timeout 1 --max-time 2 http://10.20.20.199:8081/ || true
```

Restore server:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
```

## 13. Failure map

| Where it breaks | Symptom | Check |
| --- | --- | --- |
| Docker/backend | `/health` fails | `docker compose logs control-plane --tail=120` |
| tunnel | VM cannot curl `127.0.0.1:18080` | `scripts/vmlens-tunnel.sh status <vm>` |
| agent register | `/api/agents` empty | VM `journalctl -u vmlens-agent` |
| eBPF attach | agent online but no traffic | `bpftool`, `tc filter`, `CAPTURE_INTERFACE` |
| probe | no idle green line | VM `ss -ltnp 'sport = :18081'`, `/api/connections/targets` |
| server port | curl returns `000` | target VM `ss -ltnp 'sport = :8081'` |
| backend ingest | VM sees request but API empty | `/api/flows/ingest` logs, `flow_service.go` |
| graph | flows exist but no edge | graph filters/excluded ports/IPs in `docker-compose.yml` |
| frontend | API has data but UI stale | browser network tab, `frontend/src/api/client.ts` |

## 14. Target code location after refactor

After the agent refactor, the same request should be traced like this:

```text
curl
  ↓
internal/features/traffic/packet
  ↓
internal/features/traffic/bytes
internal/features/traffic/packets
internal/features/classification
  ↓
internal/flow/store.go
  ↓
internal/flow/debug.go if FLOW_DEBUG=true
  ↓
internal/exporter/flow_payload.go
  ↓
internal/exporter/backend.go
  ↓
backend /api/flows/ingest
  ↓
frontend graph/tables
```

The old current locations are documented in:

```text
docs/agent-refactor-structure.md
```
