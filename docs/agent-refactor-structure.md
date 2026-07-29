# Agent Refactor Structure

Dokumen ini adalah kontrak struktur agent setelah DeepFlow/v1-stack tidak lagi
menjadi sumber utama. Agent mengambil telemetry dari VM sendiri memakai TC/eBPF,
probe ringan, lalu mengirim aggregate flow ke backend.

Prinsipnya:

- `features/traffic/` untuk metric umum semua packet;
- `features/protocols/network/` untuk IPv4, IPv6, ICMP;
- `features/protocols/transport/` untuk TCP/UDP, termasuk RTT dan retransmission;
- `features/protocols/application/` untuk HTTP, DNS, TLS;
- `features/classification/` hanya menentukan jenis protocol;
- `flow/` hanya menggabungkan event menjadi connection/flow state;
- `exporter/` hanya mengubah data agent menjadi payload backend.

## Struktur aktif

```text
agent/
├── cmd/
│   └── agent/
│       ├── main.go
│       └── main_test.go
│
├── internal/
│   ├── features/
│   │   ├── traffic/
│   │   │   ├── packet/
│   │   │   │   ├── collector.go
│   │   │   │   ├── ebpf_collector.go
│   │   │   │   ├── mock_collector.go
│   │   │   │   ├── multi_collector.go
│   │   │   │   └── ebpf_collector_test.go
│   │   │   ├── bytes/
│   │   │   ├── packets/
│   │   │   └── direction/
│   │   │
│   │   ├── protocols/
│   │   │   ├── network/
│   │   │   │   ├── ipv4/
│   │   │   │   ├── ipv6/
│   │   │   │   └── icmp/
│   │   │   ├── transport/
│   │   │   │   ├── tcp/
│   │   │   │   │   ├── connection/
│   │   │   │   │   ├── rtt/
│   │   │   │   │   └── retrans/
│   │   │   │   └── udp/
│   │   │   └── application/
│   │   │       ├── http/
│   │   │       ├── dns/
│   │   │       └── tls/
│   │   │
│   │   └── classification/
│   │
│   ├── flow/
│   ├── pipeline/
│   ├── exporter/
│   ├── config/
│   ├── identity/
│   ├── lifecycle/
│   ├── probe/
│   └── shared/
│       ├── bpf/
│       └── network/
│
└── tests/
```

Runtime eBPF C sekarang mengikuti struktur domain yang sama:

```text
agent/internal/
├── features/
│   ├── traffic/
│   │   ├── packet/flow_tracker.bpf.c
│   │   ├── bytes/
│   │   └── packets/
│   ├── classification/
│   └── protocols/transport/tcp/
└── shared/bpf/
```

`flow_tracker.bpf.c` masih menjadi satu object output (`flow_tracker.bpf.o`)
supaya loader agent tetap stabil. Entrypoint sekarang meng-include program
domain seperti `tc_ingress.bpf.c`, `tc_egress.bpf.c`, dan
`protocols/transport/tcp/connection/connect.bpf.c`.

## Cara membaca path

| Path | Fungsi |
| --- | --- |
| `features/traffic/packet` | load/attach eBPF object, baca ring buffer, convert raw packet/socket event |
| `features/traffic/bytes` | reducer byte sent/received |
| `features/traffic/packets` | reducer packet count |
| `features/traffic/direction` | resolver ingress/egress |
| `features/protocols/network/icmp` | collector ICMP fallback dan parser ICMP |
| `features/protocols/network/ipv4` | parser/model IPv4 |
| `features/protocols/network/ipv6` | parser/model IPv6 |
| `features/protocols/transport/tcp/connection` | request/error hint dari TCP connection signal |
| `features/protocols/transport/tcp/rtt` | model/reducer RTT |
| `features/protocols/transport/tcp/retrans` | model/reducer retransmission |
| `features/protocols/transport/udp` | parser/model UDP |
| `features/protocols/application/http` | HTTP parser/model placeholder |
| `features/protocols/application/dns` | DNS parser/model placeholder |
| `features/protocols/application/tls` | TLS parser/model placeholder |
| `features/classification` | protocol/application classification |
| `flow` | key, state, accumulator, debug string |
| `exporter` | backend sender dan payload API |

## Alur data runtime

```text
agent/cmd/agent/main.go
    ↓
config.Load()
identity.Collect()
exporter.Register()
lifecycle.Run()
probe.Run()
packet.NewEBPF()
    ↓
eBPF raw event
    ↓
features/traffic/packet.convert()
    ↓
features/classification.NormalizePorts()
features/traffic/direction.FromKernel()
features/protocols/transport/tcp/connection.InferRequestCount()
features/traffic/bytes.ApplyDirectionalBytes()
    ↓
flow.Accumulator
    ↓
exporter.Flow()
    ↓
backend POST /api/flows/ingest
```

## Flow state mental model

```text
Flow
├── Traffic
│   ├── Bytes
│   ├── Packets
│   └── Direction
├── TCP
│   ├── Connection
│   ├── RTT
│   └── Retransmission
└── Classification
```

Saat ini flow yang dikirim ke backend tetap memakai `exporter.FlowEvent`, supaya
API backend tidak ikut berubah besar. Model `flow.State` disiapkan untuk fase
berikutnya ketika RTT, retransmission, HTTP, DNS, TLS sudah dikirim sebagai
state gabungan.

## Debug flow

Aktifkan:

```bash
FLOW_DEBUG=true sudo -E bash scripts/install-agent.sh
```

Atau edit VM:

```bash
echo 'FLOW_DEBUG=true' | sudo tee -a /etc/vmlens/agent.env
sudo systemctl restart vmlens-agent
sudo journalctl -u vmlens-agent -f | grep flow_debug
```

Log yang keluar:

```text
flow_debug stage=captured agent=... iface=ens3 10.20.20.130:43000 -> 10.20.20.199:8081 protocol=tcp direction=egress sent=...
flow_debug stage=drain batch_size=...
flow_debug stage=exported agent=... iface=ens3 ...
```

Itu berarti event sudah melewati:

```text
eBPF collector → flow accumulator → backend exporter
```
