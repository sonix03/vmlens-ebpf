-- Semantic comments for the generated schema documentation.
--
-- Applied only to the throwaway database used by scripts/generate-schema-dbml.sh,
-- not to a running deployment. They become Note: entries in the generated DBML,
-- so the dbdiagram.io view carries the meaning and not just the column types.
--
-- The full semantics live in contracts/data-schema.md. Keep these in sync with
-- that file; it is the prose version of the same contract.

COMMENT ON TABLE agents IS
  'One row per installed agent process. id is chosen by the agent, not the backend. Written by /api/agents/register and /heartbeat. status is derived by the sweep, never sent by the agent.';
COMMENT ON COLUMN agents.id IS 'Agent-supplied identifier. Upsert key.';
COMMENT ON COLUMN agents.machine_id IS 'Strongest re-identification key across agent reinstalls.';
COMMENT ON COLUMN agents.status IS 'Derived by the status sweep: online <1m, stale <5m, else offline.';
COMMENT ON COLUMN agents.last_seen IS 'Bumped by heartbeat AND by every accepted flow ingest.';

COMMENT ON TABLE vms IS
  'One row per known VM. Created by agent registration (discovered_by=agent) and enriched from configs/vms.local by IP match. Inventory values win over self-reported ones.';
COMMENT ON COLUMN vms.tenant_id IS 'Drives internal_same_tenant vs internal_cross_tenant. A NULL tenant can never produce a same-tenant flow.';
COMMENT ON COLUMN vms.private_ip IS 'Primary observed address. Used to match the inventory entry.';
COMMENT ON COLUMN vms.host_type IS 'Inventory-assigned host classification.';
COMMENT ON COLUMN vms.discovered_by IS 'Set to agent for self-registered VMs. Only those are eligible for VM_DELETE_AFTER reaping.';
COMMENT ON COLUMN vms.status IS 'Derived by the status sweep, same windows as agents.status.';

COMMENT ON TABLE vm_interfaces IS
  'Every NIC an agent reported. This is the lookup table that turns a peer IP into a VM ID, so an unreported interface makes that path look external. Unique on (vm_id, interface_name, ip_address, mac_address) NULLS NOT DISTINCT.';

COMMENT ON TABLE network_flows IS
  'Aggregate flow state, one row per flow bucket, updated in place. Counters here are CUMULATIVE since first_seen, while the agent posts window deltas. No unique index: the bucket is found by SELECT ... FOR UPDATE on (src_vm_id, dst_vm_id, src_ip, dst_ip, protocol, dst_port, scope, direction) under a pg_advisory_xact_lock. Rows are never deleted.';
COMMENT ON COLUMN network_flows.src_ip IS 'Always the observing VM, never simply the packet header source.';
COMMENT ON COLUMN network_flows.dst_ip IS 'The peer.';
COMMENT ON COLUMN network_flows.src_port IS 'First-seen only. NOT part of the bucket key and not meaningful for display.';
COMMENT ON COLUMN network_flows.dst_port IS 'The peer port. On an ingress flow this is the client ephemeral port; resolve service_port for display.';
COMMENT ON COLUMN network_flows.direction IS 'Which way the bytes moved relative to the observing NIC.';
COMMENT ON COLUMN network_flows.scope IS 'Part of the bucket key, so a peer that later registers as a VM starts a NEW bucket.';
COMMENT ON COLUMN network_flows.packets IS 'TC path only. A kprobe-only flow has bytes and connections but zero packets.';
COMMENT ON COLUMN network_flows.avg_rtt_ms IS 'Kernel TCP RTT, smoothed as (old+new)/2. 0 means no sample, not 0 ms.';
COMMENT ON COLUMN network_flows.avg_app_delay_ms IS 'Time to first response byte, same smoothing. 0 means no sample.';
COMMENT ON COLUMN network_flows.error_count IS 'TCP RST count, cumulative.';
COMMENT ON COLUMN network_flows.last_error_at IS 'NULL until the first non-zero error window arrives.';
COMMENT ON COLUMN network_flows.http_5xx_count IS 'Always 0 for TLS, which is not the same as no errors.';
COMMENT ON COLUMN network_flows.last_http_status IS 'Plaintext HTTP/1.x only, derived in-kernel from the status line. No payload is stored.';
COMMENT ON COLUMN network_flows.observed_at IS 'When the control plane last accepted an update. Use this for freshness.';

COMMENT ON TABLE flow_observations IS
  'Append-only event log, one row per accepted ingest request. Counters are the WINDOW DELTA exactly as posted and latency is the raw window average, the opposite semantics of network_flows. Use this for rates and time series. Nothing deletes from this table: it grows one row per bucket per FLOW_INTERVAL (2s) per agent, forever.';
COMMENT ON COLUMN flow_observations.flow_id IS 'The bucket this window was folded into. CASCADE on delete.';
COMMENT ON COLUMN flow_observations.observed_at IS 'When the request was accepted. The only time column safe for rate windows.';

COMMENT ON TABLE external_hosts IS
  'Peer registry for public addresses and out-of-inventory private ones. Written on ingest, unique on ip.';
COMMENT ON COLUMN external_hosts.domain IS 'Enrichment column. Nothing populates it today.';
COMMENT ON COLUMN external_hosts.asn IS 'Enrichment column. Nothing populates it today.';
COMMENT ON COLUMN external_hosts.country IS 'Enrichment column. Nothing populates it today.';
COMMENT ON COLUMN external_hosts.provider IS 'Enrichment column. Nothing populates it today.';

COMMENT ON TABLE unknown_internal_hosts IS
  'Private peers inside INTERNAL_CIDRS that are not registered VMs. When an agent later registers with that IP the row gains resolved_vm_id and the affected flows are re-pointed at the VM.';

COMMENT ON TABLE connection_probes IS
  'Active probe results: CURRENT STATE, NOT HISTORY. The unique index on (agent_id, src_vm_id, dst_vm_id, dst_ip, protocol, dst_port) makes each new result overwrite the previous one, so a flapping path is indistinguishable from a stable one. Feeds graph edges with kind=reachability, which must never be drawn as workload traffic.';
COMMENT ON COLUMN connection_probes.rtt_ms IS 'Probe wall-clock elapsed time. NOT the kernel TCP RTT in network_flows.avg_rtt_ms.';
COMMENT ON COLUMN connection_probes.success IS 'Reachable at the probe layer only.';

COMMENT ON TABLE cloud_public_ips IS
  'DECLARED ONLY: read by /api/cloud/context, written by nothing. The cloud provider is wired to a noop.';
COMMENT ON TABLE cloud_firewall_rules IS
  'DECLARED ONLY: read by /api/cloud/context, written by nothing. Empty does not mean nothing is configured, it means nothing has been synced.';
COMMENT ON TABLE cloud_routes IS
  'DECLARED ONLY: read by /api/cloud/context, written by nothing.';
COMMENT ON TABLE cloud_network_policies IS
  'DECLARED ONLY: read by /api/cloud/context, written by nothing.';
COMMENT ON TABLE connection_intents IS
  'DECLARED ONLY: read by the connection state service, written by nothing. Intended to hold what a path is supposed to be.';
COMMENT ON TABLE connection_configurations IS
  'DECLARED ONLY: read by the connection state service, written by nothing. Intended to link an intent to a firewall rule and a route.';
COMMENT ON TABLE connection_change_events IS
  'DECLARED ONLY: no reader and no writer anywhere in the codebase.';
