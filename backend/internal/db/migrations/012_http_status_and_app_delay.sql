-- HTTP response status classes and application delay.
--
-- The agent derives the status code in-kernel from a plaintext HTTP/1.x status
-- line and sends only the class counts plus the most recent code. No payload,
-- header, or reason phrase is transmitted or stored. Encrypted traffic reports
-- nothing here, so these columns stay zero for HTTPS.

ALTER TABLE network_flows
  ADD COLUMN IF NOT EXISTS http_1xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_1xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_2xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_2xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_3xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_3xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_4xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_4xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_5xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_5xx_count >= 0),
  ADD COLUMN IF NOT EXISTS last_http_status INTEGER NULL CHECK (last_http_status IS NULL OR (last_http_status BETWEEN 100 AND 599));

ALTER TABLE flow_observations
  ADD COLUMN IF NOT EXISTS http_1xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_1xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_2xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_2xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_3xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_3xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_4xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_4xx_count >= 0),
  ADD COLUMN IF NOT EXISTS http_5xx_count BIGINT NOT NULL DEFAULT 0 CHECK (http_5xx_count >= 0),
  ADD COLUMN IF NOT EXISTS last_http_status INTEGER NULL CHECK (last_http_status IS NULL OR (last_http_status BETWEEN 100 AND 599));

-- Error-rate style questions ("which paths are returning 5xx") scan by recency.
CREATE INDEX IF NOT EXISTS idx_network_flows_http_5xx
  ON network_flows(observed_at DESC)
  WHERE http_5xx_count > 0;
