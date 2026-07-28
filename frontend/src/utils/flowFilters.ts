import type { Flow } from '../types/flow'

export const topologyNoisePorts = new Set([22, 53, 123, 8080, 18080, 18081, 18082])

export function isNoiseFlow(item: Flow) {
  return topologyNoisePorts.has(item.src_port) || topologyNoisePorts.has(item.dst_port)
}

export function isConnectionFlow(item: Flow) {
  if (isNoiseFlow(item)) return false
  return (item.connection_count ?? 0) > 0 || (item.error_count ?? 0) > 0
}

export function isRequestFlow(item: Flow) {
  if (isNoiseFlow(item)) return false
  return (item.request_count ?? 0) > 0 || (item.error_count ?? 0) > 0
}
