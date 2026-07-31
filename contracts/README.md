# VMLens Contracts

This folder defines the tracking contract for VMLens. The purpose is to make it
clear which data is collected, where each signal comes from, and how the UI
should interpret it.

## Contract files

| File | Scope |
| --- | --- |
| `metrics.md` | Passive eBPF flow metrics and TCP kernel metrics. |
| `probing.md` | Active connectivity probing and service-specific probing. |
| `inventory-cloud-ui.md` | Static inventory, future cloud configuration data, UI states, limitations, and next metrics. |

## Source categories

| Source | Purpose | Produces | Should answer |
| --- | --- | --- | --- |
| Passive eBPF TC capture | Observe real packets without creating traffic | Flow, IP, port, protocol, bytes, packets, TCP flags | "Who is actually talking to whom?" |
| TCP eBPF kprobes | Read kernel TCP quality signals | RTT, retransmission | "Is the TCP path healthy?" |
| Active connectivity probe | Validate reachability by creating controlled test traffic | Reachable/failed, probe RTT, error reason | "Can this host reach that host now?" |
| Inventory file | Static local assignment and metadata | Name, role, type, network metadata, probe defaults | "What is this host supposed to be?" |
| Cloud provider API | Future configuration truth | Firewall, route, subnet, public IP, security group | "Is the network configured correctly?" |

## Core separation

```text
eBPF tells what happened.
Probe tells what currently works.
Inventory tells what we expected locally.
Cloud API tells what was configured.
```

These sources must stay separate in code and UI. A packet observed by eBPF is
not the same thing as a configured firewall rule. A successful probe is not the
same thing as a user-declared intent.
