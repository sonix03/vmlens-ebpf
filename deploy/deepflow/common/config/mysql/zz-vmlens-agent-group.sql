USE deepflow;

SET @vmlens_default_group_lcuuid := (
  SELECT lcuuid
  FROM vtap_group
  WHERE name = 'default'
  LIMIT 1
);

INSERT INTO vtap_group_configuration (
  user_id,
  team_id,
  vtap_group_lcuuid,
  system_load_circuit_breaker_threshold,
  system_load_circuit_breaker_recover,
  system_load_circuit_breaker_metric,
  lcuuid
)
SELECT
  1,
  1,
  @vmlens_default_group_lcuuid,
  10.00,
  7.00,
  'load15',
  UUID()
WHERE @vmlens_default_group_lcuuid IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM vtap_group_configuration
    WHERE vtap_group_lcuuid = @vmlens_default_group_lcuuid
  );

UPDATE vtap_group_configuration
SET
  system_load_circuit_breaker_threshold = 10.00,
  system_load_circuit_breaker_recover = 7.00,
  system_load_circuit_breaker_metric = 'load15'
WHERE vtap_group_lcuuid = @vmlens_default_group_lcuuid;
