import type { Flow } from '../types/flow'

export const topologyNoisePorts = new Set([22, 53, 123, 8080, 18080, 18081, 18082])

export function isNoiseFlow(item: Flow) {
  return topologyNoisePorts.has(item.src_port) || topologyNoisePorts.has(item.dst_port)
}

export function isConnectionFlow(item: Flow) {
  if (isNoiseFlow(item)) return false
  return (item.connection_count ?? 0) > 0 || (item.error_count ?? 0) > 0
}

export function httpStatusCount(item: Flow) {
  return (item.http_1xx_count ?? 0) + (item.http_2xx_count ?? 0) + (item.http_3xx_count ?? 0)
    + (item.http_4xx_count ?? 0) + (item.http_5xx_count ?? 0)
}

export function isRequestFlow(item: Flow) {
  if (isNoiseFlow(item)) return false
  // An observed HTTP response is the strongest request evidence there is. It
  // arrives on the client's ingress row, which carries no connection or request
  // counter of its own, so without this the response rows never reach this view.
  return (item.request_count ?? 0) > 0 || (item.error_count ?? 0) > 0 || httpStatusCount(item) > 0
}

// Rows that actually carried an HTTP response. Most flow buckets never do, so
// this is what makes a readable status log rather than a column of dashes.
export function isHTTPFlow(item: Flow) {
  if (isNoiseFlow(item)) return false
  return httpStatusCount(item) > 0
}
