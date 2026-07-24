import type { FlowScope } from './flow'

export type GraphNodeType = 'vm' | 'unknown_internal' | 'external' | 'unknown'

export interface GraphNode {
  id: string
  type: GraphNodeType
  label: string
  ip?: string
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
  p95_rtt_ms?: number
  avg_response_duration_ms?: number
  last_response_code?: number
  agent_ids?: string[]
  observation_points?: string[]
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
