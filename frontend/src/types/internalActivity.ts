import type { FlowScope } from './flow'

export interface InternalActivity {
  id: string
  observer_vm_id: string
  observer_name: string
  observer_ip: string
  peer_vm_id?: string
  peer_name: string
  peer_ip: string
  source_vm_id?: string
  source_name: string
  source_ip: string
  destination_vm_id?: string
  destination_name: string
  destination_ip: string
  protocol: 'tcp' | 'udp' | 'icmp'
  direction: 'ingress' | 'egress'
  scope: FlowScope
  service: string
  service_port: number
  local_port: number
  peer_port: number
  bytes_sent: number
  bytes_received: number
  connection_count: number
  request_count: number
  error_count: number
  requests_per_second: number
  connections_per_second: number
  first_seen: string
  last_seen: string
  observed_at: string
}
