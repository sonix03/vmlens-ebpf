import type { Agent } from '../types/agent'
import type { DeepFlowHealth, DeepFlowRawLogs, DeepFlowTopology } from '../types/deepflow'
import type { Flow } from '../types/flow'
import type { GraphData, GraphFilters } from '../types/graph'
import type { InternalActivity } from '../types/internalActivity'
import type { Summary, TopTalker } from '../types/stats'
import type { VM } from '../types/vm'

export const API_URL = ((import.meta.env.VITE_API_URL as string | undefined) ?? '').replace(/\/$/, '')
export const GRAFANA_URL = ((import.meta.env.VITE_GRAFANA_URL as string | undefined) ?? 'http://localhost:3001').replace(/\/$/, '')
export const GRAFANA_L4_URL = `${GRAFANA_URL}/d/VMLens_Network_Flow_Log_Live/vmlens-live-network-flow-log?orgId=1&from=now-1h&to=now&refresh=5s`
export const GRAFANA_L7_URL = `${GRAFANA_URL}/d/VMLens_Request_Log_Live/vmlens-live-request-log?orgId=1&from=now-1h&to=now&refresh=5s`
export const GRAFANA_NETWORK_HOST_URL = `${GRAFANA_URL}/d/VMLens_Network_Cloud_Host_Live/vmlens-live-network-cloud-host?orgId=1&from=now-15m&to=now&refresh=5s`
export const GRAFANA_APPLICATION_HOST_URL = `${GRAFANA_URL}/d/VMLens_Application_Cloud_Host_Live/vmlens-live-application-cloud-host?orgId=1&from=now-15m&to=now&refresh=5s`

async function get<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, { cache: 'no-store' })
  if (!response.ok) {
    const message = await response.text()
    throw new Error(`${response.status} ${response.statusText}: ${message}`)
  }
  return response.json() as Promise<T>
}

type QueryParams = Partial<GraphFilters> & { limit?: string }

function graphQuery(filters: QueryParams): string {
  const query = new URLSearchParams()
  Object.entries(filters as Record<string, string | undefined>).forEach(([key, value]) => {
    if (value) query.set(key, value)
  })
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ''
}

export const api = {
  graph: (filters: GraphFilters) => get<GraphData>(`/api/graph${graphQuery(filters)}`),
  deepFlowGraph: (filters: GraphFilters) => get<DeepFlowTopology>(`/api/deepflow/graph${graphQuery(filters)}`),
  deepFlowRaw: (filters: GraphFilters, limit = 500) => get<DeepFlowRawLogs>(`/api/deepflow/raw/flows${graphQuery({ ...filters, min_bytes: '', status: '', limit: String(limit) })}`),
  deepFlowHealth: () => get<DeepFlowHealth>('/api/deepflow/health'),
  summary: () => get<Summary>('/api/stats/summary'),
  topTalkers: () => get<TopTalker[]>('/api/stats/top-talkers?limit=10'),
  agents: () => get<Agent[]>('/api/agents'),
  vms: () => get<VM[]>('/api/vms'),
  flows: () => get<Flow[]>('/api/flows'),
  internalActivity: (limit = 200, timeRange = '5m') => get<InternalActivity[]>(`/api/internal/activity?limit=${limit}&time_range=${encodeURIComponent(timeRange)}`),
}
