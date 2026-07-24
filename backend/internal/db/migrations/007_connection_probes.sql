CREATE TABLE IF NOT EXISTS connection_probes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  src_vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  dst_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  src_ip INET NOT NULL,
  dst_ip INET NOT NULL,
  protocol TEXT NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp', 'udp', 'icmp')),
  dst_port INTEGER NOT NULL DEFAULT 18081,
  source TEXT NOT NULL DEFAULT 'vmlens_probe',
  probe_type TEXT NOT NULL DEFAULT 'connectivity_check',
  success BOOLEAN NOT NULL DEFAULT false,
  rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (rtt_ms >= 0),
  error TEXT NULL,
  first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_connection_probes_pair
  ON connection_probes(agent_id, src_vm_id, dst_vm_id, dst_ip, protocol, dst_port) NULLS NOT DISTINCT;

CREATE INDEX IF NOT EXISTS idx_connection_probes_recent
  ON connection_probes(success, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_connection_probes_src_vm
  ON connection_probes(src_vm_id, observed_at DESC);
