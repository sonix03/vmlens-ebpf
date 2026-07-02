export type AgentStatus = 'online' | 'stale' | 'offline'

export interface Agent {
  id: string
  vm_id?: string
  hostname: string
  machine_id?: string
  os?: string
  kernel?: string
  agent_version?: string
  environment?: string
  status: AgentStatus
  first_seen: string
  last_seen: string
}

