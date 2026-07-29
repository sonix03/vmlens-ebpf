# VMLens VM Network Test Cases

Dokumen ini berisi command manual untuk menguji warna dan log network VMLens.
Tidak ada script repo tambahan; copy command sesuai VM yang sedang dipakai.

## VM lab

| VM | IP | Fungsi |
| --- | --- | --- |
| testing-a-1 | `10.20.20.130` | client + server |
| testing-a-2 | `10.20.20.199` | client + server |
| testing-a-3 | `10.20.20.220` | client + server |

## Visual model

| Warna | Arti | Sumber evidence |
| --- | --- | --- |
| Hijau diam | VM pair reachable / connected idle | VMLens probe `18081` atau recent success |
| Hijau bergerak | request sedang lewat | real traffic, misalnya HTTP `8081` |
| Kuning | masih reachable tapi degraded | RTT melewati threshold |
| Merah | attempt gagal | refused, timeout, blocked, agent/probe failure |

Default threshold UI:

```text
VITE_SLOW_RTT_THRESHOLD_MS=100
```

## Trackability matrix

| ID | Warna | Case | Bisa ditrack sekarang? | Expected saat ini |
| --- | --- | --- | --- | --- |
| G1 | Hijau | HTTP `200` ke `8081` | Ya | edge hijau bergerak, row request/L4 |
| G2 | Hijau | Bidirectional HTTP `8081` | Ya | satu VM-pair line, animasi dua arah saat bersamaan |
| G3 | Hijau | VM reachability probe `18081` | Ya | edge hijau diam, tidak dihitung request |
| G4 | Hijau | ICMP ping | Partial | network flow bisa muncul; request flow tidak |
| Y1 | Kuning | RTT tinggi via `tc netem delay` | Ya | row kuning + RTT angka di atas threshold |
| Y2 | Kuning | jitter via `tc netem delay ... distribution` | Partial | bisa terlihat dari RTT naik; jitter explicit belum |
| Y3 | Kuning/Merah | packet loss via `tc netem loss` | Partial | timeout/error bisa muncul; packet loss metric explicit belum |
| Y4 | Kuning | app response lambat | Belum penuh | command bisa dites; app delay classification masih future |
| R1 | Merah | closed port `65534` | Ya | status `000`, failed row/edge |
| R2 | Merah | service `8081` dimatikan | Ya | request ke `8081` gagal |
| R3 | Merah | firewall REJECT TCP reset | Ya | failed/refused style |
| R4 | Merah | firewall DROP timeout | Partial | timeout terlihat; reason firewall_drop belum explicit |
| R5 | Merah/offline | block probe `18081` | Ya untuk connection state | line connection hilang/degraded; request port lain bisa tetap hidup |
| R6 | Merah | route blackhole | Partial | timeout/no route; route reason belum explicit |
| R7 | Merah/offline | stop `vmlens-agent` | Ya | VM stale/offline setelah timeout |
| R8 | Merah | DNS failed | Belum penuh | command gagal; DNS classification masih future |
| R9 | Merah | TLS handshake failed | Belum penuh | command gagal; TLS classification masih future |
| R10 | Merah | HTTP `500` | Belum penuh | request terlihat; response code classification future |
| R11 | Merah | HTTP app timeout | Belum penuh | command timeout; app_timeout classification future |
| R12 | Merah | HTTP `401/403` auth failed | Belum penuh | request terlihat; auth classification future |
| R13 | Merah | HTTP `429` rate limit | Belum penuh | request terlihat; rate_limit classification future |
| R14 | Merah | MTU / packet too large | Belum | command bisa dites; MTU classification future |

## Local app check

Run dari root repo:

```bash
docker compose ps
curl -sS http://localhost:8080/health
curl -I http://localhost:3000
```

Reset kosong total kalau perlu:

```bash
docker compose down -v
docker compose up -d --build datastore control-plane dashboard
```

## VM health check

Run di masing-masing VM:

```bash
hostname
hostname -I
systemctl is-active vmlens-agent
ss -ltnp 'sport = :8081' || true
ss -ltnp 'sport = :18081' || true
tc qdisc show dev ens3
```

Kalau `8081` belum listen:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
ss -ltnp 'sport = :8081'
```

Restart agent kalau backend baru di-reset:

```bash
sudo systemctl restart vmlens-agent
systemctl is-active vmlens-agent
ss -ltnp 'sport = :18081' || true
```

## Common cleanup

Run di VM yang sedang dipakai untuk eksperimen:

```bash
sudo tc qdisc del dev ens3 root 2>/dev/null || true
sudo iptables -D INPUT -p tcp --dport 8081 -j REJECT --reject-with tcp-reset 2>/dev/null || true
sudo iptables -D INPUT -p tcp --dport 8081 -j DROP 2>/dev/null || true
sudo iptables -D INPUT -p tcp --dport 18081 -j DROP 2>/dev/null || true
sudo ip route del blackhole 10.20.20.130/32 2>/dev/null || true
sudo ip route del blackhole 10.20.20.199/32 2>/dev/null || true
sudo ip route del blackhole 10.20.20.220/32 2>/dev/null || true
sudo systemctl start vmlens-agent
```

Kill temporary test servers:

```bash
sudo fuser -k 8081/tcp 2>/dev/null || true
sudo fuser -k 8082/tcp 2>/dev/null || true
sudo fuser -k 8083/tcp 2>/dev/null || true
sudo fuser -k 8084/tcp 2>/dev/null || true
sudo fuser -k 8085/tcp 2>/dev/null || true
```

Restore normal `8081` server:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
ss -ltnp 'sport = :8081'
```

## Peer variables

Set peer list sesuai VM tempat command dijalankan.

Di `testing-a-1`:

```bash
PEERS="10.20.20.199 10.20.20.220"
```

Di `testing-a-2`:

```bash
PEERS="10.20.20.130 10.20.20.220"
```

Di `testing-a-3`:

```bash
PEERS="10.20.20.130 10.20.20.199"
```

## G1. Green: HTTP success on 8081

Expected:

```text
status=200
green moving line
Request Flow + L4 Flow row
```

```bash
for dst in $PEERS; do
  for i in 1 2 3 4 5; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 green_$i status=$code"
    sleep 0.2
  done
done
```

## G2. Green: bidirectional request

Run this at the same time on two VMs.

On `testing-a-1`:

```bash
for i in $(seq 1 20); do
  curl -s -o /dev/null --connect-timeout 2 --max-time 4 http://10.20.20.199:8081/ || true
  echo "testing-a-1 -> testing-a-2 green_bidir_$i"
  sleep 0.2
done
```

On `testing-a-2`:

```bash
for i in $(seq 1 20); do
  curl -s -o /dev/null --connect-timeout 2 --max-time 4 http://10.20.20.130:8081/ || true
  echo "testing-a-2 -> testing-a-1 green_bidir_$i"
  sleep 0.2
done
```

Expected:

```text
one visual VM pair line
request animation can show both directions
```

## G3. Green idle: reachability probe only

Do not send app request. Just check probe listener:

```bash
ss -ltnp 'sport = :18081' || true
```

Then verify backend from local host:

```bash
curl -sS 'http://localhost:8080/api/graph?time_range=5m' \
  | jq '[.edges[] | select(.dst_port == 18081) | {source,target,dst_port,avg_rtt_ms,last_observed_at}]'
```

Expected:

```text
green idle connection
request_count=0
RTT available
```

## G4. ICMP ping

Run on any VM:

```bash
for dst in $PEERS; do
  ping -c 5 -i 0.2 "$dst"
done
```

Expected now:

```text
VM network activity may be visible as L4/network evidence.
It should not be counted as HTTP request.
```

## Y1. Yellow: high RTT with tc netem delay

Run on one source VM, for example `testing-a-2`.

```bash
set -e
trap "sudo tc qdisc del dev ens3 root 2>/dev/null || true" EXIT
sudo tc qdisc replace dev ens3 root netem delay 180ms

for dst in 10.20.20.130 10.20.20.220; do
  for i in 1 2 3 4 5; do
    t=$(curl -s -o /dev/null -w "%{time_total}" --connect-timeout 3 --max-time 6 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 yellow_$i total=${t}s"
    sleep 0.2
  done
done
```

Cleanup manual if needed:

```bash
sudo tc qdisc del dev ens3 root 2>/dev/null || true
tc qdisc show dev ens3
```

Expected:

```text
RTT number visible in log table
row/chip turns yellow if RTT >= threshold
```

## Y2. Yellow: jitter

Run on one source VM:

```bash
set -e
trap "sudo tc qdisc del dev ens3 root 2>/dev/null || true" EXIT
sudo tc qdisc replace dev ens3 root netem delay 80ms 80ms distribution normal

for dst in $PEERS; do
  for i in 1 2 3 4 5 6 7 8; do
    t=$(curl -s -o /dev/null -w "%{time_total}" --connect-timeout 3 --max-time 6 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 jitter_$i total=${t}s"
    sleep 0.2
  done
done
```

Expected now:

```text
RTT can become yellow.
Explicit jitter metric is future work.
```

## Y3. Yellow/Red: packet loss

Run on one source VM:

```bash
set -e
trap "sudo tc qdisc del dev ens3 root 2>/dev/null || true" EXIT
sudo tc qdisc replace dev ens3 root netem loss 30%

for dst in $PEERS; do


  for i in 1 2 3 4 5 6 7 8 9 10; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 loss_$i status=$code"
    sleep 0.2
  done
done
```

Expected now:

```text
some requests may still succeed
some may timeout as status=000
packet_loss metric itself is future work
```

## Y4. Future: app response slow

Run this on target VM to start a slow HTTP server on `8085`:

```bash
sudo fuser -k 8085/tcp 2>/dev/null || true
cat >/tmp/vmlens-slow-http.py <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer
import time

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        time.sleep(3)
        self.send_response(200)
        self.send_header("Content-Length", "2")
        self.end_headers()
        self.wfile.write(b"ok")
    def log_message(self, *args):
        pass

HTTPServer(("0.0.0.0", 8085), H).serve_forever()
PY
nohup python3 /tmp/vmlens-slow-http.py >/tmp/vmlens-slow-http.log 2>&1 &
ss -ltnp 'sport = :8085'
```

From source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    t=$(curl -s -o /dev/null -w "%{http_code} %{time_total}" --connect-timeout 2 --max-time 6 "http://$dst:8085/" || true)
    echo "$HOSTNAME -> $dst:8085 slow_app_$i result=$t"
  done
done
```

Expected now:

```text
L4 request can be observed.
App-delay classification is future unless L7/app probe is added.
```

Cleanup on target:

```bash
sudo fuser -k 8085/tcp 2>/dev/null || true
```

## R1. Red: closed port 65534

Expected curl status: `000`.

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://$dst:65534/" || true)
    echo "$HOSTNAME -> $dst:65534 red_closed_port_$i status=$code"
    sleep 0.2
  done
done
```

Expected:

```text
red row / failed edge
reason should become port_refused or connect_failed
```

## R2. Red: service 8081 stopped

Run on target VM:

```bash
sudo fuser -k 8081/tcp 2>/dev/null || true
ss -ltnp 'sport = :8081' || true
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 red_service_down_$i status=$code"
    sleep 0.2
  done
done
```

Restore on target:

```bash
nohup python3 -m http.server 8081 --bind 0.0.0.0 >/tmp/vmlens-http-8081.log 2>&1 &
ss -ltnp 'sport = :8081'
```

## R3. Red: firewall REJECT / TCP reset

Run on target VM:

```bash
sudo iptables -I INPUT -p tcp --dport 8081 -j REJECT --reject-with tcp-reset
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 red_reject_$i status=$code"
    sleep 0.2
  done
done
```

Cleanup on target:

```bash
sudo iptables -D INPUT -p tcp --dport 8081 -j REJECT --reject-with tcp-reset 2>/dev/null || true
```

Expected:

```text
failed attempt should be visible
future failure_reason should say tcp_reset or firewall_reject
```

## R4. Red: firewall DROP / timeout

Run on target VM:

```bash
sudo iptables -I INPUT -p tcp --dport 8081 -j DROP
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://$dst:8081/" || true)
    echo "$HOSTNAME -> $dst:8081 red_drop_timeout_$i status=$code"
    sleep 0.2
  done
done
```

Cleanup on target:

```bash
sudo iptables -D INPUT -p tcp --dport 8081 -j DROP 2>/dev/null || true
```

Expected now:

```text
timeout can be visible as failed attempt.
firewall_drop as exact reason is future work.
```

## R5. Red/offline connection: block VMLens probe port 18081

This tests connection line separately from app port `8081`.

Run on target VM:

```bash
sudo iptables -I INPUT -p tcp --dport 18081 -j DROP
```

Wait 15-30 seconds, then check graph.

Cleanup on target:

```bash
sudo iptables -D INPUT -p tcp --dport 18081 -j DROP 2>/dev/null || true
```

Expected:

```text
connection probe fails
green idle line can disappear/degrade
app request to 8081 may still work if 8081 is not blocked
```

## R6. Red: route blackhole

Run on source VM. Pick one destination only.

Example from `testing-a-2` blocking route to `testing-a-3`:

```bash
sudo ip route add blackhole 10.20.20.220/32
curl -s -o /dev/null -w "%{http_code}\n" --connect-timeout 1 --max-time 2 http://10.20.20.220:8081/ || true
```

Cleanup:

```bash
sudo ip route del blackhole 10.20.20.220/32 2>/dev/null || true
```

Expected now:

```text
request/probe timeout can be visible
route_unreachable as exact reason is future work
```

## R7. Red/offline: stop vmlens-agent

Run on one VM:

```bash
sudo systemctl stop vmlens-agent
systemctl is-active vmlens-agent || true
```

Wait according to current stale/offline TTL, then check UI/API.

Restore:

```bash
sudo systemctl start vmlens-agent
systemctl is-active vmlens-agent
```

Expected:

```text
VM becomes stale/offline
probe listener 18081 stops
flow ingest stops
```

## R8. Future: DNS failure

Run on source VM:

```bash
for i in 1 2 3; do
  curl --noproxy '*' -s -o /dev/null -w "%{http_code} %{errormsg}\n" --connect-timeout 2 --max-time 4 http://vmlens-does-not-exist.invalid:8081/ || true
done
```

Expected now:

```text
command fails locally
DNS failure is not fully classified by VMLens yet
```

Future reason:

```text
dns_failed
```

## R9. Future: TLS handshake failure

Use the normal plain HTTP server on `8081`, but call it as HTTPS.

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    curl -k -s -o /dev/null -w "%{http_code} %{errormsg}\n" --connect-timeout 2 --max-time 4 "https://$dst:8081/" || true
  done
done
```

Expected now:

```text
TCP connection may be observed
TLS failure reason is future work
```

Future reason:

```text
tls_failed
```

## R10. Future: HTTP 500

Run on target VM:

```bash
sudo fuser -k 8082/tcp 2>/dev/null || true
cat >/tmp/vmlens-http-500.py <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"error"
        self.send_response(500)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args):
        pass

HTTPServer(("0.0.0.0", 8082), H).serve_forever()
PY
nohup python3 /tmp/vmlens-http-500.py >/tmp/vmlens-http-500.log 2>&1 &
ss -ltnp 'sport = :8082'
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8082/" || true)
    echo "$HOSTNAME -> $dst:8082 http500_$i status=$code"
  done
done
```

Cleanup on target:

```bash
sudo fuser -k 8082/tcp 2>/dev/null || true
```

Expected now:

```text
L4 request can be observed
HTTP response code classification is future work
```

Future reason:

```text
http_5xx
```

## R11. Future: HTTP app timeout

Use the slow server from Y4, but call with max-time below server delay.

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    result=$(curl -s -o /dev/null -w "%{http_code} %{time_total} %{errormsg}" --connect-timeout 1 --max-time 1 "http://$dst:8085/" || true)
    echo "$HOSTNAME -> $dst:8085 app_timeout_$i result=$result"
  done
done
```

Expected now:

```text
TCP/request may be observed
app_timeout classification is future work
```

## R12. Future: HTTP auth failure 401

Run on target VM:

```bash
sudo fuser -k 8083/tcp 2>/dev/null || true
cat >/tmp/vmlens-http-401.py <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"unauthorized"
        self.send_response(401)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args):
        pass

HTTPServer(("0.0.0.0", 8083), H).serve_forever()
PY
nohup python3 /tmp/vmlens-http-401.py >/tmp/vmlens-http-401.log 2>&1 &
ss -ltnp 'sport = :8083'
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8083/" || true)
    echo "$HOSTNAME -> $dst:8083 auth_failed_$i status=$code"
  done
done
```

Cleanup on target:

```bash
sudo fuser -k 8083/tcp 2>/dev/null || true
```

Future reason:

```text
auth_failed
```

## R13. Future: HTTP rate limit 429

Run on target VM:

```bash
sudo fuser -k 8084/tcp 2>/dev/null || true
cat >/tmp/vmlens-http-429.py <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"rate limited"
        self.send_response(429)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args):
        pass

HTTPServer(("0.0.0.0", 8084), H).serve_forever()
PY
nohup python3 /tmp/vmlens-http-429.py >/tmp/vmlens-http-429.log 2>&1 &
ss -ltnp 'sport = :8084'
```

Run on source VM:

```bash
for dst in $PEERS; do
  for i in 1 2 3; do
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "http://$dst:8084/" || true)
    echo "$HOSTNAME -> $dst:8084 rate_limited_$i status=$code"
  done
done
```

Cleanup on target:

```bash
sudo fuser -k 8084/tcp 2>/dev/null || true
```

Future reason:

```text
rate_limited
```

## R14. Future: MTU / large packet issue

Run on source VM:

```bash
for dst in $PEERS; do
  ping -M do -s 1472 -c 3 "$dst" || true
done
```

Try larger payload:

```bash
for dst in $PEERS; do
  ping -M do -s 8972 -c 3 "$dst" || true
done
```

Expected now:

```text
ping failure can be observed manually
MTU classification is future work
```

Future reason:

```text
mtu_exceeded
```

## Backend verification

Run from local host / WSL repo:

```bash
curl -sS 'http://localhost:8080/api/vms' \
  | jq 'map({name, private_ip, status, last_seen})'
```

```bash
curl -sS 'http://localhost:8080/api/graph?time_range=5m' \
  | jq '{
    nodes: (.nodes|length),
    edges: (.edges|length),
    pairs: ([.edges[] | select(.source != .target) | [.source,.target] | sort | join("<->")] | unique | length),
    sample: [.edges[] | {source,target,dst_port,request_count,error_count,avg_rtt_ms,last_observed_at}][0:12]
  }'
```

```bash
curl -sS 'http://localhost:8080/api/flows?limit=80' \
  | jq '[.[] | select(.src_ip != .dst_ip)] | {
    rows: length,
    requests: (map(.request_count // 0) | add),
    errors: (map(.error_count // 0) | add),
    recent: .[0:12] | map({observed_at,src_ip,dst_ip,dst_port,request_count,error_count,bytes_sent,bytes_received})
  }'
```

## Recommended quick test order

Use this order for a clean demo:

```text
1. Common cleanup on all VMs
2. Start normal 8081 server on all VMs
3. G1 green from each VM
4. Y1 yellow from testing-a-2
5. R1 red closed port from testing-a-3
6. R2 stop 8081 on one target, test red, restore 8081
7. Backend verification
```

## Notes

- Jangan jalankan burst besar dulu kalau UI terasa berat.
- Untuk test normal, cukup `3-5` request per destination.
- `8081` adalah app test server.
- `18081` adalah reachability probe VMLens agent.
- `65534` dipakai untuk simulasi port closed / failed attempt.
- Heartbeat membuktikan VM/agent online.
- Probe membuktikan VM-to-VM reachable.
- eBPF flow membuktikan real traffic/request.
