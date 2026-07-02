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
  protocol: 'tcp' | 'udp'
  dst_port: number
  scope: FlowScope
  bytes_sent: number
  bytes_received: number
  packets: number
  connection_count: number
  first_seen: string
  last_seen: string
  weight: number
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

