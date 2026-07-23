import type { DeepFlowL4Flow, DeepFlowL7Request } from '../types/deepflow'

export const topologyNoisePorts = new Set([22, 53, 123, 8080, 18080, 18081, 20033, 20035, 30033, 30035])

const requestNoiseProtocols = new Set(['dns'])

export function isDeepFlowConnectionFlow(item: DeepFlowL4Flow) {
  if (!item.protocol || item.protocol === '0') return false
  if (topologyNoisePorts.has(item.client_port) || topologyNoisePorts.has(item.server_port)) return false
  return true
}

export function isDeepFlowRequestFlow(item: DeepFlowL7Request) {
  const protocol = item.l7_protocol_str.toLowerCase()
  if (requestNoiseProtocols.has(protocol)) return false
  const domainParts = item.request_domain.split(':')
  if (topologyNoisePorts.has(Number(domainParts[domainParts.length - 1]))) return false
  return true
}
