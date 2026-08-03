import type { FlowScope } from './flow'

export type GraphNodeType = 'vm' | 'unknown_internal' | 'external' | 'unknown'

export interface GraphNode {
  id: string
  type: GraphNodeType
  label: string
  ip?: string
  private_ip?: string
  public_ip?: string
  status?: string
  tenant_id?: string
  role?: string
  traffic_in: number
  traffic_out: number
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  protocol: 'tcp' | 'udp' | string
  dst_port: number
  scope: FlowScope
  bytes_sent: number
  bytes_received: number
  packets: number
  connection_count: number
  request_count: number
  retransmission_count?: number
  first_seen: string
  last_seen: string
  last_observed_at: string
  active: boolean
  active_until: string
  failed?: boolean
  failed_until?: string
  weight: number
  kind?: 'traffic' | 'reachability' | string
  reachable?: boolean
  source_ip?: string
  dest_ip?: string
  source_role?: string
  dest_role?: string
  direction?: string
  server_port?: number
  error_count?: number
  last_error_at?: string
  total_bytes?: number
  avg_rtt_ms?: number
  avg_app_delay_ms?: number
  http_2xx_count?: number
  http_4xx_count?: number
  http_5xx_count?: number
  last_http_status?: number
  p95_rtt_ms?: number
  avg_response_duration_ms?: number
  last_response_code?: number
  agent_ids?: string[]
  observation_points?: string[]
}

export type ConnectionHealth = 'healthy' | 'degraded' | 'failed' | 'inactive' | 'unknown'

export interface ConnectionSummary {
  id: string
  source: string
  target: string
  source_label: string
  target_label: string
  source_ip?: string
  target_ip?: string
  direction: 'one_way' | 'two_way'
  active_direction: 'source_to_target' | 'target_to_source' | 'bidirectional' | 'none'
  health: ConnectionHealth
  scope: string
  protocols: string[]
  ports: number[]
  connected: boolean
  active: boolean
  failed: boolean
  slow: boolean
  request_count: number
  error_count: number
  retransmission_count?: number
  total_bytes: number
  packets: number
  connection_count: number
  avg_rtt_ms: number
  p95_rtt_ms: number
  avg_response_duration_ms: number
  last_response_code?: number
  last_seen?: string
  last_observed_at?: string
  agent_ids: string[]
  observation_points: string[]
  recommendation: string
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface GraphFilters {
  vm_id: string
  scope: string
  protocol: string
  port: string
  time_range: string
  min_bytes: string
  status: string
}
