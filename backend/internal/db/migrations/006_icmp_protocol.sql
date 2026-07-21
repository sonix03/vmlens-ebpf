ALTER TABLE network_flows
  DROP CONSTRAINT IF EXISTS network_flows_protocol_check;

ALTER TABLE network_flows
  ADD CONSTRAINT network_flows_protocol_check
  CHECK (protocol IN ('tcp', 'udp', 'icmp'));

ALTER TABLE flow_observations
  DROP CONSTRAINT IF EXISTS flow_observations_protocol_check;

ALTER TABLE flow_observations
  ADD CONSTRAINT flow_observations_protocol_check
  CHECK (protocol IN ('tcp', 'udp', 'icmp'));
