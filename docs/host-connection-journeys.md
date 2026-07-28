# Host Connection Journeys

This document defines how VMLens should help users understand and manage
connections between cloud hosts. A connection is treated as a first-class object,
not only as a visual line between two nodes.

## Product goal

Users should be able to answer these questions without reading raw firewall,
subnet, route, or flow rows one by one.

| Question | VMLens answer |
| --- | --- |
| What hosts exist? | VM inventory and host nodes |
| How are hosts connected? | VM-to-VM connection edges |
| Is the connection working? | observed health, RTT, errors, and last seen |
| Is the connection active now? | animated request state on the same edge |
| Is the connection safe? | internal/external scope, public/private boundary, protocol and port |
| What should I do next? | connection detail recommendation |

## User mental model

VMLens maps user intent into observed network behavior.

```text
System intent
  ↓
Host relationship
  ↓
Network configuration
  ↓
Observed connectivity
  ↓
Troubleshooting or action
```

Example:

| Layer | Example |
| --- | --- |
| Intent | Backend should privately access database |
| Relationship | app-host → db-host |
| Network configuration | private network, TCP 5432, firewall allow rule |
| Observed connectivity | healthy, RTT 4 ms, no errors |
| Action | keep, monitor, or validate after changes |

## Core UI flow

### 1. Understand

User opens the project and sees all hosts, even if no traffic exists yet.

```text
Open dashboard
  ↓
See all VM nodes
  ↓
Identify roles, IPs, status
  ↓
See observed connection lines
  ↓
Click host or connection for details
```

Implemented behavior:

| UI object | Meaning |
| --- | --- |
| VM node | cloud host from VMLens inventory |
| Green idle edge | recent reachable or healthy connection |
| Yellow idle edge | reachable but degraded RTT |
| Red edge | failed/refused connection attempt |
| Moving edge | active request/traffic on the same connection |

### 2. Build

User wants two hosts to communicate.

```text
Choose source host
  ↓
Choose destination host
  ↓
Choose protocol and port
  ↓
Apply network/firewall configuration outside VMLens
  ↓
Generate validation traffic
  ↓
VMLens marks the connection as observed
```

Current VMLens scope:

| Capability | Status |
| --- | --- |
| Show source and destination | implemented |
| Show protocol and port | implemented |
| Validate observed traffic | implemented |
| Create firewall rule automatically | future |
| Preview security impact before applying rule | future |

### 3. Operate

User monitors whether communication stays normal.

```text
Open topology
  ↓
Watch connection health
  ↓
Open Connection Flow for L4/network evidence
  ↓
Open Request Flow for TC/eBPF request/attempt evidence
  ↓
Click connection for focused details
```

Connection detail includes:

| Category | Fields |
| --- | --- |
| Identity | source host, target host, source IP, target IP |
| Direction | one-way, two-way, currently active direction |
| Network | scope, protocol, port |
| Health | healthy, degraded, failed, inactive |
| Traffic | bytes, packets, requests, connections |
| Latency | average RTT, p95 RTT, average app delay |
| Errors | error count, last response code |
| Observability | tracker agents, evidence points, last seen |
| Guidance | next recommended action |

### 4. Resolve

User investigates failed or degraded connections.

```text
Click degraded or failed edge
  ↓
Read health state and metrics
  ↓
Check recommendation
  ↓
Inspect matching table rows
  ↓
Fix route/firewall/service/app issue
  ↓
Retest
```

Diagnostic interpretation:

| Signal | Meaning | Likely next check |
| --- | --- | --- |
| Green idle | connection is recently healthy/reachable | validate only if dependency changed |
| Green moving | real request/traffic active | inspect Request Flow or L4 Flow |
| Yellow idle/moving | RTT or app delay is high | inspect route, packet loss, host load, cross-zone path |
| Red | failed attempt, refused, timeout, or error | check service port, firewall, route, destination host |
| No edge | no recent observed connectivity | run probe/request or check agent coverage |

## Important distinction

VMLens separates four concepts that are often mixed together.

| Concept | Meaning | Example |
| --- | --- | --- |
| Architecture relationship | intended dependency | app should access db |
| Configured access | network policy allows it | firewall allows TCP 5432 |
| Observed connection | telemetry saw communication | app sent traffic to db |
| Active request | traffic is happening now | HTTP request currently animated |

An edge is not proof of firewall correctness by itself. The strongest state is:

```text
configured access
  +
observed connection
  +
successful application response
```

## Status semantics

| Status | Source | UI behavior | Meaning |
| --- | --- | --- | --- |
| Healthy | recent successful L4/L7/reachability data | green static line | host path is reachable |
| Degraded | RTT/app delay over threshold or retransmission | yellow static/moving line | path works but is slow |
| Failed | TCP refused, timeout, error, or failed request | red short-lived line | attempted path failed |
| Inactive | no recent telemetry | no line or stale line | not enough recent evidence |
| Unknown | missing mapping/data | IP-only or no detail | cannot fully classify |

Recommended default timing:

| Profile | Request animation TTL | Connection idle TTL | Probe interval | Failure threshold |
| --- | ---: | ---: | ---: | ---: |
| Realtime lab | 2-4s | 15-30s | 5s | 3 |
| Production small fleet | 2-4s | 60-120s | 30s | 3 |
| Production large fleet | 2-4s | 2-5m | 30-60s | 3-5 |

## Data source mapping

| Product data | Primary source | Fallback |
| --- | --- | --- |
| VM inventory | VMLens backend inventory | agent registration |
| Agent health | VMLens agent heartbeat | agent status sweep |
| L4 network flow | VMLens TC/eBPF | kprobe fallback |
| Request flow | VMLens TC/eBPF request/attempt counters | app/proxy/OpenTelemetry logs |
| Reachability | VMLens probe | ICMP/TCP test |
| Public/private boundary | VM inventory + IP classifier | cloud inventory enrichment |
| Change history | cloud control-plane events | future audit log |

## Current implementation in VMLens

| Feature | Implementation |
| --- | --- |
| Single visual edge | one SVG path per VM pair |
| Connection vs request | same line: idle means connection, moving means request |
| Bidirectional traffic | one line with bidirectional animation |
| Connection detail | click the line to open the connection object |
| Host detail | click node to open VM detail |
| Health colors | green/yellow/red based on healthy/degraded/failed |
| Tables | Internal Activity, Connection Flow, Request Flow, L4 Flow |
| Slow rows | yellow table rows with `SLOW RTT` signal |
| Failed rows | red table rows with `FAILED` signal |
| Host incoming/outgoing | click a VM node to inspect who accesses it and what it accesses |
| Host exposure hint | node detail shows public IP and whether external traffic is observed |
| Host impact hint | node detail summarizes observed upstream dependencies |

## First-class connection model

The frontend connection object is normalized from one or more raw telemetry rows.

| Field group | Fields |
| --- | --- |
| Identity | id, source, target, source_label, target_label, source_ip, target_ip |
| Direction | direction, active_direction |
| Health | health, connected, active, failed, slow |
| Network | scope, protocols, ports |
| Traffic | total_bytes, packets, connection_count, request_count, error_count |
| Latency | avg_rtt_ms, p95_rtt_ms, avg_response_duration_ms |
| History | last_seen, last_observed_at |
| Evidence | agent_ids, observation_points, last_response_code |
| Guidance | recommendation |

## Journey coverage matrix

The table below maps each journey requirement to the current VMLens surface.
`Description` points to the implemented UI/API location or the dependency that
keeps an item partial/future.

| Journey | Requirement | Current answer in VMLens | Status | Description |
| --- | --- | --- | --- | --- |
| Understand topology | Show all available hosts | VM topology nodes from inventory | Implemented | `/api/vms`, `App.tsx` VM inventory merge, and `GraphView` VM nodes. |
| Understand topology | Show how hosts are connected | One connection edge per VM pair | Implemented | `GraphView` single-edge rendering and backend `/api/graph` normalization. |
| Understand topology | Distinguish public/private hosts and traffic | Public IP, external scope, and external traffic hints | Partial | `NodeDetailsPanel` shows public IP/exposure hints; full proof needs cloud firewall/route inventory. |
| Understand topology | Show protocol and port | Connection detail plus Connection Flow/L4 tables | Implemented | `EdgeDetailsPanel`, `FlowTelemetryTable`, and `GraphEdge` protocol/port fields. |
| Host connectivity | Show who accesses a host | Incoming connections on VM detail | Implemented | `NodeDetailsPanel` incoming connection list from graph edges. |
| Host connectivity | Show what a host accesses | Outgoing connections on VM detail | Implemented | `NodeDetailsPanel` outgoing connection list from graph edges. |
| Host connectivity | Show impact if a host stops | Observed upstream dependency hint | Implemented | `NodeDetailsPanel` impact/dependency summary from incoming and outgoing edges. |
| Traffic path | Show how request traffic moves | Moving line on the same connection edge | Implemented | `GraphView` edge animation and request TTL handling. |
| Traffic path | Show slow hop/path | Yellow edge and RTT/app delay metrics | Implemented | `GraphView` degraded edge color, `EdgeDetailsPanel` latency fields, and yellow flow rows. |
| Exposure | Show external/public traffic | External scope and IP-only external nodes when available | Partial | VMLens scope classification is available; firewall exposure reason is future cloud integration. |
| Connectivity health | Show whether a connection works | Healthy, degraded, failed, inactive status | Implemented | `GraphView` color state, `EdgeDetailsPanel` health badge, and table severity chips. |
| Build connection | Choose source, target, protocol, and port | Visible after observed validation traffic | Partial | Current UI explains observed source/target/port; creating intended connections needs product workflow and cloud APIs. |
| Build connection | Create firewall or route automatically | Not managed by VMLens yet | Future | Requires cloud provider control-plane integration, authorization, rollback, and audit model. |
| Build connection | Validate connection result | Observed edge plus L4/request evidence | Implemented | Graph edges, Connection Flow, Request Flow, and L4 Flow tabs. |
| Add host | Show new host in architecture | Node appears from inventory/agent registration | Implemented | VM inventory loading and `mergeVMInventory` keep hosts visible even without traffic. |
| Add host | Preview required firewall/routing rules | Not connected to cloud firewall/router APIs yet | Future | Requires intended topology model and provider rule diff preview. |
| Public exposure | Show public endpoint and port | Public IP and external flow evidence where available | Partial | `NodeDetailsPanel` and flow tables expose public IP/scope; provider security-group state is not ingested yet. |
| Public exposure | Enforce safe exposure path | Guidance only | Future | Requires policy engine and cloud-provider write permissions. |
| Segmentation | Show allowed communication zones | Internal/external scope and VM pair topology | Partial | Scope is visible from inventory and telemetry; configured zone policies require cloud network inventory. |
| Admin access | Show bastion or SSH paths | SSH can be observed as L4 but hidden from graph by default noise filters | Partial | Flow data can show port 22; graph excludes common management/noise ports unless filters are changed. |
| Validation | Validate host reachability | Reachability/L4 edge | Implemented | Recent L4/probe-derived connection state in graph edges. |
| Validation | Validate port reachability | Success/failed edge and service port rows | Implemented | Failed edge state, `FlowTelemetryTable` status rows, and `EdgeDetailsPanel` port fields. |
| Validation | Validate application response | Available only with app/proxy/OpenTelemetry enrichment | Partial | Core TC/eBPF telemetry validates L4 request/attempts; HTTP status/path requires app-layer source. |
| Operate health | Monitor traffic, latency, and errors | Stats, graph, tables, and Grafana links | Implemented | `StatCards`, `GraphView`, activity tabs, and Grafana dashboard links. |
| Inspect connection | See one path as a connection object | Click edge to open detail | Implemented | `GraphView` `onConnectionSelect` and `EdgeDetailsPanel`. |
| Review changes | Show who changed firewall/network | Not available yet | Future | Requires cloud audit log ingestion and actor/change timeline storage. |
| Change connection | Preview before/after impact | Not available yet | Future | Requires config-management integration and dependency impact model. |
| Restrict connection | Warn before breaking active traffic | Traffic counters and dependency hints | Partial | `NodeDetailsPanel` and `EdgeDetailsPanel` show observed impact; no automated change guardrail yet. |
| Capacity | Review bandwidth and traffic trend | Stats and flow tables | Partial | Current UI shows recent counters; long-term trend/baseline needs aggregate dashboards. |
| Access review | Find unexpected active traffic | Observed edges and raw rows reveal undocumented traffic | Partial | Supported by graph/table evidence; intended-policy comparison is future. |
| Resolve failure | Show where a connection failed | Red edge, failed row, and next-action text | Partial | Implemented for direct observed edges; exact firewall/route root cause needs provider config data. |
| Trace path | Find first failing hop in multi-hop path | Direct edge status only | Partial | Direct VM-to-VM failures are visible; route-aware multi-hop tracing is future. |
| Latency debug | Distinguish network delay from app delay | RTT is available; app delay needs app-layer source | Partial | `EdgeDetailsPanel` and `FlowTelemetryTable` expose RTT; app delay requires app/proxy/OpenTelemetry enrichment. |
| Public exposure debug | Explain why a host is public | Public IP/external flow evidence | Partial | Public IP and observed external traffic are visible; rule reason needs firewall/route inventory. |
| Change correlation | Show what changed before failure | Not available yet | Future | Requires cloud audit/change event ingestion and correlation with telemetry timestamps. |
| Recovery | Verify recovery after fix | Retest and observe edge returning healthy | Implemented | Graph refresh/SSE returns the edge to green status after successful traffic is observed. |
| Intermittent failure | Compare healthy and failing paths | Manual comparison in tables and graph | Partial | Tables and graph expose evidence; automated baseline comparison is future. |

## Fundamental question map

| Fundamental question | Where the user answers it now |
| --- | --- |
| Host apa saja yang tersedia? | VM topology nodes and VM count |
| Bagaimana host saling terhubung? | single VM-to-VM connection edges |
| Apakah koneksi benar-benar bekerja? | edge health, connection detail, L4/L7 tables |
| Apakah koneksinya aman? | scope, public IP, external traffic evidence; full firewall proof is future |
| Apa dampaknya jika koneksi berubah/gagal? | host incoming/outgoing detail and connection traffic/error counters |
| Apa yang harus dilakukan berikutnya? | connection detail `NEXT ACTION` recommendation |

## What is intentionally not claimed yet

VMLens currently observes and explains connectivity. It does not yet act as the
cloud network orchestrator.

| Capability | Reason |
| --- | --- |
| Create firewall rules | requires cloud provider integration and authorization |
| Modify routes/subnets | requires cloud provider integration and rollback model |
| Prove configured access | requires firewall/security-group/route inventory |
| Show actor/change history | requires cloud audit log ingestion |
| Full multi-hop path resolver | requires route, gateway, load balancer and service mesh topology |

This distinction matters for production messaging:

```text
Current VMLens:
  "This connection was observed, here is its health and evidence."

Future VMLens with cloud integration:
  "This connection is intended, configured, observed, validated, and safe."
```

## Future production phases

### Phase 1: Observe and explain

| Item | Goal |
| --- | --- |
| VM inventory | always show known hosts |
| Connection object | inspect relationship directly |
| TC/eBPF L4 | reliable network evidence |
| App/proxy/OpenTelemetry enrichment | request path, status and app delay evidence |
| Health colors | fast visual triage |
| Raw tables | debug source data |

### Phase 2: Validate and guardrail

| Item | Goal |
| --- | --- |
| Registered service probes | validate known ports without noisy all-to-all probing |
| Port-level status | distinguish host reachable from service reachable |
| Security exposure view | show public/private boundary clearly |
| Intended vs observed | find undocumented active traffic |
| Change event timeline | correlate failure with configuration changes |

### Phase 3: Manage and recover

| Item | Goal |
| --- | --- |
| Firewall/rule integration | create or modify access from VMLens |
| Impact preview | show affected hosts before changes |
| Rollback hints | safer recovery |
| SLO dashboards | connection health over time |
| Tenant retention policies | production data governance |
