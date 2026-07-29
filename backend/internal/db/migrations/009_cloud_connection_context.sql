ALTER TABLE vms
  ADD COLUMN IF NOT EXISTS project_id TEXT NULL,
  ADD COLUMN IF NOT EXISTS region TEXT NULL,
  ADD COLUMN IF NOT EXISTS zone TEXT NULL,
  ADD COLUMN IF NOT EXISTS environment TEXT NULL,
  ADD COLUMN IF NOT EXISTS owner TEXT NULL,
  ADD COLUMN IF NOT EXISTS network_id TEXT NULL,
  ADD COLUMN IF NOT EXISTS subnet_id TEXT NULL;

ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS project_id TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_vms_ownership
  ON vms(tenant_id, project_id, status, last_seen DESC);

CREATE TABLE IF NOT EXISTS cloud_public_ips (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  ip_address INET NOT NULL,
  provider_id TEXT NULL,
  status TEXT NOT NULL DEFAULT 'assigned' CHECK (status IN ('assigned', 'available', 'released', 'unknown')),
  assigned_at TIMESTAMPTZ NULL,
  last_seen_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_cloud_public_ips_provider
  ON cloud_public_ips(provider_id) WHERE provider_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cloud_public_ips_ownership
  ON cloud_public_ips(tenant_id, project_id, vm_id);

CREATE TABLE IF NOT EXISTS cloud_firewall_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  provider_id TEXT NULL,
  name TEXT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('ingress', 'egress')),
  protocol TEXT NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp', 'udp', 'icmp', 'any')),
  port_min INTEGER NULL CHECK (port_min IS NULL OR (port_min >= 1 AND port_min <= 65535)),
  port_max INTEGER NULL CHECK (port_max IS NULL OR (port_max >= 1 AND port_max <= 65535)),
  source_cidr CIDR NULL,
  dest_cidr CIDR NULL,
  source_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  dest_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  action TEXT NOT NULL DEFAULT 'allow' CHECK (action IN ('allow', 'deny', 'reject')),
  scope TEXT NOT NULL DEFAULT 'private' CHECK (scope IN ('private', 'public', 'management', 'unknown')),
  description TEXT NULL,
  last_synced_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_cloud_firewall_rules_provider
  ON cloud_firewall_rules(provider_id) WHERE provider_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cloud_firewall_rules_ownership
  ON cloud_firewall_rules(tenant_id, project_id, source_vm_id, dest_vm_id);

CREATE TABLE IF NOT EXISTS cloud_routes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  provider_id TEXT NULL,
  network_id TEXT NULL,
  subnet_id TEXT NULL,
  destination CIDR NOT NULL,
  next_hop INET NULL,
  route_type TEXT NOT NULL DEFAULT 'private' CHECK (route_type IN ('private', 'public', 'default', 'peering', 'vpn', 'unknown')),
  description TEXT NULL,
  last_synced_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_cloud_routes_provider
  ON cloud_routes(provider_id) WHERE provider_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cloud_routes_ownership
  ON cloud_routes(tenant_id, project_id, network_id, subnet_id);

CREATE TABLE IF NOT EXISTS cloud_network_policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  name TEXT NOT NULL,
  zone TEXT NULL,
  source_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  dest_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  protocol TEXT NULL CHECK (protocol IS NULL OR protocol IN ('tcp', 'udp', 'icmp', 'any')),
  port INTEGER NULL CHECK (port IS NULL OR (port >= 1 AND port <= 65535)),
  action TEXT NOT NULL DEFAULT 'allow' CHECK (action IN ('allow', 'deny', 'reject')),
  description TEXT NULL,
  last_synced_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cloud_network_policies_ownership
  ON cloud_network_policies(tenant_id, project_id, source_vm_id, dest_vm_id);

CREATE TABLE IF NOT EXISTS connection_intents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  source_vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  dest_vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  protocol TEXT NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp', 'udp', 'icmp', 'any')),
  port INTEGER NOT NULL DEFAULT 0 CHECK (port >= 0 AND port <= 65535),
  purpose TEXT NULL,
  exposure TEXT NOT NULL DEFAULT 'private' CHECK (exposure IN ('private', 'public', 'management', 'unknown')),
  required BOOLEAN NOT NULL DEFAULT true,
  status TEXT NOT NULL DEFAULT 'intended' CHECK (status IN ('intended', 'validated', 'inactive', 'deprecated', 'blocked')),
  created_by TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_connection_intents_identity
  ON connection_intents(source_vm_id, dest_vm_id, protocol, port, exposure);

CREATE INDEX IF NOT EXISTS idx_connection_intents_ownership
  ON connection_intents(tenant_id, project_id, source_vm_id, dest_vm_id);

CREATE TABLE IF NOT EXISTS connection_configurations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  intent_id UUID NULL REFERENCES connection_intents(id) ON DELETE SET NULL,
  source_vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  dest_vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  protocol TEXT NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp', 'udp', 'icmp', 'any')),
  port INTEGER NOT NULL DEFAULT 0 CHECK (port >= 0 AND port <= 65535),
  firewall_rule_id UUID NULL REFERENCES cloud_firewall_rules(id) ON DELETE SET NULL,
  route_id UUID NULL REFERENCES cloud_routes(id) ON DELETE SET NULL,
  network_id TEXT NULL,
  config_state TEXT NOT NULL DEFAULT 'unknown' CHECK (config_state IN ('unknown', 'allowed', 'denied', 'blocked', 'missing', 'stale')),
  security_state TEXT NOT NULL DEFAULT 'unknown' CHECK (security_state IN ('unknown', 'private', 'public', 'management', 'overexposed', 'blocked')),
  last_synced_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_connection_configurations_identity
  ON connection_configurations(source_vm_id, dest_vm_id, protocol, port, COALESCE(network_id, ''));

CREATE INDEX IF NOT EXISTS idx_connection_configurations_ownership
  ON connection_configurations(tenant_id, project_id, source_vm_id, dest_vm_id);

CREATE TABLE IF NOT EXISTS connection_change_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NULL,
  project_id TEXT NULL,
  connection_id UUID NULL,
  change_type TEXT NOT NULL,
  actor TEXT NULL,
  before_state JSONB NULL,
  after_state JSONB NULL,
  affected_source_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  affected_dest_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  validation_state TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connection_change_events_ownership
  ON connection_change_events(tenant_id, project_id, created_at DESC);
