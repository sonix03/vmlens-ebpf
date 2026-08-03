# VMLens Contracts

VMLens is a network relationship tracker for VM infrastructure. A TC/eBPF agent
on each VM reports who talked to whom and how well that path performed, and the
control plane turns it into a topology a provider console can draw.

This folder is the contract for that data. It exists so the agent team, the
backend team, and whoever builds the provider-facing console can agree on what a
number means before arguing about a graph.

## Read in this order

| Order | File | Answers |
| --- | --- | --- |
| 1 | `metrics.md` | What is measured, how, and what each number is allowed to mean. Start with the observer model in §1. |
| 2 | `telemetry-schema.md` | The exact JSON on each hop, field by field, plus the stability rules for consumers. |
| 3 | `probing.md` | Active connectivity probing: policy, results, and how to read them. |
| 4 | `inventory-cloud-ui.md` | Static inventory format, future cloud-provider data, and UI state mapping. |
| 5 | `known-gaps.md` | Where the current implementation is approximate, and what is dead code rather than a working feature. |

## Source categories

| Source | Purpose | Produces | Answers |
| --- | --- | --- | --- |
| Passive eBPF TC capture | Observe real packets without creating traffic | Flow tuple, bytes, packets, TCP flags | "Who is actually talking to whom?" |
| TCP eBPF kprobes | Read kernel TCP quality signals | RTT, retransmissions | "Is the TCP path healthy?" |
| Active connectivity probe | Validate reachability with controlled test traffic | Reachable/failed, probe RTT, error reason | "Can this host reach that host right now?" |
| Inventory file | Static local assignment and metadata | Name, role, type, network metadata, probe defaults | "What is this host supposed to be?" |
| Cloud provider API | Configuration truth (future) | Firewall, route, subnet, public IP, security group | "Is the network configured correctly?" |

## Core separation

```text
eBPF tells what happened.
Probe tells what currently works.
Inventory tells what we expected locally.
Cloud API tells what was configured.
```

These four must stay separate in code and in the UI. A packet observed by eBPF
is not a configured firewall rule. A successful probe is not a declared intent.
When the console shows a line between two VMs, it must be able to say which of
these four produced it.

## The three rules that cause the most confusion

1. **Every flow is written from the observing VM's point of view.** `src_ip` is
   the VM running the agent, never simply "the IP in the packet header". See
   `metrics.md` §1.
2. **`dst_port` is not always the service port.** On a server-side flow it is
   the client's ephemeral port. Display `service_port` instead.
3. **Ingest counters are window deltas; read-back counters are cumulative.**
   Never mix the two in one calculation. See `telemetry-schema.md` §7.
