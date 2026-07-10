ALTER TABLE network_flows
  DROP CONSTRAINT IF EXISTS network_flows_scope_check;

ALTER TABLE network_flows
  ADD CONSTRAINT network_flows_scope_check
  CHECK (scope IN (
    'internal_same_tenant',
    'internal_cross_tenant',
    'unknown_internal',
    'external_public',
    'external_private',
    'unknown'
  ));

ALTER TABLE flow_observations
  DROP CONSTRAINT IF EXISTS flow_observations_scope_check;

ALTER TABLE flow_observations
  ADD CONSTRAINT flow_observations_scope_check
  CHECK (scope IN (
    'internal_same_tenant',
    'internal_cross_tenant',
    'unknown_internal',
    'external_public',
    'external_private',
    'unknown'
  ));

INSERT INTO external_hosts (ip, first_seen, last_seen)
SELECT ip, first_seen, last_seen
FROM unknown_internal_hosts
WHERE resolved_vm_id IS NULL
ON CONFLICT (ip) DO UPDATE
SET last_seen = GREATEST(external_hosts.last_seen, EXCLUDED.last_seen);

UPDATE network_flows
SET scope = 'external_private'
WHERE scope = 'unknown_internal'
  AND dst_vm_id IS NULL;

UPDATE flow_observations
SET scope = 'external_private'
WHERE scope = 'unknown_internal'
  AND dst_vm_id IS NULL;

DELETE FROM unknown_internal_hosts
WHERE resolved_vm_id IS NULL;
