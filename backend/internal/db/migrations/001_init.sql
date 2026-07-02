CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  vm_id TEXT NULL,
  hostname TEXT NOT NULL,
  machine_id TEXT NULL,
  os TEXT NULL,
  kernel TEXT NULL,
  agent_version TEXT NULL,
  environment TEXT NULL,
  status TEXT NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'stale', 'offline')),
  first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vms (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  tenant_id TEXT NULL,
  private_ip INET NULL,
  public_ip INET NULL,
  mac_address TEXT NULL,
  host_id TEXT NULL,
  role TEXT NULL,
  discovered_by TEXT NOT NULL DEFAULT 'agent',
  agent_id TEXT NULL,
  machine_id TEXT NULL,
  status TEXT NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'stale', 'offline')),
  first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vm_interfaces (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  vm_id TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
  interface_name TEXT NOT NULL,
  ip_address INET NULL,
  mac_address TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS network_flows (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id TEXT NULL REFERENCES agents(id) ON DELETE SET NULL,
  src_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  dst_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL,
  src_ip INET NOT NULL,
  dst_ip INET NOT NULL,
  src_port INTEGER,
  dst_port INTEGER,
  protocol TEXT NOT NULL CHECK (protocol IN ('tcp', 'udp')),
  direction TEXT NOT NULL DEFAULT 'egress' CHECK (direction IN ('ingress', 'egress')),
  scope TEXT NOT NULL CHECK (scope IN ('internal_same_tenant', 'internal_cross_tenant', 'unknown_internal', 'external_public', 'unknown')),
  bytes_sent BIGINT NOT NULL DEFAULT 0 CHECK (bytes_sent >= 0),
  bytes_received BIGINT NOT NULL DEFAULT 0 CHECK (bytes_received >= 0),
  packets BIGINT NOT NULL DEFAULT 0 CHECK (packets >= 0),
  connection_count BIGINT NOT NULL DEFAULT 0 CHECK (connection_count >= 0),
  first_seen TIMESTAMPTZ NOT NULL,
  last_seen TIMESTAMPTZ NOT NULL,
  interface_name TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS external_hosts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ip INET NOT NULL UNIQUE,
  domain TEXT NULL,
  asn TEXT NULL,
  country TEXT NULL,
  provider TEXT NULL,
  first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS unknown_internal_hosts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ip INET NOT NULL UNIQUE,
  first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_vm_id TEXT NULL REFERENCES vms(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_vm_interfaces_identity
  ON vm_interfaces (vm_id, interface_name, ip_address, mac_address) NULLS NOT DISTINCT;
