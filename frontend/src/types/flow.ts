export type FlowScope =
  | 'internal_same_tenant'
  | 'internal_cross_tenant'
  | 'unknown_internal'
  | 'external_public'
  | 'external_private'
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
  protocol: 'tcp' | 'udp' | 'icmp'
  direction: 'ingress' | 'egress'
  scope: FlowScope
  service?: string
  service_port?: number
  bytes_sent: number
  bytes_received: number
  packets: number
  connection_count: number
  request_count: number
  error_count?: number
  retransmission_count?: number
  avg_rtt_ms?: number
  avg_app_delay_ms?: number
  http_1xx_count?: number
  http_2xx_count?: number
  http_3xx_count?: number
  http_4xx_count?: number
  http_5xx_count?: number
  last_http_status?: number
  requests_per_second?: number
  connections_per_second?: number
  first_seen: string
  last_seen: string
  observed_at?: string
  last_error_at?: string
  interface_name?: string
}
