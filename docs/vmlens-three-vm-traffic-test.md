# VMLens 3 VM traffic test

Dokumen ini dipakai untuk menguji map dan log data dari TC/eBPF tracker VMLens.

| VM | IP | Role test |
| --- | --- | --- |
| testing-a-1 | `10.20.20.130` | client + server |
| testing-a-2 | `10.20.20.199` | client + server |
| testing-a-3 | `10.20.20.220` | client + server |

Expected visual:

| Case | Trigger | Expected map | Expected log |
| --- | --- | --- | --- |
| Green | HTTP `200` to port `8081` | one green edge; moving while request is active, then idle | success flow row |
| Yellow | temporary RTT delay via `tc netem` | yellow/degraded if RTT threshold is exceeded | `SLOW RTT` row |
| Red | request to closed port `65534` | red failed edge/flash | failed/refused/timeout row |

Important distinction:

- Green idle line = VM path is reachable or recent successful connection evidence exists.
- Moving line = request/traffic is active.
- Yellow = path works but RTT is slow.
- Red = service/port attempt failed.
- No line = no recent observed evidence in the current TTL/window.

## 1. Local prerequisites

Run from repo root:

```bash
cd /mnt/c/documents/ionext/vmlens-ebpf
bash scripts/vmlens-stack.sh start
```

Open:

```text
VMLens UI: http://localhost:3000
Backend:   http://localhost:8080/health
```

## 2. Start tunnels

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
bash scripts/vmlens-tunnel.sh start 10.20.20.130 "$KEY"
bash scripts/vmlens-tunnel.sh start 10.20.20.199 "$KEY"
bash scripts/vmlens-tunnel.sh start 10.20.20.220 "$KEY"
```

Check:

```bash
bash scripts/vmlens-tunnel.sh status 10.20.20.130 "$KEY"
bash scripts/vmlens-tunnel.sh status 10.20.20.199 "$KEY"
bash scripts/vmlens-tunnel.sh status 10.20.20.220 "$KEY"
```

## 3. Check VM agent and test server

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
for ip in 10.20.20.130 10.20.20.199 10.20.20.220; do
  ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@$ip \
    'hostname; systemctl is-active vmlens-agent; ss -ltnp "sport = :8081" || true'
done
```

Start server if `8081` is not listening:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
```

Restart agents after tunnel is active:

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
for ip in 10.20.20.130 10.20.20.199 10.20.20.220; do
  ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@$ip \
    'sudo systemctl restart vmlens-agent && systemctl is-active vmlens-agent'
done
```

## 4. Green test: success request on port 8081

From each source VM:

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.130 \
  'for dst in 10.20.20.199 10.20.20.220; do for i in 1 2 3 4 5; do code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8081/" || true); echo "$HOSTNAME -> $dst:8081 green_$i status=$code"; sleep 0.2; done; done'

ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.199 \
  'for dst in 10.20.20.130 10.20.20.220; do for i in 1 2 3 4 5; do code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8081/" || true); echo "$HOSTNAME -> $dst:8081 green_$i status=$code"; sleep 0.2; done; done'

ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.220 \
  'for dst in 10.20.20.130 10.20.20.199; do for i in 1 2 3 4 5; do code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8081/" || true); echo "$HOSTNAME -> $dst:8081 green_$i status=$code"; sleep 0.2; done; done'
```

## 5. Red test: closed port 65534

Expected curl status is `000`.

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.199 \
  'for dst in 10.20.20.130 10.20.20.220; do for i in 1 2 3; do code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://$dst:65534/" || true); echo "$HOSTNAME -> $dst:65534 red_$i status=$code"; sleep 0.2; done; done'
```

## 6. Yellow test: real network RTT delay

This temporarily adds delay on `.199` `ens3`, sends traffic, then cleans up.

```bash
KEY="$HOME/.ssh/id_ed25519_vmlens"
ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.199 \
  'set -e; trap "sudo tc qdisc del dev ens3 root 2>/dev/null || true" EXIT; sudo tc qdisc replace dev ens3 root netem delay 180ms; for dst in 10.20.20.130 10.20.20.220; do for i in 1 2 3 4 5; do t=$(curl -s -o /dev/null -w "%{time_total}" --connect-timeout 3 --max-time 6 "http://$dst:8081/" || true); echo "$HOSTNAME -> $dst:8081 yellow_$i total=${t}s"; sleep 0.2; done; done'
```

Verify cleanup:

```bash
ssh -i "$KEY" -o IdentitiesOnly=yes ubuntu@10.20.20.199 'tc qdisc show dev ens3'
```

## 7. Verify VMLens data

```bash
curl -sS 'http://127.0.0.1:8080/api/vms' | jq 'map({name, private_ip, status, last_seen})'
curl -sS 'http://127.0.0.1:8080/api/graph?time_range=15m' | jq '{nodes:(.nodes|length), edges:(.edges|length), samples:(.edges[:10] | map({source,target,protocol,dst_port,request_count,error_count,avg_rtt_ms,last_observed_at}))}'
curl -sS 'http://127.0.0.1:8080/api/flows?limit=20' | jq 'map({observed_at,src_ip,dst_ip,protocol,dst_port,connection_count,request_count,error_count,bytes_sent,bytes_received})'
curl -sS 'http://127.0.0.1:8080/api/internal/activity?limit=10&time_range=15m' | jq 'map({observed_at,source_name,destination_name,service_port,request_count,error_count})'
```

UI checks:

| UI area | What to check |
| --- | --- |
| VM Topology | 3 VM nodes should appear. Idle green line means connection. Moving line means request. |
| Internal Activity | Fast table from VM-to-VM observations. |
| Connection Flow | Clean L4 connectivity rows from TC/eBPF telemetry. |
| Request Flow | Request/attempt rows from TC/eBPF counters. |
| L4 Flow | Raw network flow aggregates from `/api/flows`. |
