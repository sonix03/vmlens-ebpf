# Known Gaps

Audit of the metric path, from eBPF hook to dashboard table, taken on
2026-08-03. Each entry says what the code actually does, why it matters for a
VM-provider console, and what to do about it.

Nothing here is a crash or a data-loss bug. These are accuracy and clarity gaps
that a consumer must know about, plus dead code that should not be mistaken for
a working feature.

## Fixed in this pass

| Item | Was | Now |
| --- | --- | --- |
| `connection.Model.CloseCount` | Declared and merged, never incremented by any producer. `struct flow_event` has no close counter. | Removed. |
| `InferRequestCount` applied twice in the agent | Called in `ebpf_collector.convert()` and again in `connection.Apply()` when the result was `0`. | Collector no longer infers. The reducer owns the rule. |
| `agent/internal/exporter/payload.go` | `type Payload interface{}` plus an identity function `FlowPayload()`. No references. | Removed. |
| `agent/internal/flow/direction.go` | `DirectionInternal` / `DirectionExternal` constants. No references. Scope is a backend concept. | Removed. |
| `agent/internal/flow/correlator.go` | `Correlate()` wrapper that only called `KeyFromMetric()`. No references. | Removed. |
| `agent/internal/flow/cleanup.go` | `Accumulator.Reset()`. No references. | Removed. |

## Verified against the lab on 2026-08-03

Run against `testing-a-1` (10.20.20.130), `testing-a-2` (10.20.20.199), and
`testing-a-3` (10.20.20.220) using `docs/vmlens-three-vm-traffic-test.md`:
30/30 green requests on port 8081 returned HTTP 200, and 6 red requests on the
closed port 65534 returned `000` as expected.

| Claim | Result |
| --- | --- |
| Observer model: `src_ip` is always the local VM | Confirmed. Every conversation appears as one egress and one ingress row per observing VM. |
| `dst_port` is the peer port, not the service port | Confirmed. Server-side rows carry the client's ephemeral port and rely on `service` to recover `http-alt`. |
| Server-side buckets multiply per client connection | Confirmed, and worse than described. 30 requests produced roughly 20 separate flow rows for the single `.199 ↔ .130` relationship. |
| G5: RST counted as error on a healthy path | Confirmed. The `.130 ↔ .220` port-8081 edge reported `err = 10` while all 30 HTTP requests succeeded. |
| G1: RTT only on the egress row | **Not testable here.** The deployed agent is `dist/agent/v3.1`, built before commit `8d66cd7` ("Attach TCP kernel probes"). It contains zero kprobe symbols, so no kernel RTT is collected at all. |

On RTT specifically: every `traffic` edge reported `avg_rtt_ms = 0.00`, and the
only non-zero RTT in the system came from `reachability` edges on port 18081,
which is the active connectivity probe described in `probing.md`. G1 stays open
and unverified until an agent containing the kprobes is deployed. A build that
has them already exists at `dist/agent/dev-rtt-retrans/`.

## Open: accuracy

### G14. A refused connection is recorded as a successful request

Found during the red test. `.199` made 3 connection attempts to the closed port
65534 on `.220`. VMLens recorded:

```text
10.20.20.199 -> 10.20.20.220  egress   dport 65534  conn 3  req 3  err 0
10.20.20.199 -> 10.20.20.220  ingress  dport 65534  conn 0  req 0  err 3
```

The SYN attempts land on the egress row. The RST that proves they failed lands
on the ingress row, because `direction` is part of the flow key. The two never
meet, so:

- The egress row reports 3 requests and **zero** errors. Read alone, a refused
  connection is indistinguishable from three successful ones.
- The `error_count > 0 → request_count = 0` rule in `InferRequestCount` never
  fires, because the row holding the errors is not the row holding the attempts.

This is the most misleading result in the current data model: the failure
evidence exists, but not on the row a consumer would read to judge the attempt.

- Options: correlate the RST back to the attempt row before export, or drop
  `direction` from the key for TCP error attribution the same way G1 proposes
  for RTT.

### G1. RTT and retransmissions only land on the egress row

`emit_tcp_rtt_sample()` and `emit_tcp_retransmission()` both call
`socket_metadata(..., DIR_EGRESS)`, and `direction` is part of the flow key. The
ingress row of the same VM pair therefore always reports `avg_rtt_ms = 0` and
`retransmission_count = 0`.

- Impact: half of all flow rows show no latency. The dashboard papers over this
  by borrowing RTT from the graph edge (`FlowTelemetryTable.rttForFlow`), so any
  other consumer that reads `/api/flows` directly will draw a different picture
  than the UI.
- Options: derive the direction from socket state in
  `socket_capture.bpf.h`, or drop `direction` from the RTT attribution key in
  the agent reducer so the sample applies to both rows of the pair.
- Until then: read RTT from `/api/graph`, which merges both directions.

### G2. Retransmissions inject an unrate-limited RTT sample

`retransmit.bpf.h:27` stamps `event->rtt_us = tcp_smoothed_rtt_us(sk)` on every
retransmission event. The RTT collector rate-limits itself to one sample per
socket per second (`RTT_EMIT_INTERVAL_NS`); this path bypasses that limit.

- Impact: during loss, RTT is sampled far more often and at its worst values, so
  `avg_rtt_ms` is pulled up by exactly the events that are already reported
  separately as `retransmission_count`. The two signals are not independent.
- Options: stop stamping RTT on the retransmission event, or tag it so the
  reducer can weight it separately.

### G3. `avg_rtt_ms` in the database is an EMA, not an average

`flow_service.go:122` merges with `(avg_rtt_ms + new) / 2`. The agent already
sends a sample-weighted window mean; the backend then halves its influence every
window regardless of how many samples each window held.

- Impact: the field name promises an average and delivers a recent-weighted
  smoothing. A one-off 900 ms spike still visibly moves the stored value two
  windows later.
- Options: carry `rtt_sample_count` on the ingest payload and do a weighted
  merge, or rename the field to `smoothed_rtt_ms` and document it as such.
- Documented in `metrics.md` §4 for now.

### G4. `request_count` is connection attempts wearing a different name

For TCP, `request_count` is a copy of `connection_count`. There is no request
parser in the passive path. A keep-alive HTTP client that issues 10,000 requests
over one connection reports `request_count = 1`.

Additionally, any window containing an RST reports `request_count = 0`, which
discards successful request evidence from that same window.

- Impact: "requests per second" on the dashboard is really "new connections per
  second". This is the field most likely to be misread by a provider console.
- Options: land the HTTP/DNS parsers that already exist under
  `features/protocols/application/` and feed real request counts, or rename to
  `connection_attempt_count` and remove the request framing.

### G5. `error_count` counts every RST

`tcp_is_reset()` flags any packet with the RST bit. Applications that close with
RST instead of FIN, and port scans, both register as errors on a healthy path.

- Impact: red rows and red edges on working links.
- Options: distinguish RST-during-handshake (refused) from RST-after-established
  (abortive close), and classify timeouts separately.

### G6. `avg_app_delay_ms` had no producer — resolved

Previously the field flowed from the agent to a dashboard column that could
never show a value. It is now produced from socket timing in
`features/protocols/application/delay/delay.bpf.h`: `tcp_sendmsg` stamps the
start of an outstanding request per socket and the return of `tcp_recvmsg`
settles the delay once response bytes have arrived.

Remaining limits, documented in `metrics.md`:

- It is time-to-first-response-byte on a socket, so it equals request latency
  only for a one-request-at-a-time exchange. Keep-alive and pipelining break it.
- TCP only.
- Requires an agent built after this change. Older agents still report `0`.

### G7. Event provenance is inferred from empty fields

`struct flow_event` carries no event-type tag, so `rawEventSource()`
(`ebpf_collector.go:351`) guesses whether a ring-buffer record came from TC or
from a TCP kprobe by checking whether bytes, packets, and connections are all
zero. The resulting `Source` value is then used only for debug logging
(`flow/debug.go:14`) and is dropped before export.

- Impact: low today, but the heuristic silently misclassifies any future event
  type that carries no bytes, and there is no way for the backend to tell which
  hook produced a number.
- Options: add a `u8 source` field to `struct flow_event` and propagate it to
  the ingest payload. Requires rebuilding `flow_tracker.bpf.o`.

### G17. HTTP status is invisible for encrypted traffic

Status codes come from reading a plaintext `HTTP/1.x` status line at TC ingress.
That covers plain HTTP and nothing else.

- TLS reports no status, because the plaintext never appears on the wire.
  Reaching it would need uprobes on the TLS library's `SSL_read`/`SSL_write`,
  which breaks across library versions and is far more invasive than the current
  read.
- HTTP/2 and HTTP/3 are not covered either: the status lives in a HPACK/QPACK
  compressed header block, not a text line.
- A missing status therefore means "not visible", never "no response". The
  dashboard renders it as `—` with that wording in the tooltip, and a consumer
  must not treat absence as failure.

## Open: modelling

### G8. `src_port` is stored but not part of the identity

Neither the agent key (`flow/key.go`) nor the backend aggregate key includes
`src_port`, yet both store one. The stored value is whichever local port was
seen first for that bucket.

On a server VM this is harmless, because the peer's ephemeral port is in the key
and each client connection gets its own bucket. On a client VM with many local
sockets to the same peer service, the displayed `src_port` is arbitrary.

- Documented in `metrics.md` §2. No code change proposed — adding `src_port` to
  the key would multiply row counts on busy clients.

### G15. One topology edge covers every port of a VM pair

`graphEdgeID(sourceID, targetID, protocol, scope)` in `graph_service.go:270`
does not include the port. Every TCP flow between a VM pair in the same scope
collapses into a single edge, and `preferredGraphPort()` merely picks which port
to *display*, preferring a non-ephemeral one.

Observed in the lab: the 3 failed attempts on port 65534 were summed into the
same edge as the healthy port-8081 traffic, so the graph showed
`.199 → .130 tcp 8081 request_count=8 error_count=3` even though port 8081 never
failed.

- Impact: a failure on any port marks the whole VM-pair relationship as
  unhealthy, and the port label on the edge is not the port that failed. For a
  console whose job is "prove this specific connection works", this is the gap
  between a useful answer and a misleading one.
- Options: key the edge by service port as well, or keep one edge per pair but
  carry a per-port health breakdown so the UI can drill in.

### G16. The service side is a guess when both ports are ephemeral

`classifyService()` picks the service side by looking both ports up in a known
-service table, then falling back to an ephemeral-range heuristic
(`port >= 32768`), then to `direction`. When neither port is a known service and
both are above 32768, the result is a coin flip.

Observed in the red test, where the real target was port 65534:

```text
.199 -> .220  egress   service tcp/65534  (correct)
.199 -> .220  ingress  service tcp/38110  (wrong; 38110 is the client's port)
```

Both rows describe the same conversation and disagree about which end is the
server.

- Impact: `service` and `service_port` are presented as resolved facts, but for
  high-numbered service ports they are inferences.
- Current handling: the dashboard marks this case. `FlowTelemetryTable` and
  `InternalActivityTable` render the client/server port boxes with a dashed
  amber border whenever the resolved service port is itself ephemeral, so an
  inferred split never reads as observed evidence.
- Options: carry the SYN direction from eBPF through to the backend so the
  connection initiator is known rather than guessed. That would settle G14 too.

### G9. Three copies of the request-inference rule

`connection.InferRequestCount` (agent) and `metrics.InferRequestCount` (backend)
implement the same rule with different signatures and a subtly different UDP
branch: the agent checks a single event byte count, the backend checks
`bytes_sent` for egress and `bytes_received` for ingress.

- Impact: the two can drift. Today the backend copy only runs when the agent
  sends `0`, so the divergence is mostly latent.
- Options: extract a shared module. `agent/` and `backend/` are separate Go
  modules under one `go.work`, so this needs a third module.

### G10. `flow.State.Traffic.Direction` duplicates `Key.Direction`

`store.go:69` and `store.go:101` maintain `Traffic.Direction.Current`, but
direction is already part of the bucket key, so the value can never differ. It is
never exported.

- Options: remove the field, or start using it to record mixed-direction buckets
  if the key ever loses `direction`.

### G11. RTT min/max/current are computed and thrown away

`rtt.Model` tracks `CurrentMs`, `MinMs`, and `MaxMs`, and `mergeRTT()` maintains
them across merges, but `flow_payload.go:47` exports only `AvgMs`.

- Impact: work done per event for no output. A provider console asking "how bad
  did it get?" cannot be answered even though the agent knows.
- Options: add `min_rtt_ms` / `max_rtt_ms` to the ingest payload and the flow
  table, or delete the fields.

### G12. `agent/internal/shared/network` is unimported

`address.go`, `direction.go`, `endpoint.go`, `flow_key.go`, and `protocol.go`
define a typed network vocabulary that nothing imports. `direction.go` also
duplicates the string constants that `features/traffic/direction` actually uses.

The package is named in `docs/agent-refactor-structure.md`, so it is treated as
planned scaffolding and left in place. If the refactor is not going to land,
delete it — right now it reads as the real model when it is not.

## Open: dashboard

### G13. The frontend `GraphEdge` type is wider than the API

`frontend/src/types/graph.ts` declares `p95_rtt_ms`, `agent_ids`,
`observation_points`, `server_port`, `source_ip`, `dest_ip`,
`avg_response_duration_ms`, and `last_response_code`. `/api/graph` sends none of
them. `GraphView.tsx` reads them, gets `undefined`, and folds the defaults into
the connection summary.

- Impact: `EdgeDetailsPanel` permanently shows "P95 RTT —", "Tracker agents —",
  and "Evidence points —". A reader cannot tell whether that means "no data yet"
  or "never implemented".
- Options: emit the fields from the backend, or remove them from the type and
  the panel until they exist.

## Priority

Reordered after the 2026-08-03 lab run, which promoted two findings above the
code-reading ones.

1. **G14** — a refused connection reads as three successful requests. Confirmed
   live, and it breaks the core promise that the console can tell a working path
   from a failing one.
2. **G15** — one edge per VM pair means a failure on any port reddens every
   port. Confirmed live.
3. G5 — RST counted as error made a fully healthy path report 10 errors.
   Confirmed live.
4. G1 — one-sided RTT. Still unverified; needs the kprobe build deployed first.
5. G4 — `request_count` is connection attempts wearing a different name.
6. G3 — rename or fix, but stop calling an EMA an average.
7. G6 and G13 — remove or fill the permanently empty columns.
8. G2, G7, G9, G11 — correctness and tidiness work with no consumer impact yet.
