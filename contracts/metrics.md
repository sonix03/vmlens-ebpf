# Metrics Contract

This document defines the data VMLens collects from passive eBPF packet capture
and TCP kernel probes.

## Passive eBPF flow metrics

Passive tracking is the base signal. It observes traffic that already exists.

| Metric | How it is tracked | Current status | Notes |
| --- | --- | --- | --- |
| Source IP | Read IPv4/IPv6 source address from packet header at TC ingress/egress | Implemented | Used to map traffic to VM source. |
| Destination IP | Read IPv4/IPv6 destination address from packet header | Implemented | Used to map traffic to VM destination. |
| Protocol | Read IPv4 `protocol` or IPv6 `next_header` | Implemented | TCP, UDP, ICMP are supported. |
| Source port | Read first 2 bytes of TCP/UDP header | Implemented | Client ephemeral ports can be high/random. |
| Destination port | Read bytes 2-3 of TCP/UDP header | Implemented | Service port, e.g. 80, 443, 8081, 5432. |
| Direction | Determined from TC attach direction: ingress or egress | Implemented | Direction is packet direction at the observed VM interface. |
| Interface | From collector attach/config, e.g. `ens3` | Implemented | Currently configured by agent settings/inventory. |
| Bytes | `skb->len` per observed packet | Implemented | Aggregated into flow bytes sent/received. |
| Packets | Increment once per observed packet | Implemented | Large counts are normal for tunnels, SSH, loops, or long windows. |
| TCP flags | Read TCP flag byte | Partial | Used for SYN/request evidence and basic RST/failure signal. |
| Connection count | Derived mostly from TCP SYN/request evidence | Partial | More reliable for TCP than UDP. |
| Request count | Derived from request-like evidence and probes | Partial | Not a full HTTP request parser yet. |
| Error count | Derived from failed probe and basic TCP error evidence | Partial | Needs stronger RST/timeout classification. |

Important rule:

```text
Passive eBPF observes traffic.
It does not prove that a configured connection exists.
It only proves traffic was seen.
```

## Passive eBPF flow path

```text
TC ingress/egress hook
    ↓
read Ethernet header
    ↓
read IPv4/IPv6 header
    ↓
read protocol field
    ↓
if TCP/UDP:
    read source port
    read destination port
    read TCP flags if TCP
    ↓
set bytes = skb->len
increment packet count
    ↓
emit flow event
    ↓
userspace aggregate
    ↓
backend flow table and graph edge
```

## TCP kernel metrics

These metrics come from TCP-specific kernel state. They do not apply to UDP or
ICMP.

| Metric | eBPF hook | How it is tracked | Current status | Notes |
| --- | --- | --- | --- | --- |
| TCP RTT | `kprobe/tcp_rcv_established` | Read TCP socket smoothed RTT (`srtt_us`) and convert to ms | Implemented | Kernel-dependent; attach can be disabled if unsupported. |
| TCP retransmission | `kprobe/tcp_retransmit_skb` | Count retransmission events per flow/socket | Implemented | Good signal for packet loss/congestion. |
| Attached kprobes | Agent collector attach result | Logged by agent at startup | Implemented | Shows which supplemental probes are active. |
| Disabled kprobes | Attach failure or unsupported kernel | Logged by agent at startup | Implemented | Agent should continue with passive TC capture. |

## TCP metric interpretation

| Signal | Meaning |
| --- | --- |
| RTT high | Path or peer is slow, or queueing/congestion exists. |
| Retransmission > 0 | TCP had to resend packets; possible packet loss or congestion. |
| RTT missing | No TCP sample yet, UDP/ICMP traffic, or kprobe unavailable. |

## Protocol notes

| Protocol | What VMLens can track passively |
| --- | --- |
| TCP | IPs, ports, flags, bytes, packets, connection evidence, RTT, retransmission. |
| UDP | IPs, ports, bytes, packets. No TCP-style connection state. |
| ICMP | IPs, protocol/type-level flow. No TCP/UDP port. |

For application requests that do not explicitly specify a port, the operating
system or client library still uses a port. For example, `http://host/` uses TCP
port 80 and `https://host/` uses TCP port 443.
