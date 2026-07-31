# Inventory, Cloud, and UI Contract

This document defines the static inventory contract, future cloud configuration
contract, UI interpretation rules, known limitations, and recommended next
metrics.

## Inventory contract

`configs/vms.local` is the local static assignment source. It is read by the
backend at startup and applied again when an agent registers. It is not synced
on every API request.

Format:

```text
alias|host|ssh_user|ssh_key|remote_backend|local_backend|proxy_jump|role|type|environment|owner|tenant_id|project_id|region|zone|network_id|subnet_id|public_ip|provider_id|probe_protocol|probe_port|capture_interface|ignore_ports|ignore_ips|flow_allow_cidrs|flow_deny_cidrs|notes
```

| Field group | Used by | Purpose |
| --- | --- | --- |
| `alias`, `host` | Tunnel script, backend | SSH selection and VM display name. |
| `ssh_user`, `ssh_key`, `remote_backend`, `local_backend`, `proxy_jump` | Tunnel script | Reverse tunnel management only. |
| `role`, `type`, `environment`, `owner` | Backend/UI | Host context. |
| `tenant_id`, `project_id`, `region`, `zone` | Backend/UI/future cloud provider | Ownership and placement context. |
| `network_id`, `subnet_id`, `public_ip`, `provider_id` | Backend/UI/future cloud provider | Network/cloud context. |
| `probe_protocol`, `probe_port` | Backend probe target generation | Default validation target for that VM. |
| `capture_interface`, `ignore_ports`, `ignore_ips`, `flow_allow_cidrs`, `flow_deny_cidrs` | Agent/backend policy | Tracking/filtering defaults. |
| `notes` | Operator | Local explanation only. |

Restart rule:

```text
After editing configs/vms.local, restart backend to re-apply metadata:

docker compose restart control-plane
```

## Cloud configuration contract

Cloud provider data is a different source from eBPF. It should be integrated
through provider interfaces, not inferred from packets.

| Data | Source | Status | Purpose |
| --- | --- | --- | --- |
| Public IP assignment | OpenStack/cloud API | Future | Prove public exposure. |
| Firewall/security group rule | OpenStack/cloud API | Future | Explain allow/deny configuration. |
| Route table | OpenStack/cloud API | Future | Explain route availability. |
| Network/subnet | OpenStack/cloud API | Future | Explain private/public boundaries. |
| Change actor/timeline | OpenStack/cloud audit API | Future | Explain what changed and who changed it. |

Rule:

```text
eBPF tells what happened.
Cloud API tells what was configured.
Inventory tells what we expected.
Probe tells what currently works.
```

## UI interpretation contract

| UI state | Data source | Meaning |
| --- | --- | --- |
| Green edge | Observed healthy traffic or successful probe | Connection is currently healthy. |
| Yellow edge | High RTT, retransmission, or slow probe | Connection works but is degraded. |
| Red edge | Failed probe, TCP error, or recent failure evidence | Connection is failing or recently failed. |
| Idle edge | Known edge without recent request movement | Connection exists but is not currently active. |
| Unknown | Missing/insufficient evidence | Do not assume healthy or failed. |

The UI must keep these concepts separate:

```text
Observed connection    = packet/flow was seen.
Validated connection   = active probe succeeded.
Configured connection  = cloud firewall/route says it is allowed.
Intended connection    = inventory/user intent says it should exist.
```

## Known limitations

| Limitation | Current handling |
| --- | --- |
| UDP has no TCP-style connection state | Track packets/ports; infer response if seen. |
| ICMP has no port | Track protocol/type-level flow, port stays empty/zero. |
| HTTPS payload is encrypted | Port and TLS metadata may be visible; HTTP path/body is not. |
| Packet count can be high | Tunnels, SSH, telemetry loops, and long windows increase counts. |
| Firewall truth is not in eBPF | Must come from OpenStack/cloud API. |
| Route truth is not in eBPF | Must come from OpenStack/cloud API. |
| App-level health is not guaranteed by TCP success | Needs service-specific probe. |

## Recommended next metrics

Priority order:

1. Stronger TCP RST/refused/timeout classification.
2. DNS query/response parser.
3. Service-specific probe contract from inventory.
4. HTTP plaintext request/response parser.
5. TLS ClientHello/SNI metadata parser.
6. OpenStack firewall/security-group provider.
7. OpenStack route/subnet/public-IP provider.
8. Change/audit timeline provider.
