# Probing Contract

This document defines active connectivity probing and service-specific probing.
Probing validates connectivity by creating controlled traffic. It should not be
treated as user workload traffic.

## Active connectivity probes

Active probes create controlled traffic from one agent to another target. This is
used to validate reachability, not to discover all possible hosts.

| Probe metric | How it is tracked | Current status | Notes |
| --- | --- | --- | --- |
| Source VM | Agent identity making the probe | Implemented | Comes from registered agent. |
| Destination VM/IP | Target chosen from observed/internal peers and inventory defaults | Implemented | Avoid all-to-all probing by default. |
| Protocol | Inventory/default probe protocol | Implemented | Default is TCP. |
| Destination port | Inventory/default probe port | Implemented | Default/fallback is 18081. |
| Success | TCP connect/probe success | Implemented | Indicates reachable target at probe layer. |
| RTT ms | Probe elapsed time | Implemented | Separate from kernel TCP RTT. |
| Error message | Probe timeout/refused/error string | Implemented | Used for failed edge detail. |
| Observed time | Probe result timestamp | Implemented | Used for graph active/failed state. |

## Recommended active probe policy

```text
Do:
- probe VM pairs that are already observed or assigned
- keep timeout short
- keep interval controlled
- use service-specific probes only for declared dependencies

Do not:
- probe every VM to every VM on every port
- treat probe traffic as user workload traffic
- draw probe-only edges as application traffic without labeling
```

## Service-specific probing

Service probes are future/optional probes that validate application behavior.

| Probe type | Target | Good validation | Status |
| --- | --- | --- | --- |
| TCP connect | Any TCP service | Port reachable | Basic available through connectivity probe |
| HTTP health | HTTP service | HTTP status, latency, optional body check | Future |
| DNS query | DNS service | Query success, response code, latency | Future |
| PostgreSQL | Database host | TCP connect or lightweight protocol handshake | Future |
| Redis | Redis host | `PING` response | Future |
| TLS metadata | HTTPS/TLS service | TLS handshake/SNI/cert metadata | Future |

Service-specific probes must come from inventory or user intent. They should not
be guessed blindly from observed ports.

## Probe interpretation

| Result | Meaning |
| --- | --- |
| Success | The target was reachable at the probe layer. |
| Timeout | Route, firewall, host, or service may be blocking or unavailable. |
| Connection refused | The host was reachable, but the destination port was closed or no service was listening. |
| High RTT | The path works but is slow or congested. |

Probe result is validation evidence. It does not replace passive eBPF traffic
observation and it does not prove firewall configuration by itself.
