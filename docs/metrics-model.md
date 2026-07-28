# VMLens Metrics Model

Dokumen ini mendefinisikan metric utama VMLens untuk telemetry core berbasis TC/eBPF agent.

## Metric groups

| Group | Source utama | Fungsi UI | Catatan |
|---|---|---|---|
| `bytes` | TC/eBPF `ens3`, socket fallback | throughput, traffic volume, edge weight | Tidak membaca payload. Hanya ukuran packet/socket IO. |
| `packets` | TC/eBPF `ens3` | packet rate, baseline network activity | Untuk ICMP/UDP/TCP packet-level signal. |
| `ports` | TC/eBPF + socket metadata | service port, client port, failed port | ICMP dinormalisasi ke port `0`. |
| `tcp_state` | TCP flags dari TC/eBPF | SYN connection attempt, RST failed attempt | SYN tanpa ACK = connection attempt. RST = failed/error signal. |
| `request_response` | L4 inference + optional service probe | request count, error count, request flow animation | Tidak dihitung dari heartbeat/probe connectivity. |
| `rtt` | connectivity probe | hijau/kuning connection line | RTT graph sekarang dari probe, bukan eBPF packet timing. |
| `retransmit` | future TC/eBPF sequence tracking | packet loss/retrans warning | Dipisah dari `error_count`; belum aktif sebagai metric utama. |
| `severity` | backend metric normalization | green/yellow/red UI state | normal/warning/error dari success, RTT, error count. |

## Color rule

| Color | Meaning | Source |
|---|---|---|
| Green | VM-to-VM reachable dan RTT normal | successful reachability probe |
| Yellow | VM-to-VM reachable tapi RTT lambat | successful probe dengan RTT melewati threshold |
| Red | connection/service attempt gagal | failed probe, TCP RST, timeout, closed port |

## Request vs connection

| Type | Counted as user traffic | Visual |
|---|---:|---|
| Connectivity probe | no | idle connection line |
| Service probe | no by default | service health signal |
| Real L4 flow | yes | connection/request flow evidence |
| Real L7/app request | yes when available | moving request animation |

## Code ownership

| Layer | Folder | Responsibility |
|---|---|---|
| Kernel capture metrics | `agent/ebpf/programs/metrics/` | Small eBPF helpers for bytes, packets, ports, TCP state, request/error signal |
| Agent normalization | `agent/internal/metrics/` | Convert raw capture event into VMLens telemetry counters |
| Backend normalization | `backend/internal/telemetry/metrics/` | Validate, infer request count, calculate rate/severity, normalize flow metrics |

## Production defaults

| Setting | Lab | Production large fleet |
|---|---:|---:|
| Reachability probe interval | `5s` | `30s` |
| Request animation TTL | `2-4s` | `2-4s` |
| Connection idle TTL | `30s` | `30-60s` |
| Agent stale timeout | `20s` | `2m` |
| Agent offline timeout | `90s` | `10m` |
| Service probe interval | `10s` | `60s` |

## Non-goals

- Tidak mencoba menjadi full L7 APM/tracing system.
- Tidak membaca packet payload.
- Tidak mencampur heartbeat/probe ke `request_count` atau `bytes` user traffic.
- Tidak menjadikan failed service port sebagai VM offline; VM reachability dan service reachability tetap dipisah.
