export type FlowScope =
  | 'internal_same_tenant'
  | 'internal_cross_tenant'
  | 'unknown_internal'
  | 'external_public'
  | 'unknown'

export interface Flow {
  id: string
  agent_id?: string
  src_vm_id?: string
  dst_vm_id?: string
  src_ip: string
  dst_ip: string
  src_port: number
  dst_port: number
  protocol: 'tcp' | 'udp'
  direction: 'ingress' | 'egress'
  scope: FlowScope
  bytes_sent: number
  bytes_received: number
  packets: number
  connection_count: number
  request_count: number
  requests_per_second?: number
  connections_per_second?: number
  first_seen: string
  last_seen: string
  observed_at?: string
}
