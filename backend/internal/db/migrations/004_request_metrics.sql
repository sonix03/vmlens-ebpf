ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS request_count BIGINT NOT NULL DEFAULT 0 CHECK (request_count >= 0);

UPDATE network_flows
SET request_count = connection_count
WHERE request_count = 0 AND connection_count > 0;

CREATE TABLE IF NOT EXISTS flow_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_id UUID NOT NULL REFERENCES network_flows(id) ON DELETE CASCADE,
  agent_id TEXT NULL REFERENCES agents(id) ON DELETE SET NULL,
  src_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  dst_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  src_ip INET NOT NULL,
  dst_ip INET NOT NULL,
  src_port INTEGER,
  dst_port INTEGER,
  protocol TEXT NOT NULL CHECK (protocol IN ('tcp', 'udp')),
  direction TEXT NOT NULL CHECK (direction IN ('ingress', 'egress')),
  scope TEXT NOT NULL CHECK (scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal', 'external_public', 'external_private', 'unknown')),
  bytes_sent BIGINT NOT NULL DEFAULT 0 CHECK (bytes_sent >= 0),
  bytes_received BIGINT NOT NULL DEFAULT 0 CHECK (bytes_received >= 0),
  packets BIGINT NOT NULL DEFAULT 0 CHECK (packets >= 0),
  connection_count BIGINT NOT NULL DEFAULT 0 CHECK (connection_count >= 0),
  request_count BIGINT NOT NULL DEFAULT 0 CHECK (request_count >= 0),
  first_seen TIMESTAMPTZ NOT NULL,
  last_seen TIMESTAMPTZ NOT NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_flow_observations_observed_at
  ON flow_observations(observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_flow_observations_scope_observed_at
  ON flow_observations(scope, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_flow_observations_flow_id
  ON flow_observations(flow_id);
