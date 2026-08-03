# Telemetry Schema Contract

This is the wire contract. It lists the exact JSON that crosses each hop, what
every field means, and what a consumer may rely on. Semantics of the numbers
themselves are in `metrics.md`; read that first, especially the observer model.

Intended audience: anyone building a VM-provider console, a billing/report job,
or an alerting rule on top of VMLens.

## Hops

```text
VM agent ──POST /api/flows/ingest──────► control plane ──GET /api/flows─────────► console
         ──POST /api/connections/probe─►              ──GET /api/graph──────────►
         ──POST /api/agents/register───►              ──GET /api/internal/activity►
         ──POST /api/agents/heartbeat──►              ──GET /api/realtime (SSE)──►
```

## 1. Agent → control plane: `POST /api/flows/ingest`

One request per flow bucket per window. Source of truth:
`agent/internal/exporter/flow_payload.go`, validated by
`backend/internal/telemetry/metrics/validation.go`.

| Field | Type | Required | Unit | Notes |
| --- | --- | --- | --- | --- |
| `agent_id` | string | yes | — | Rejected if empty. |
| `src_ip` | string | yes | IPv4/IPv6 | The observing VM. Rejected if unparseable. |
| `dst_ip` | string | yes | IPv4/IPv6 | The peer. Rejected if unparseable. |
| `src_port` | int | yes | 0–65535 | Local port, first-seen only, not part of the key. Forced to `0` for ICMP. |
| `dst_port` | int | yes | 0–65535 | Peer port. Forced to `0` for ICMP. Not always the service port. |
| `protocol` | string | yes | — | `tcp`, `udp`, or `icmp`. Lowercased. Anything else is rejected. |
| `direction` | string | no | — | `ingress` or `egress`. Defaults to `egress` if empty. |
| `bytes_sent` | int64 | yes | bytes | Window delta. Negative is rejected. |
| `bytes_received` | int64 | yes | bytes | Window delta. Negative is rejected. |
| `packets` | int64 | yes | count | Window delta. TC path only. |
| `connection_count` | int64 | yes | count | Window delta. Connection attempts. |
| `request_count` | int64 | yes | count | Window delta. If `0`, the backend re-infers it. |
| `error_count` | int64 | yes | count | Window delta. RST count. |
| `retransmission_count` | int64 | omitempty | count | Window delta. |
| `avg_rtt_ms` | float | omitempty | ms | Window average, not a delta. |
| `avg_app_delay_ms` | float | omitempty | ms | Window average of time-to-first-response-byte. |
| `http_1xx_count` … `http_5xx_count` | int64 | omitempty | count | Window deltas. Plaintext HTTP/1.x only; `0` for TLS. |
| `last_http_status` | int | omitempty | 100–599 | Most recent code in the window. Omitted when no status line was seen. Rejected if outside 100–599. |
| `first_seen` | RFC3339 | yes | UTC | Defaults to now if zero. |
| `last_seen` | RFC3339 | yes | UTC | Defaults to `first_seen`. Rejected if before `first_seen`. |
| `interface` | string | no | — | Capture NIC. |

Delivery semantics: fire-and-forget with retry (`exporter/retry.go`). There is
no idempotency key, so a retried window can be counted twice. Counters are
"at least once", not "exactly once".

## 2. Agent → control plane: `POST /api/connections/probe`

Active connectivity probe result. See `probing.md` for policy.

| Field | Type | Notes |
| --- | --- | --- |
| `agent_id` | string | Probing agent. |
| `source_ip` | string | Optional. Probe origin. |
| `dest_ip` | string | Probe target. |
| `protocol` | string | Default `tcp`. |
| `dest_port` | int | Probe target port. Default/fallback `18081`. |
| `success` | bool | Reachable at the probe layer. |
| `rtt_ms` | float | Probe elapsed time. **Not** kernel TCP RTT. |
| `error` | string | Timeout / refused / error text when `success` is false. |
| `source` | string | Emitter tag, e.g. `vmlens_probe`. |
| `type` | string | Probe type, e.g. `connectivity`. |
| `counted_as_request` | bool | Whether this probe may increment request counters. |
| `counted_as_user_traffic` | bool | Always treat `false` as "synthetic traffic, exclude from workload reports". |
| `timestamp` | RFC3339 | Probe result time. |

## 3. Control plane → console: `GET /api/flows`

`model.Flow`. Everything from hop 1 plus resolved context. Counters here are
**cumulative** for the flow bucket, not window deltas.

| Field | Type | Added by | Notes |
| --- | --- | --- | --- |
| `id` | string | backend | Flow bucket UUID. |
| `agent_id` | string | agent | Reporting agent. |
| `src_vm_id` / `dst_vm_id` | string | backend | Empty when the peer is not a registered VM. |
| `src_ip` / `dst_ip` / `src_port` / `dst_port` | — | agent | See hop 1. |
| `protocol` / `direction` | — | agent | See hop 1. |
| `scope` | string | backend | See scope table below. |
| `service` | string | backend | Resolved service name, e.g. `postgresql`, or `tcp/8081`. |
| `service_port` | int | backend | The port the service actually listens on. **Use this, not `dst_port`, for display.** |
| `bytes_sent` / `bytes_received` / `packets` | int64 | backend | Cumulative sums. |
| `connection_count` / `request_count` / `error_count` / `retransmission_count` | int64 | backend | Cumulative sums. |
| `avg_rtt_ms` / `avg_app_delay_ms` | float | backend | Smoothed EMA (α = 0.5), not a lifetime mean. `0` means "no sample", not "0 ms". |
| `http_1xx_count` … `http_5xx_count` | int64 | backend | Cumulative sums. `0` for TLS and non-HTTP flows. |
| `last_http_status` | int | backend | Most recent observed code. Omitted when none was ever seen — which is not the same as a failed request. |
| `requests_per_second` / `connections_per_second` | float | backend | Derived from the reported window with a 1 second minimum divisor. |
| `first_seen` / `last_seen` | RFC3339 | backend | Lifetime of the bucket. |
| `observed_at` | RFC3339 | backend | When the control plane last accepted an update. Use this for freshness. |
| `last_error_at` | RFC3339 or null | backend | Null until an error window arrives. |
| `interface_name` | string | agent | Capture NIC. |
| `created_at` | RFC3339 | backend | Row creation. |

### Scope values

| Value | Meaning |
| --- | --- |
| `internal_same_tenant` | Both ends are registered VMs in the same tenant. |
| `internal_cross_tenant` | Both ends are registered VMs, different tenants. |
| `unknown_internal` | Peer is a private address that is not a registered VM. |
| `external_private` | Peer is private and treated as outside the inventory. |
| `external_public` | Peer is a public address. |
| `unknown` | Not enough information to classify. |

## 4. Control plane → console: `GET /api/graph`

The topology the dashboard draws. `model.Graph` = `{ nodes, edges }`.

An edge merges **both directions** of a VM pair into one record, which is why
edge `avg_rtt_ms` is populated even though a single ingress flow row is not.
When a row and an edge disagree, the edge is the better latency source.

`GraphNode`: `id`, `type` (`vm` / `unknown_internal` / `external` / `unknown`),
`label`, `ip`, `status`, `tenant_id`, `role`, `traffic_in`, `traffic_out`.

`GraphEdge`:

| Field | Type | Notes |
| --- | --- | --- |
| `id`, `source`, `target` | string | Node IDs. |
| `protocol`, `dst_port`, `scope` | — | Representative values for the merged pair. |
| `bytes_sent` … `retransmission_count` | int64 | Sums across both directions. |
| `avg_rtt_ms`, `avg_app_delay_ms` | float | Merged averages. Omitted when zero. |
| `first_seen`, `last_seen`, `last_observed_at` | RFC3339 | Edge lifetime and freshness. |
| `active`, `active_until` | bool / RFC3339 | Drives the moving line. |
| `failed`, `failed_until`, `last_error_at` | bool / RFC3339 | Drives the red line. |
| `reachable` | bool | Set from probe evidence. |
| `kind` | string | `traffic` (observed packets) or `reachability` (probe-only). Never draw a `reachability` edge as workload traffic. |
| `weight` | int | 1–5 bucket derived from total bytes, for line thickness. |

Fields the frontend `GraphEdge` type declares but the API does **not** send
today: `source_ip`, `dest_ip`, `source_role`, `dest_role`, `direction`,
`server_port`, `total_bytes`, `p95_rtt_ms`, `avg_response_duration_ms`,
`last_response_code`, `agent_ids`, `observation_points`. They are either derived
client-side in `GraphView.tsx` or permanently empty. Do not build a second
consumer on them.

## 5. Control plane → console: `GET /api/internal/activity`

`model.InternalActivity`. A VM-to-VM view of the same data, already flipped into
client/server language so a table can render it directly.

| Field group | Fields | Notes |
| --- | --- | --- |
| Observer | `observer_vm_id`, `observer_name`, `observer_ip` | The VM whose agent saw this. |
| Peer | `peer_vm_id`, `peer_name`, `peer_ip` | The other end. |
| Client side | `source_vm_id`, `source_name`, `source_ip` | Resolved connection initiator. |
| Server side | `destination_vm_id`, `destination_name`, `destination_ip` | Resolved connection acceptor. |
| Classification | `protocol`, `direction`, `scope`, `service`, `service_port` | `service_port` is the display port. |
| Ports | `local_port`, `peer_port` | Observer-local and peer-local ports. |
| Counters | `bytes_sent`, `bytes_received`, `connection_count`, `request_count`, `error_count`, `retransmission_count` | Cumulative. |
| Latency | `avg_rtt_ms`, `avg_app_delay_ms` | Same caveats as `/api/flows`. |
| Rates | `requests_per_second`, `connections_per_second` | 1 second minimum divisor. |
| Time | `first_seen`, `last_seen`, `observed_at` | Use `observed_at` for freshness. |

`bytes_sent` here means client → server, and `bytes_received` means
server → client. That is different from `/api/flows`, where the split is by the
observing VM's NIC direction.

## 6. Realtime: `GET /api/realtime`

Server-sent events. The event that matters for tables is `flow.updated`, whose
payload is one `model.Flow` exactly as in hop 3. It is throttled to at most one
broadcast per 500 ms per flow. Treat it as a hint to refresh, not as a complete
stream — no delivery guarantee, no replay.

## 7. Stability rules

For anyone consuming these endpoints:

1. **Additive only.** New fields may appear at any time. Ignore unknown fields;
   never fail on them.
2. **Units live in the field name.** `_ms`, `_bytes`, `_count`, `_per_second`.
   A field without a unit suffix is an identifier or an enum.
3. **`0` is not `null`.** For `avg_rtt_ms` and `avg_app_delay_ms`, `0` means "no
   sample in this window". Do not average zeros into a latency chart.
4. **Enums may gain values.** `scope`, `kind`, `service`, and node `type` are
   open sets. Render unknown values as-is instead of dropping the record.
5. **Counters are at-least-once.** Do not use them for billing without an
   independent reconciliation source.
6. **Delta vs cumulative depends on the hop.** Ingest is a window delta;
   everything read back is cumulative. Do not mix them in one calculation.
7. **Never present eBPF evidence as configuration truth.** See the source
   separation table in `README.md`.
