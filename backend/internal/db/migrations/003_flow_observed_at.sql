ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_network_flows_observed_at
  ON network_flows(observed_at DESC);
