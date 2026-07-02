import type { AgentStatus } from './agent'

export interface VMInterface {
  name: string
  ip_address?: string
  mac_address?: string
}

export interface VM {
  id: string
  name: string
  tenant_id?: string
  private_ip?: string
  public_ip?: string
  role?: string
  agent_id?: string
  machine_id?: string
  status: AgentStatus
  interfaces?: VMInterface[]
}

