export interface Summary {
  total_vms: number
  online_vms: number
  stale_vms: number
  offline_vms: number
  total_flows: number
  internal_flows: number
  external_flows: number
  internal_bytes: number
  internal_sent_bytes: number
  internal_received_bytes: number
  external_bytes: number
  external_sent_bytes: number
  external_received_bytes: number
  unknown_internal_hosts: number
  updated_at: string
}

export interface TopTalker {
  vm_id: string
  name: string
  bytes_sent: number
  bytes_received: number
  total_bytes: number
}
