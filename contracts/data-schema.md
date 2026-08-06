# Data Schema Contract

This is the storage contract: every table the control plane owns, what writes
it, what reads it, and what a row means. `telemetry-schema.md` covers the JSON
on the wire; this file covers what survives after the request returns.

Source of truth for the DDL is `backend/internal/db/migrations/` (001–012,
applied in filename order at startup). Where this document and a migration
disagree, the migration wins and this file is the bug.

**The diagrams are generated, not drawn.** `make schema-dbml` builds a throwaway
database from the migrations and emits two DBML files; paste either into
[dbdiagram.io](https://dbdiagram.io). They carry every column, type, default,
index, and foreign key with its delete action, plus the notes from
`scripts/schema-comments.sql`. Regenerate in the same change as a migration —
`make schema-dbml-check` fails when either file is stale.

| File | Is | Use it for |
| --- | --- | --- |
| `docs/schema/vmlens.dbml` | The full schema, 15 tables — what Postgres actually has | Reading the system as deployed |
| `docs/schema/vmlens-minimal.dbml` | A **proposal**: 8 tables, minus the columns whose metrics cost more than they return | Deciding what to keep; see §9 |

Both come from one curation pass (`scripts/curate-dbml.js`) that groups tables by
evidence source, colours them to match, and promotes value-list CHECKs to DBML
enums. It fails the build when a new table has no group or a manifest entry
names a column that no longer exists, so neither a migration nor a stale
decision can slip past unnoticed. This file explains what the tables *mean*; the
DBML is the exact shape.

## 1. What is live and what is only declared

Fifteen tables exist. Only eight of them are written by running code. The rest
were created ahead of the cloud-provider integration and are empty in every
deployment today, because the provider is wired to `cloud.NewNoopProvider()`
and no code path inserts into them.

| Table | Written by | Read by | Live? |
| --- | --- | --- | --- |
| `agents` | `POST /api/agents/register`, `/heartbeat`, status sweep | agent list, flow ingest, graph | yes |
| `vms` | register, heartbeat, inventory reload, status sweep | almost every service | yes |
| `vm_interfaces` | register | IP → VM resolution | yes |
| `network_flows` | `POST /api/flows/ingest` | `/api/flows`, `/api/graph`, `/api/internal/activity`, stats | yes |
| `flow_observations` | `POST /api/flows/ingest` | `/api/stats/summary` (60 s window) | yes |
| `external_hosts` | ingest, when scope is external | — (peer registry only) | yes |
| `unknown_internal_hosts` | ingest, resolved on register | stats, graph labels | yes |
| `connection_probes` | `POST /api/connections/probe` | `/api/connections`, graph reachability edges | yes |
| `cloud_public_ips` | nothing | `/api/cloud/context` | declared only |
| `cloud_firewall_rules` | nothing | `/api/cloud/context` | declared only |
| `cloud_routes` | nothing | `/api/cloud/context` | declared only |
| `cloud_network_policies` | nothing | `/api/cloud/context` | declared only |
| `connection_intents` | nothing | connection state service | declared only |
| `connection_configurations` | nothing | connection state service | declared only |
| `connection_change_events` | nothing | nothing | declared only |

Treat "declared only" as schema reserved for the Cloud API evidence source. A
console must not present an empty `/api/cloud/context` as "nothing is
configured" — it means nothing has been synced.

## 2. Relationships

```text
                       agents ──vm_id──┐
                          │            │
                     agent_id          │
                          │            ▼
  vm_interfaces ──vm_id──────────────► vms ◄──── inventory file (configs/vms.local)
                                     ▲  ▲ ▲
                       src_vm_id ────┘  │ └──── dst_vm_id
                                        │
                    network_flows ──────┘   (aggregate state, cumulative)
                          │
                       flow_id
                          │
                          ▼
                  flow_observations              (append-only window deltas)

  connection_probes ──src_vm_id / dst_vm_id──► vms
  unknown_internal_hosts ──resolved_vm_id────► vms
  external_hosts                                (standalone IP registry)

  ── declared, unwritten ───────────────────────────────────────────────
  cloud_public_ips / cloud_firewall_rules / cloud_routes /
  cloud_network_policies / connection_intents /
  connection_configurations ──► vms, and each other
  connection_change_events                     (no FK to a connection table)
```

Delete behaviour matters because agent-discovered VMs can be reaped:

| Parent | Child | On delete |
| --- | --- | --- |
| `vms` | `vm_interfaces` | CASCADE |
| `vms` | `connection_probes` (`src_vm_id`) | CASCADE |
| `vms` | `network_flows`, `flow_observations` (`src_vm_id`, `dst_vm_id`) | SET NULL |
| `vms` | `unknown_internal_hosts.resolved_vm_id` | SET NULL |
| `agents` | `network_flows.agent_id` | SET NULL |
| `network_flows` | `flow_observations` | CASCADE |

So deleting a VM does **not** delete its traffic history; it orphans the rows to
`NULL` VM IDs while keeping the IPs. Deleting the flow bucket *does* delete its
observation history.

## 3. Identity tables

### `agents`

One row per installed agent process. `id` is the agent-chosen ID sent at
registration, not generated by the backend.

| Column | Type | Notes |
| --- | --- | --- |
| `id` | TEXT PK | Agent-supplied. Upsert key. |
| `vm_id` | TEXT | The VM this agent observes from. |
| `hostname`, `machine_id`, `os`, `kernel`, `agent_version` | TEXT | Reported at registration. `machine_id` is the strongest re-identification key across reinstalls. |
| `environment`, `project_id` | TEXT | Inventory or registration metadata. |
| `status` | TEXT | `online` / `stale` / `offline`. Derived, never sent by the agent. |
| `first_seen`, `last_seen` | TIMESTAMPTZ | `last_seen` is bumped by heartbeat *and* by every accepted flow ingest. |

### `vms`

One row per known VM. Created by agent registration (`discovered_by = 'agent'`)
and enriched from `configs/vms.local` by IP match — the inventory wins over
self-reported values for name, tenant, project, role, type, owner, region,
zone, network, subnet, and public IP.

Identity resolution order when an agent registers: existing `machine_id`, then
existing interface IP/MAC, then a stable derived ID. This is why a reinstalled
agent normally keeps its VM row instead of forking a duplicate node.

| Column group | Columns |
| --- | --- |
| Identity | `id` PK, `name`, `machine_id`, `agent_id`, `host_id`, `discovered_by` |
| Addressing | `private_ip` INET, `public_ip` INET, `mac_address` |
| Ownership | `tenant_id`, `project_id`, `owner`, `environment` |
| Placement | `region`, `zone`, `network_id`, `subnet_id` |
| Classification | `role`, `host_type` |
| Lifecycle | `status`, `first_seen`, `last_seen`, `created_at` |

`tenant_id` is what makes `internal_same_tenant` vs `internal_cross_tenant`
possible. A VM with a NULL tenant can never produce a same-tenant flow.

### `vm_interfaces`

Every NIC an agent reported, unique on
`(vm_id, interface_name, ip_address, mac_address)` with `NULLS NOT DISTINCT`.
This is the lookup table that turns a packet's peer IP into a VM ID, so a VM
with an unreported interface will show up as an external peer on that path.

## 4. Observation tables

These two hold the same numbers with opposite semantics. Getting them backwards
is the single most expensive mistake in this schema.

| | `network_flows` | `flow_observations` |
| --- | --- | --- |
| Grain | one row per flow bucket, updated in place | one row per accepted ingest request |
| Counters | cumulative sums since `first_seen` | the window delta exactly as posted |
| Latency | smoothed, `(old + new) / 2` when both are non-zero | the raw window average |
| Time | `first_seen` = LEAST, `last_seen` = GREATEST, `observed_at` = last accepted update | `observed_at` = when the request was accepted |
| Use for | current state, topology, tables | rates, time series, "what happened in the last N seconds" |

### Flow bucket identity

There is **no unique index** on `network_flows`. The bucket is found by a
`SELECT ... FOR UPDATE` on:

```text
src_vm_id, dst_vm_id, src_ip, dst_ip, protocol, dst_port, scope, direction
```

serialised by `pg_advisory_xact_lock` over the same key. Consequences:

- `src_port` is **not** part of the key. It is first-seen-wins and decorative.
- `scope` is part of the key, so a peer that later becomes a registered VM
  starts a **new** bucket instead of merging into the old one.
- Concurrency safety comes from the advisory lock, not from a constraint. A
  direct `INSERT` that bypasses `FlowService.Ingest` can create a duplicate
  bucket and a duplicate graph edge.

### Column semantics beyond the obvious

| Column | Meaning |
| --- | --- |
| `src_ip` | Always the observing VM. Never "the source in the packet header". |
| `dst_port` | The peer's port. On an ingress flow this is the client's ephemeral port; resolve `service_port` in the API layer for display. |
| `direction` | Which way the bytes moved relative to the observing NIC. |
| `packets` | TC path only. A kprobe-only flow has bytes and connections but zero packets. |
| `avg_rtt_ms`, `avg_app_delay_ms` | `0` means "no sample", not "0 ms". Never average zeros into a chart. |
| `error_count`, `last_error_at` | RST count. `last_error_at` stays NULL until the first non-zero error window. |
| `http_1xx_count` … `http_5xx_count`, `last_http_status` | Plaintext HTTP/1.x only, derived in-kernel from the status line. Always zero for TLS — which is not the same as "no errors". |
| `interface_name` | Capture NIC. Kept on update via COALESCE, so it never regresses to NULL. |

No payload is stored anywhere in this schema: no URLs, headers, bodies, query
text, or command lines. `last_http_status` is a three-digit integer and is the
most application-specific value in the database.

### Peer registries

`external_hosts` (public or out-of-inventory private peers) and
`unknown_internal_hosts` (private peers inside `INTERNAL_CIDRS` that are not
registered VMs) are both keyed unique on `ip` and only carry
`first_seen`/`last_seen`. Enrichment columns on `external_hosts` — `domain`,
`asn`, `country`, `provider` — exist but nothing populates them. When an agent
later registers with that IP, the `unknown_internal_hosts` row gets
`resolved_vm_id` and the affected flows are re-pointed at the VM.

## 5. Probe table

`connection_probes` is **current state, not history**. The unique index on
`(agent_id, src_vm_id, dst_vm_id, dst_ip, protocol, dst_port)` means each new
result overwrites `success`, `rtt_ms`, `error`, `last_seen`, and `observed_at`
for that pair. There is no probe time series; a flapping path looks identical
to a stable one.

`rtt_ms` here is the probe's wall-clock elapsed time, a different quantity from
`network_flows.avg_rtt_ms`, which comes from the kernel's TCP RTT estimator. Do
not chart them on one axis.

Probe rows feed graph edges with `kind = "reachability"`. Those edges must never
be rendered as workload traffic — they are synthetic traffic VMLens generated
itself.

## 6. Declared-but-unwritten tables

Kept here so nobody re-invents them, and so nobody mistakes them for a working
feature. All are read by `/api/cloud/context` or the connection state service
and all return empty today.

| Table | Intended content | Key shape |
| --- | --- | --- |
| `cloud_public_ips` | Provider-assigned public IPs and their VM binding | unique on `provider_id` |
| `cloud_firewall_rules` | Direction, protocol, port range, source/dest CIDR, allow/deny | unique on `provider_id` |
| `cloud_routes` | Destination CIDR, next hop, route type | unique on `provider_id` |
| `cloud_network_policies` | Named VM-to-VM allow/deny policy | ownership index only |
| `connection_intents` | What a path is *supposed* to be, with `required` and `purpose` | unique on `(source_vm_id, dest_vm_id, protocol, port, exposure)` |
| `connection_configurations` | Resolved config state per path, linking intent → firewall rule → route | unique on `(source_vm_id, dest_vm_id, protocol, port, COALESCE(network_id,''))` |
| `connection_change_events` | Audit trail with `before_state` / `after_state` JSONB | no reader, no writer |

When these do get populated, they are the Cloud API and Inventory evidence
sources. They must stay joinable to, but visually separate from, the eBPF and
probe tables — see the source separation rule in `README.md`.

## 7. Lifecycle and growth

| Concern | Behaviour |
| --- | --- |
| Status sweep | Every `STATUS_SWEEP_PERIOD` (30 s). `last_seen` within 1 min → `online`, within 5 min → `stale`, else `offline`. Applied to both `agents` and `vms`. |
| VM reaping | Only when `VM_DELETE_AFTER` is set (default `0`, disabled; must exceed 5 minutes). Deletes agent-discovered `vms` and their `agents` rows after the window. |
| Flow buckets | Never deleted. A bucket persists after its VM is reaped, with NULL VM IDs. |
| Observations | **Never deleted.** `flow_observations` gains one row per flow bucket per `FLOW_INTERVAL` (2 s) per agent, forever. This is the table that will fill the disk first; anything long-running needs a retention job or partitioning, and neither exists yet. |
| Duplicate ingest | Delivery is at-least-once with no idempotency key. A retried window is counted twice in both observation tables. Do not bill from these numbers. |

## 8. Indexing

Hot paths are indexed for recency and for join keys:

- Recency: `network_flows(observed_at DESC)`, `(last_seen DESC)`,
  `flow_observations(observed_at DESC)` and `(scope, observed_at DESC)`,
  `connection_probes(success, observed_at DESC)`.
- Partial: `network_flows(last_error_at DESC) WHERE last_error_at IS NOT NULL`
  and `(observed_at DESC) WHERE http_5xx_count > 0` — error-rate questions scan
  only failing rows.
- Joins: `agent_id`, `src_vm_id`, `dst_vm_id`, `src_ip`, `dst_ip`, `scope`,
  `protocol` on flows; `machine_id`, `private_ip`, `public_ip`, `agent_id` on
  VMs; `ip_address`, `mac_address` on interfaces.

The flow bucket lookup in §4 uses eight columns and no covering index; it is
served by the single-column indexes plus the advisory lock. This is fine at lab
scale and is the first thing to revisit under real fleet load.

## 9. The minimal profile

`docs/schema/vmlens-minimal.dbml` is a proposal, not a description. It exists so
the cost of every stored metric can be argued about explicitly. Generating it
changes nothing: the columns stay in Postgres until a migration removes them.

The rule it applies: keep what answers *"is VM A connected to VM B, and is that
path healthy?"*, drop what only decorates the answer.

### Capture profile it matches

Four eBPF attachments instead of the sixteen the source tree defines:

```text
tcx/ingress                  who talks to whom, bytes, SYN -> connections, RST -> errors
tcx/egress                   the same, other direction
kprobe/tcp_rcv_established   RTT
kprobe/tcp_retransmit_skb    retransmissions
```

plus the userspace connectivity probe. Note that the default `CAPTURE_MODE` is
already `tc`, so the eleven socket connection probes are **not attached in a
default deployment** — they only exist in the `kprobe` fallback for kernels
where TCX will not attach. Cutting to four means dropping the three app-delay
programs and deciding whether that fallback is still worth carrying.

### What it removes and why

| Removed | Reason | Cost of removing |
| --- | --- | --- |
| 7 cloud / intent / change-event tables | Nothing writes them; the provider is a noop | Loses the reserved shape for the Cloud API evidence source |
| `packets` | Adds nothing over `bytes` for topology or health | Display columns only |
| `avg_app_delay_ms` | Needs 3 kprobes on `tcp_sendmsg` and `tcp_recvmsg`, the two hottest TCP syscalls, and the number is wrong for pipelined or multiplexed protocols | Display columns only; no filter depends on it |
| `external_hosts.domain` / `asn` / `country` / `provider` | Enrichment columns nothing populates | Nothing — they are already always NULL |

### What it deliberately keeps

`http_1xx_count` … `http_5xx_count` and `last_http_status` survive despite being
the only payload-adjacent capture in the system. `isRequestFlow()` in
`frontend/src/utils/flowFilters.ts` treats an observed HTTP response as request
evidence, because a client's ingress row carries no connection or request
counter of its own. Dropping HTTP status would make those rows disappear from
the requests view — that needs a different request signal first, so it is a
change to design, not a column to delete.

`retransmission_count` and `avg_rtt_ms` stay: two cheap kprobes, and they are
the entire answer to the "healthy" half of the product question.
