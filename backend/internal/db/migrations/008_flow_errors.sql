ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS error_count BIGINT NOT NULL DEFAULT 0 CHECK (error_count >= 0);

ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ NULL;

ALTER TABLE flow_observations
  ADD COLUMN IF NOT EXISTS error_count BIGINT NOT NULL DEFAULT 0 CHECK (error_count >= 0);

CREATE INDEX IF NOT EXISTS idx_network_flows_last_error_at
  ON network_flows(last_error_at DESC)
  WHERE last_error_at IS NOT NULL;
