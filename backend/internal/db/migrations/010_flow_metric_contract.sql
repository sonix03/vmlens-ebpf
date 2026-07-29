ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS retransmission_count BIGINT NOT NULL DEFAULT 0 CHECK (retransmission_count >= 0),
  ADD COLUMN IF NOT EXISTS avg_rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (avg_rtt_ms >= 0),
  ADD COLUMN IF NOT EXISTS avg_app_delay_ms DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (avg_app_delay_ms >= 0);

ALTER TABLE flow_observations
  ADD COLUMN IF NOT EXISTS retransmission_count BIGINT NOT NULL DEFAULT 0 CHECK (retransmission_count >= 0),
  ADD COLUMN IF NOT EXISTS avg_rtt_ms DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (avg_rtt_ms >= 0),
  ADD COLUMN IF NOT EXISTS avg_app_delay_ms DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (avg_app_delay_ms >= 0);
