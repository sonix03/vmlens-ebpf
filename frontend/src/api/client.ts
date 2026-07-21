import type { Agent } from '../types/agent'
import type { DeepFlowHealth, DeepFlowRawLogs, DeepFlowTopology } from '../types/deepflow'
import type { Flow } from '../types/flow'
import type { GraphData, GraphFilters } from '../types/graph'
import type { InternalActivity } from '../types/internalActivity'
import type { Summary, TopTalker } from '../types/stats'
import type { VM } from '../types/vm'

export const API_URL = (import.meta.env.VITE_API_URL as string | undefined)?.replace(/\/$/, '') || 'http://localhost:8080'

async function get<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, { cache: 'no-store' })
  if (!response.ok) {
    const message = await response.text()
    throw new Error(`${response.status} ${response.statusText}: ${message}`)
  }
  return response.json() as Promise<T>
}

function graphQuery(filters: GraphFilters): string {
  const query = new URLSearchParams()
  Object.entries(filters).forEach(([key, value]) => {
    if (value) query.set(key, value)
  })
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ''
}

export const api = {
  graph: (filters: GraphFilters) => get<GraphData>(`/api/graph${graphQuery(filters)}`),
  deepFlowGraph: (filters: GraphFilters) => get<DeepFlowTopology>(`/api/deepflow/graph${graphQuery(filters)}`),
  deepFlowRaw: (filters: GraphFilters) => get<DeepFlowRawLogs>(`/api/deepflow/raw/flows${graphQuery({ ...filters, min_bytes: '', status: '' })}`),
  deepFlowHealth: () => get<DeepFlowHealth>('/api/deepflow/health'),
  summary: () => get<Summary>('/api/stats/summary'),
  topTalkers: () => get<TopTalker[]>('/api/stats/top-talkers?limit=10'),
  agents: () => get<Agent[]>('/api/agents'),
  vms: () => get<VM[]>('/api/vms'),
  flows: () => get<Flow[]>('/api/flows'),
  internalActivity: () => get<InternalActivity[]>('/api/internal/activity?limit=100'),
}
