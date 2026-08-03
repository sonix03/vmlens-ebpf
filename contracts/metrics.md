# Metrics Contract

This document defines what VMLens measures, how each number is produced, and how
a consumer is allowed to read it. It covers passive eBPF packet capture and TCP
kernel probes. Active probing is defined in `probing.md`.

Read this document before building anything on top of `/api/flows`,
`/api/graph`, or `/api/internal/activity`. The exact JSON field list lives in
`telemetry-schema.md`.

## 1. The observer model

This is the single most important rule in VMLens. Every flow record is written
from the point of view of the VM that observed it.

```text
src_ip / src_port  = the observing VM (the VM running the agent)
dst_ip / dst_port  = the peer on the other side
direction          = which way the bytes moved on that VM's NIC
```

`src_ip` is **not** the IP in the packet's source field. The agent normalizes it
before emitting the event:

| Capture path | Where the normalization happens |
| --- | --- |
| TC ingress/egress | `features/traffic/packet/parser.bpf.h` → `fill_directional_tuple()` |
| TCP/UDP kprobes | `features/protocols/transport/tcp/connection/socket_capture.bpf.h` → `socket_metadata()` |

On an ingress packet the parser swaps the packet tuple so the local VM stays in
the source position, then reports `direction = ingress` to say the bytes came
in. This keeps one VM pair on one axis instead of producing mirrored rows.

### What this means per role

| Role of the observing VM | `src_port` holds | `dst_port` holds |
| --- | --- | --- |
| Client (it opened the connection) | Local ephemeral port | Peer **service** port, e.g. 443 |
| Server (it accepted the connection) | Local **service** port, e.g. 443 | Peer ephemeral port |

Consequence for consumers: **`dst_port` is not always the service port.** Use
the backend-resolved `service` / `service_port` fields for anything shown to a
user. `classifyService()` in `backend/internal/service/service_classifier.go`
picks the service side by checking both ports against a known-service table and
an ephemeral-range heuristic.

### Direction values

| Value | Meaning |
| --- | --- |
| `egress` | The observing VM sent these bytes. |
| `ingress` | The observing VM received these bytes. |

A single TCP conversation normally produces **two rows** on the observing VM,
one per direction. They share `src_ip`, `dst_ip`, `protocol`, and `dst_port`.

## 2. Flow identity

Counters are aggregated into buckets. Two different bucket keys exist and they
do not match on purpose.

Agent-side bucket (`agent/internal/flow/key.go`):

```text
agent_id + src_ip + dst_ip + dst_port + protocol + direction + interface
```

Backend-side bucket (`backend/internal/service/flow_service.go`):

```text
src_vm_id + dst_vm_id + src_ip + dst_ip + protocol + dst_port + scope + direction
```

`src_port` is deliberately **not** part of either key. The stored `src_port` is
the first local port observed for that bucket and is informational only. Do not
use it to identify a socket, and do not treat it as stable.

Practical effect: on a server VM, the peer's ephemeral port is part of the key,
so a busy server produces one bucket per client connection. On a client VM the
bucket collapses cleanly onto the peer service port.

Measured in the lab on 2026-08-03: 30 HTTP requests across one VM pair produced
roughly 20 distinct flow rows, because each client connection used a fresh
ephemeral port. Any consumer listing `/api/flows` must group by
`(src_ip, dst_ip, protocol, service_port)` before showing a relationship to a
user, or the same logical connection will appear dozens of times.

## 3. Passive eBPF flow metrics

Passive tracking observes traffic that already exists. It never creates traffic.

| Field | Unit | Produced by | Aggregation | Meaning |
| --- | --- | --- | --- | --- |
| `src_ip` / `dst_ip` | IPv4/IPv6 string | IP header (TC) or socket (kprobe), normalized to observer-first | Part of key | Who talked to whom. |
| `protocol` | `tcp` / `udp` / `icmp` | IPv4 `protocol` / IPv6 `next_header` | Part of key | Transport protocol. |
| `src_port` / `dst_port` | 0–65535 | TCP/UDP header or socket, normalized to observer-first | `dst_port` is part of key, `src_port` is first-seen only | See section 1. Both are `0` for ICMP. |
| `direction` | enum | TC attach hook, or caller of `socket_metadata()` | Part of key | See section 1. |
| `interface` | string | Agent config / registration | Part of agent key | Capture NIC, usually `ens3`. |
| `bytes_sent` / `bytes_received` | bytes | `skb->len` per packet (TC) or `sendmsg`/`recvmsg` return value (kprobe) | Sum over the window | Split by `direction` in the bytes reducer. |
| `packets` | count | +1 per observed packet | Sum over the window | TC path only. Kprobe events do not carry a packet count. |
| `connection_count` | count | TCP SYN without ACK, or a `tcp_v4_connect` / `inet_csk_accept` hit | Sum over the window | Connection **attempts**, not established sessions. SYN retransmits count again. |
| `request_count` | count | Inferred, see section 5 | Sum over the window | Best-effort request evidence. Not an HTTP request count. |
| `error_count` | count | TCP RST flag seen | Sum over the window | Any RST, including RST-based teardown by healthy apps. |
| `retransmission_count` | count | `kprobe/tcp_retransmit_skb` | Sum over the window | One per retransmitted segment. |
| `avg_rtt_ms` | milliseconds | `kprobe/tcp_rcv_established`, see section 4 | Sample-weighted mean in the agent, then smoothed in the backend | TCP path latency. |
| `avg_app_delay_ms` | milliseconds | Socket timing, see section 4 | Sample-weighted mean, then smoothed in the backend | Time from request sent to first response byte. |
| `http_1xx…5xx_count` | count | Status digits parsed in-kernel, see section 4 | Sum over the window | Plaintext HTTP/1.x only. Always `0` for TLS. |
| `last_http_status` | 100–599 | As above | Last observed wins | `0`/absent means no status line was seen. |
| `first_seen` / `last_seen` | RFC3339 UTC | Agent wall clock at event decode | `LEAST` / `GREATEST` | Window boundaries, not kernel timestamps. |

### Reporting window

The agent drains its accumulator on `FLOW_INTERVAL` (default `2s`) and posts one
event per bucket. Every counter in that event is a **delta for that window**, not
a cumulative total. The backend adds deltas onto the stored aggregate row and
also appends the raw delta to `flow_observations`.

`avg_rtt_ms` and `avg_app_delay_ms` are the exception: they are averages for the
window, not deltas.

### Capture path

```text
TC ingress/egress hook  ──┐
kprobe tcp/udp sockets  ──┤
kprobe tcp_rcv_established│──► struct flow_event ──► ring buffer
kprobe tcp_retransmit_skb ┘
                             │
                             ▼
                  decode + normalize (ebpf_collector.go)
                             │
                             ▼
                  reducers: bytes, packets, connection, rtt, retrans
                             │
                             ▼
                  flow.State bucket, drained every FLOW_INTERVAL
                             │
                             ▼
                  POST /api/flows/ingest
```

## 4. TCP kernel metrics

These come from TCP socket state. They do not exist for UDP or ICMP.

| Metric | Hook | How | Notes |
| --- | --- | --- | --- |
| RTT | `kprobe/tcp_rcv_established` | Reads `tcp_sock.srtt_us`, shifts right by 3 (the kernel stores it `<< 3`), converts to ms | Rate limited to one sample per socket per second (`RTT_EMIT_INTERVAL_NS`). |
| Retransmission | `kprobe/tcp_retransmit_skb` | Emits one event with `retransmissions = 1` | Also stamps the current `srtt_us` on the same event, so a retransmission contributes an extra RTT sample outside the 1s rate limit. |

Attach is best-effort. If the kernel does not expose a symbol, the agent logs it
as a disabled kprobe and keeps running with TC capture only. `/api/agents`
reports which probes attached.

### RTT attribution rule

Both kprobes call `socket_metadata(..., DIR_EGRESS)`, so RTT and retransmission
counters are always attributed to the **egress** row of a VM pair. The matching
ingress row keeps `avg_rtt_ms = 0`.

This is why the dashboard falls back to the graph edge RTT when a row has none
(`FlowTelemetryTable.rttForFlow`). Any consumer that reads a single flow row must
do the same, or read RTT from `/api/graph` where both directions are merged.

## Application metrics

These two are the only signals above the transport layer.

### Application delay

Measured from socket timing, not payload. `tcp_sendmsg` stamps the start of an
outstanding request per socket; the return of `tcp_recvmsg` settles the delay
when the response bytes have actually arrived. Reading it at call entry instead
would time the application's readiness rather than the network.

| Property | Value |
| --- | --- |
| Applies to | TCP only. UDP sockets can be unconnected and shared across peers, which would mix conversations. |
| Measures | Time to first response byte on that socket. |
| Does not measure | Per-request duration on a connection carrying several requests. Keep-alive and pipelining break the one-to-one mapping. |
| Encrypted traffic | Works. No payload is involved. |
| Discarded | Samples above 60 s, so an idle keep-alive socket does not report its idle time as latency. |

### HTTP status

The eBPF program reads the first bytes of a TCP payload at TC ingress, checks
for an `HTTP/1.x ` prefix, and converts the three status digits into an integer.

```text
Read in kernel:  "HTTP/1.1 404"
Leaves kernel:   404
```

Nothing else is read and no payload byte is copied into the ring buffer. Request
lines, URLs, headers, cookies, reason phrases, and bodies are never touched. See
`docs/privacy.md`.

| Property | Value |
| --- | --- |
| Applies to | Plaintext HTTP/1.x over TCP. |
| Encrypted traffic | Reports nothing. The plaintext is not on the wire. |
| Granularity | Class counters plus the most recent code, per flow bucket. |
| Absent value | `0` means no status line was observed, which is the normal case for every non-HTTP and every TLS flow. It does not mean "no response". |

A consumer must not read a missing status as an application failure. Combine it
with `error_count` and probe evidence before calling a path unhealthy.

### Backend RTT aggregation

The backend does not compute a true mean across windows. It applies:

```sql
avg_rtt_ms = CASE
    WHEN new > 0 AND avg_rtt_ms > 0 THEN (avg_rtt_ms + new) / 2
    WHEN new > 0 THEN new
    ELSE avg_rtt_ms
END
```

That is an exponential moving average with α = 0.5, so the stored value tracks
recent windows and is not a lifetime average. Read `avg_rtt_ms` as "smoothed
recent RTT". The same rule applies to `avg_app_delay_ms`.

## 5. Derived counters

### request_count

There is no HTTP parser in the passive path. `request_count` is inferred:

```text
if error_count > 0            → 0
else if connection_count > 0  → connection_count
else if protocol is UDP/ICMP and this window carried bytes → 1
else                          → 0
```

Implemented in `connection.InferRequestCount` (agent) and re-applied by
`metrics.InferRequestCount` (backend) only when the agent sent `0`.

Two consequences worth stating plainly:

- For TCP, `request_count` equals `connection_count`. It counts connection
  attempts, not requests inside a connection. HTTP keep-alive traffic will show
  many bytes and few requests.
- A window with any RST reports `request_count = 0`, even if that window also
  carried successful traffic.

### error_count

`error_count` counts RST packets. Applications that close with RST instead of
FIN will register errors on a healthy path. Treat a non-zero `error_count` as
"something reset a connection", not as "the path is broken". Combine it with
probe results from `probing.md` before showing a red edge.

## 6. Protocol coverage

| Protocol | Available |
| --- | --- |
| TCP | IPs, ports, direction, bytes, packets, connection attempts, RST errors, RTT, retransmissions. |
| UDP | IPs, ports, direction, bytes, packets. No connection state, no RTT. |
| ICMP | IPs, direction, bytes, packets. Ports are forced to `0`. |

For requests that do not name a port, the port still exists on the wire:
`http://host/` is TCP 80 and `https://host/` is TCP 443.

## 7. What passive capture does not prove

```text
Passive eBPF observes traffic.
It does not prove that a configured connection exists.
It only proves traffic was seen.
```

VMLens never captures packet payloads, HTTP bodies, TLS plaintext, SSH content,
database queries, files, or command lines.

Known accuracy gaps and cleanup items are tracked in `known-gaps.md`.
