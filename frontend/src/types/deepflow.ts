export interface DeepFlowNode {
  id: string
  type: 'vm' | 'external' | 'unknown' | string
  label: string
  ip?: string
  vm_id?: string
  tenant_id?: string
  status?: string
  role?: string
  masked?: boolean
}

export interface DeepFlowEdge {
  id: string
  source: string
  target: string
  source_vm_id?: string
  dest_vm_id?: string
  source_ip: string
  dest_ip: string
  source_role: string
  dest_role: string
  direction: 'internal_internal' | 'internal_external' | 'external_internal' | string
  protocol: string
  server_port: number
  request_count: number
  error_count: number
  total_bytes: number
  avg_rtt_ms: number
  p95_rtt_ms: number
  avg_response_duration_ms: number
  last_response_code?: number
  last_seen: string
  agent_ids: string[]
  observation_points: string[]
}

export interface DeepFlowTopology {
  nodes: DeepFlowNode[]
  edges: DeepFlowEdge[]
  window: string
  generated_at: string
  source: 'deepflow'
  warnings?: string[]
}

export interface DeepFlowL4Flow {
  time: string
  source_ip: string
  dest_ip: string
  client_port: number
  server_port: number
  protocol: string
  status: string
  byte_tx: number
  byte_rx: number
  total_bytes: number
  rtt_ms: number
  retrans_total: number
  agent_id: string
  l3_epc_id_0: number
  l3_epc_id_1: number
  internet_direction: string
}

export interface DeepFlowL7Request {
  time: string
  source_ip: string
  dest_ip: string
  request_type: string
  request_domain: string
  request_resource: string
  response_code: number
  response_duration_ms: number
  request_length: number
  response_length: number
  l7_protocol_str: string
  agent_id: string
  observation_point: string
  internet_direction: string
}

export interface DeepFlowRawLogs {
  l4: DeepFlowL4Flow[]
  l7: DeepFlowL7Request[]
  window: string
  limit: number
}

export interface DeepFlowHealth {
  enabled: boolean
  clickhouse_reachable: boolean
  querier_reachable: boolean
  controller_reachable: boolean
  agent_list_not_empty: boolean
  latest_l4_timestamp?: string
  latest_l7_timestamp?: string
  warnings?: string[]
  errors?: string[]
  checked_at: string
}
