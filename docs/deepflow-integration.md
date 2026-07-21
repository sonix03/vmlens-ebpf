# DeepFlow integration

VMLens can use DeepFlow as an external telemetry source for VM-centric topology.
The product flow is:

```text
DeepFlow L4/L7 raw rows
  -> filter by VMLens VM inventory
  -> map IPs to VMs
  -> deduplicate observation points
  -> emit VM-to-VM / VM-to-external topology edges
```

Do not render raw DeepFlow rows directly as topology. DeepFlow can emit the same
request at several tap sides:

```text
c   = Client NIC
c-p = Client Process
s   = Server NIC
s-p = Server Process
```

VMLens deduplicates duplicated L7 rows by priority:

```text
s-p > s > c-p > c
```

## Local DeepFlow services

Expected local lab endpoints:

```text
Grafana        http://localhost:3001
Querier API    http://localhost:20416
Controller API http://localhost:30417
ClickHouse     http://localhost:8123
Container      deepflow-clickhouse
```

If ClickHouse is not exposed on `localhost:8123`, either expose it from the
DeepFlow compose stack or put the vmlens control-plane container on the same
Docker network and set:

```bash
DEEPFLOW_CLICKHOUSE_URL=http://deepflow-clickhouse:8123
```

Temporary lab command when an existing DeepFlow network already exists:

```bash
DEEPFLOW_CLICKHOUSE_URL=http://deepflow-clickhouse:8123 \
DEEPFLOW_QUERIER_URL=http://deepflow-server:20416 \
DEEPFLOW_CONTROLLER_URL=http://deepflow-server:30417 \
docker compose up -d --build

docker network connect <DEEPFLOW_NETWORK_NAME> vmlens-ebpf-control-plane-1
docker restart vmlens-ebpf-control-plane-1
```

Example network name from the current lab:

```bash
docker network connect deepflow-runtime-0721_deepflow vmlens-ebpf-control-plane-1
docker restart vmlens-ebpf-control-plane-1
```

## Configuration

Copy `.env.example` to `.env`, then adjust:

```bash
DEEPFLOW_ENABLED=true
DEEPFLOW_CLICKHOUSE_URL=http://host.docker.internal:8123
DEEPFLOW_CLICKHOUSE_DATABASE=default
DEEPFLOW_CLICKHOUSE_USERNAME=default
DEEPFLOW_CLICKHOUSE_PASSWORD=
DEEPFLOW_QUERIER_URL=http://host.docker.internal:20416
DEEPFLOW_CONTROLLER_URL=http://host.docker.internal:30417
DEEPFLOW_DEFAULT_WINDOW=30m
DEEPFLOW_QUERY_TIMEOUT=5s
DEEPFLOW_MAX_LIMIT=1000
DEEPFLOW_MASK_EXTERNAL_IPS=false
DEEPFLOW_REQUIRE_INVENTORY_FILTER=true
```

`DEEPFLOW_REQUIRE_INVENTORY_FILTER=true` is the safe default. It means VMLens
only queries and returns rows involving IPs from the VMLens VM inventory.

## Start VMLens

```bash
docker compose up -d --build
```

Check the regular API:

```bash
curl http://127.0.0.1:8080/health
```

Check DeepFlow health:

```bash
curl http://127.0.0.1:8080/api/deepflow/health
```

Expected healthy signals:

```json
{
  "enabled": true,
  "clickhouse_reachable": true,
  "agent_list_not_empty": true
}
```

## API endpoints

Topology graph:

```bash
curl 'http://127.0.0.1:8080/api/deepflow/graph?time_range=30m&limit=500'
```

Raw L4/L7 logs:

```bash
curl 'http://127.0.0.1:8080/api/deepflow/raw/flows?time_range=30m&limit=100'
```

Health/status:

```bash
curl 'http://127.0.0.1:8080/api/deepflow/health'
```

Optional filters:

```bash
tenant_id=<tenant>
project_id=<project>
vm_id=<vm-id>
mask_external_ips=true
```

## Required DeepFlow queries

L4:

```sql
SELECT
  time,
  toString(ip4_0) AS source_ip,
  toString(ip4_1) AS dest_ip,
  concat(toString(if(l3_epc_id_0=-2,1,0)), ' -> ', toString(if(l3_epc_id_1=-2,1,0))) AS internet_direction,
  client_port,
  server_port,
  multiIf(protocol=6, 'tcp', protocol=17, 'udp', toString(protocol)) AS protocol,
  toString(status) AS status,
  byte_tx,
  byte_rx,
  byte_tx + byte_rx AS total_bytes,
  round(rtt/1000,3) AS rtt_ms,
  retrans_tx + retrans_rx AS retrans_total,
  toString(agent_id) AS agent_id,
  l3_epc_id_0,
  l3_epc_id_1
FROM flow_log.l4_flow_log
WHERE time > now() - INTERVAL 30 MINUTE
ORDER BY time DESC
LIMIT 100;
```

L7:

```sql
SELECT
  time,
  toString(ip4_0) AS source_ip,
  toString(ip4_1) AS dest_ip,
  concat(toString(if(l3_epc_id_0=-2,1,0)), ' -> ', toString(if(l3_epc_id_1=-2,1,0))) AS internet_direction,
  request_type,
  request_domain,
  request_resource,
  response_code,
  round(response_duration/1000,3) AS response_duration_ms,
  request_length,
  response_length,
  l7_protocol_str,
  toString(agent_id) AS agent_id,
  observation_point,
  l3_epc_id_0,
  l3_epc_id_1
FROM flow_log.l7_flow_log
WHERE time > now() - INTERVAL 30 MINUTE
ORDER BY time DESC
LIMIT 100;
```

Agent mapping:

```sql
SELECT
  toString(v.id) AS agent_id,
  v.name AS agent_name,
  p.device_name AS vm_name,
  p.name AS interface_name,
  p.tap_port
FROM flow_tag.vtap_map AS v
LEFT JOIN flow_tag.vtap_port_map AS p
  ON v.id = p.vtap_id
WHERE p.name != 'lo'
ORDER BY v.id;
```

## Internal vs external classification

DeepFlow internet flags:

```text
0 -> 0 = internal to internal
0 -> 1 = internal to internet/external
1 -> 0 = internet/external to internal
```

DeepFlow usually marks external traffic with:

```text
l3_epc_id = -2
```

VMLens maps IPs to VM inventory first. If an IP cannot be mapped:

```text
internal side unknown     -> unknown IP node
internet/external side    -> external IP node
```

If `DEEPFLOW_MASK_EXTERNAL_IPS=true`, external IP nodes and edge IPs are shown
as stable masked labels.

## Single-VM smoke test

Current available VM:

```text
testing-A-1 10.20.20.130
```

Use any reachable external endpoint to generate traffic:

```bash
ssh -i ~/.vmlens/keys/id_ed25519_vmlens -o IdentitiesOnly=yes ubuntu@10.20.20.130
```

Inside the VM:

```bash
curl -I https://speed.cloudflare.com
curl --fail --show-error -o /dev/null -w 'status=%{http_code} downloaded=%{size_download}\n' 'https://speed.cloudflare.com/__down?bytes=10485760'
```

Then check local:

```bash
curl 'http://127.0.0.1:8080/api/deepflow/raw/flows?time_range=30m&limit=20'
curl 'http://127.0.0.1:8080/api/deepflow/graph?time_range=30m&limit=200'
```

Expected:

```text
10.20.20.130 -> external node
direction = internal_external
bytes/request metrics increase
```

## Known limitations

- DeepFlow sees traffic at the capture point. If NAT/VPN translated the source
  before traffic reached the VM, VMLens sees the translated IP.
- L7 `server_port` is not always present in the DeepFlow L7 query. VMLens uses
  L4 rows in the same window to infer server port when possible.
- L7 deduplication uses tap-side priority plus a short time bucket. It removes
  normal DeepFlow duplicate rows but raw logs remain available for audit.
- Tenant isolation depends on VMLens VM inventory. Keep
  `DEEPFLOW_REQUIRE_INVENTORY_FILTER=true` in shared environments.
