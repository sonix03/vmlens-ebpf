import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, GRAFANA_APPLICATION_HOST_URL, GRAFANA_L4_URL, GRAFANA_L7_URL, GRAFANA_NETWORK_HOST_URL } from './api/client'
import { connectRealtime } from './api/realtime'
import { DeepFlowFlowTable, type DeepFlowTableMode } from './components/DeepFlowFlowTable'
import { GraphView } from './components/GraphView'
import { InternalActivityTable } from './components/InternalActivityTable'
import { NodeDetailsPanel } from './components/NodeDetailsPanel'
import { StatCards } from './components/StatCards'
import type { Flow } from './types/flow'
import type { DeepFlowHealth, DeepFlowRawLogs } from './types/deepflow'
import type { GraphData, GraphEdge, GraphFilters, GraphNode } from './types/graph'
import type { InternalActivity } from './types/internalActivity'
import type { Summary } from './types/stats'
import type { VM } from './types/vm'
import { isDeepFlowConnectionFlow, isDeepFlowRequestFlow } from './utils/flowFilters'

const graphWindow: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '15m', min_bytes: '', status: '',
}

const deepFlowLogWindow: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '1h', min_bytes: '', status: '',
}

const internalActivityWindow = '5m'
const internalActivityLimit = 200
const deepFlowLogLimit = 500

const graphWindowLabel = graphWindow.time_range
const deepFlowWindowLabel = deepFlowLogWindow.time_range

const activeWindowMs = 4000
const canonicalRefreshDelayMs = 1000
const tablePulseMs = 900
type ActivityView = 'internal' | DeepFlowTableMode
const graphExcludedPorts = new Set(
  ((import.meta.env.VITE_GRAPH_EXCLUDED_PORTS as string | undefined) ?? '22,53,123,8080,18080,18081,20033,20035,30033,30035')
    .split(',')
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item)),
)
const graphExcludedIPs = new Set(
  ((import.meta.env.VITE_GRAPH_EXCLUDED_IPS as string | undefined) ?? '10.20.20.125,127.0.0.1')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean),
)

function isFlow(value: unknown): value is Flow {
  if (!value || typeof value !== 'object') return false
  const flow = value as Partial<Flow>
  return typeof flow.src_ip === 'string'
    && typeof flow.dst_ip === 'string'
    && (flow.protocol === 'tcp' || flow.protocol === 'udp' || flow.protocol === 'icmp')
    && typeof flow.dst_port === 'number'
    && typeof flow.bytes_sent === 'number'
    && typeof flow.bytes_received === 'number'
}

function applyLiveSummary(summary: Summary | undefined, flow: Flow, observedAt: string): Summary | undefined {
  if (!summary) return summary
  const bytes = flow.bytes_sent + flow.bytes_received
  const requestCount = flow.request_count ?? 0
  const requestPatch = {
    network_requests_total: (summary.network_requests_total ?? 0) + requestCount,
    network_requests_last_minute: (summary.network_requests_last_minute ?? 0) + requestCount,
    network_requests_per_second: Math.max(summary.network_requests_per_second ?? 0, flow.requests_per_second ?? 0),
    network_connections_per_second: Math.max(summary.network_connections_per_second ?? 0, flow.connections_per_second ?? 0),
  }
  if (flow.scope === 'internal_same_tenant' || flow.scope === 'internal_cross_tenant' || flow.scope === 'unknown_internal') {
    return {
      ...summary,
      ...requestPatch,
      internal_bytes: summary.internal_bytes + bytes,
      internal_sent_bytes: summary.internal_sent_bytes + flow.bytes_sent,
      internal_received_bytes: summary.internal_received_bytes + flow.bytes_received,
      updated_at: observedAt,
    }
  }
  if (flow.scope === 'external_public' || flow.scope === 'external_private') {
    return {
      ...summary,
      ...requestPatch,
      external_bytes: summary.external_bytes + bytes,
      external_sent_bytes: summary.external_sent_bytes + flow.bytes_sent,
      external_received_bytes: summary.external_received_bytes + flow.bytes_received,
      updated_at: observedAt,
    }
  }
  return { ...summary, ...requestPatch, updated_at: observedAt }
}

function edgeWeight(bytes: number): number {
  if (bytes >= 100 * 1024 * 1024) return 5
  if (bytes >= 10 * 1024 * 1024) return 4
  if (bytes >= 1024 * 1024) return 3
  if (bytes >= 100 * 1024) return 2
  return 1
}

function nodeSafe(value: string): string {
  return value.replaceAll(':', '_').replaceAll('/', '_').replaceAll('%', '_')
}

function vmToGraphNode(vm: VM): GraphNode {
  return {
    id: vm.id,
    type: 'vm',
    label: vm.name,
    ip: vm.private_ip || vm.public_ip || vm.interfaces?.find((item) => item.ip_address)?.ip_address,
    status: vm.status,
    tenant_id: vm.tenant_id,
    role: vm.role,
    traffic_in: 0,
    traffic_out: 0,
  }
}

function liveExternalTarget(flow: Flow): GraphNode | undefined {
  if (flow.dst_vm_id) return undefined
  if (flow.scope === 'external_public' || flow.scope === 'external_private') {
    return {
      id: `external-${nodeSafe(flow.dst_ip)}`,
      type: 'external',
      label: flow.dst_ip,
      ip: flow.dst_ip,
      status: 'external',
      traffic_in: 0,
      traffic_out: 0,
    }
  }
  if (flow.scope === 'unknown_internal') {
    return {
      id: `unknown-internal-${nodeSafe(flow.dst_ip)}`,
      type: 'unknown_internal',
      label: flow.dst_ip,
      ip: flow.dst_ip,
      status: 'unknown',
      traffic_in: 0,
      traffic_out: 0,
    }
  }
  return undefined
}

function mergeVMInventory(graph: GraphData, inventory: GraphNode[]): GraphData {
  if (inventory.length === 0) return graph
  const nodesByID = new Map(graph.nodes.map((node) => [node.id, node]))
  inventory.forEach((vmNode) => {
    const existing = nodesByID.get(vmNode.id)
    if (existing) {
      nodesByID.set(vmNode.id, {
        ...vmNode,
        ...existing,
        label: existing.label || vmNode.label,
        ip: existing.ip || vmNode.ip,
        status: existing.status || vmNode.status,
        tenant_id: existing.tenant_id || vmNode.tenant_id,
        role: existing.role || vmNode.role,
      })
      return
    }
    nodesByID.set(vmNode.id, vmNode)
  })

  return {
    ...graph,
    nodes: Array.from(nodesByID.values()),
  }
}

function mergeGraphData(current: GraphData, next: GraphData): GraphData {
  const currentVMs = current.nodes.filter((node) => node.type === 'vm')
  const nextVMs = next.nodes.filter((node) => node.type === 'vm')
  if (currentVMs.length > 0 && nextVMs.length === 0) return current

  const nextNodeIDs = new Set(next.nodes.map((node) => node.id))
  const preservedVMs = currentVMs.filter((node) => !nextNodeIDs.has(node.id))
  if (preservedVMs.length === 0) return next

  return {
    nodes: [...next.nodes, ...preservedVMs],
    edges: next.edges,
  }
}

function vmTopologyOnly(graph: GraphData): GraphData {
  const vmIDs = new Set(graph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  return {
    nodes: graph.nodes.filter((node) => vmIDs.has(node.id)),
    edges: graph.edges.filter((edge) => vmIDs.has(edge.source) && vmIDs.has(edge.target) && edge.source !== edge.target),
  }
}

function applyLiveFlow(graph: GraphData, flow: Flow, observedAt: string): GraphData {
  if (graphExcludedPorts.has(flow.src_port) || graphExcludedPorts.has(flow.dst_port) || graphExcludedIPs.has(flow.src_ip) || graphExcludedIPs.has(flow.dst_ip)) return graph
  if (!flow.src_vm_id) return graph
  const sourceExists = graph.nodes.some((node) => node.id === flow.src_vm_id && node.type === 'vm')
  if (!sourceExists) return graph

  const externalTarget = liveExternalTarget(flow)
  const targetID = flow.dst_vm_id || externalTarget?.id
  if (!targetID || targetID === flow.src_vm_id) return graph
  if (flow.dst_vm_id && !graph.nodes.some((node) => node.id === flow.dst_vm_id)) return graph

  const baseNodes = externalTarget && !graph.nodes.some((node) => node.id === externalTarget.id)
    ? [...graph.nodes, externalTarget]
    : graph.nodes
  const id = `${flow.src_vm_id}->${targetID}:${flow.dst_port}/${flow.protocol}`
  const index = graph.edges.findIndex((edge) => edge.id === id)
  const previous = index >= 0 ? graph.edges[index] : undefined
  const bytesSent = (previous?.bytes_sent ?? 0) + flow.bytes_sent
  const bytesReceived = (previous?.bytes_received ?? 0) + flow.bytes_received
  const animatesRequest = (flow.request_count ?? 0) > 0
  const activeUntil = animatesRequest
    ? new Date(Date.parse(observedAt) + activeWindowMs).toISOString()
    : previous?.active_until || observedAt
  const edge: GraphEdge = {
    id,
    source: flow.src_vm_id,
    target: targetID,
    protocol: flow.protocol,
    dst_port: flow.dst_port,
    scope: flow.scope,
    bytes_sent: bytesSent,
    bytes_received: bytesReceived,
    packets: (previous?.packets ?? 0) + flow.packets,
    connection_count: (previous?.connection_count ?? 0) + flow.connection_count,
    request_count: (previous?.request_count ?? 0) + (flow.request_count ?? 0),
    first_seen: previous?.first_seen ?? flow.first_seen,
    last_seen: flow.last_seen,
    last_observed_at: observedAt,
    active: animatesRequest || (previous?.active === true && Date.parse(previous.active_until) > Date.now()),
    active_until: activeUntil,
    weight: edgeWeight(bytesSent + bytesReceived),
    kind: 'traffic',
  }
  const edges = index >= 0
    ? graph.edges.map((item, edgeIndex) => edgeIndex === index ? edge : item)
    : [...graph.edges, edge]
  const nodes = baseNodes.map((node) => {
    if (node.id === flow.src_vm_id) {
      return { ...node, traffic_out: node.traffic_out + flow.bytes_sent, traffic_in: node.traffic_in + flow.bytes_received }
    }
    if (node.id === targetID) {
      return { ...node, traffic_in: node.traffic_in + flow.bytes_sent, traffic_out: node.traffic_out + flow.bytes_received }
    }
    return node
  })
  return { nodes, edges }
}

export function App() {
  const [graph, setGraph] = useState<GraphData>({ nodes: [], edges: [] })
  const [summary, setSummary] = useState<Summary>()
  const [internalActivity, setInternalActivity] = useState<InternalActivity[]>([])
  const [deepFlowRaw, setDeepFlowRaw] = useState<DeepFlowRawLogs>()
  const [deepFlowHealth, setDeepFlowHealth] = useState<DeepFlowHealth>()
  const [activityView, setActivityView] = useState<ActivityView>('internal')
  const [selectedNode, setSelectedNode] = useState<GraphNode>()
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const [freshTabs, setFreshTabs] = useState<Partial<Record<ActivityView, boolean>>>({})
  const refreshTimer = useRef<number>()
  const vmInventory = useRef<GraphNode[]>([])

  useEffect(() => {
    const previousScrollRestoration = window.history.scrollRestoration
    window.history.scrollRestoration = 'manual'
    const resetScroll = () => {
      window.scrollTo({ left: 0, top: 0 })
      document.scrollingElement?.scrollTo({ left: 0, top: 0 })
    }
    resetScroll()
    const resetInterval = window.setInterval(resetScroll, 100)
    const stopReset = window.setTimeout(() => window.clearInterval(resetInterval), 2500)
    return () => {
      window.clearInterval(resetInterval)
      window.clearTimeout(stopReset)
      window.history.scrollRestoration = previousScrollRestoration
    }
  }, [])

  const loadDeepFlow = useCallback(async () => {
    const [nextDeepFlowRaw, nextDeepFlowHealth] = await Promise.allSettled([
      api.deepFlowRaw(deepFlowLogWindow, deepFlowLogLimit), api.deepFlowHealth(),
    ])

    if (nextDeepFlowRaw.status === 'fulfilled') {
      setDeepFlowRaw(nextDeepFlowRaw.value)
    }
    if (nextDeepFlowHealth.status === 'fulfilled') {
      setDeepFlowHealth(nextDeepFlowHealth.value)
    }
  }, [])

  const load = useCallback(async () => {
    const [nextGraph, nextSummary, nextActivity, nextVMs] = await Promise.allSettled([
      api.graph(graphWindow), api.summary(), api.internalActivity(internalActivityLimit, internalActivityWindow), api.vms(),
    ])
    if (nextVMs.status === 'fulfilled') {
      vmInventory.current = nextVMs.value.map(vmToGraphNode)
    }
    if (nextGraph.status === 'fulfilled') {
      setGraph((current) => mergeVMInventory(mergeGraphData(current, nextGraph.value), vmInventory.current))
      setError('')
    } else {
      setError(nextGraph.reason instanceof Error ? nextGraph.reason.message : 'Unable to load network graph')
      if (nextVMs.status === 'fulfilled') {
        setGraph((current) => mergeVMInventory(current, vmInventory.current))
      }
    }
    if (nextSummary.status === 'fulfilled') {
      setSummary(nextSummary.value)
    }
    if (nextActivity.status === 'fulfilled') {
      setInternalActivity(nextActivity.value)
    }
    void loadDeepFlow()
  }, [loadDeepFlow])

  useEffect(() => {
    void load()
    // SSE remains the fast path, while polling guarantees status changes are
    // reflected even if a browser/proxy silently drops a realtime event.
    const interval = window.setInterval(() => void load(), 10_000)
    return () => window.clearInterval(interval)
  }, [load])
  useEffect(() => {
    const disconnect = connectRealtime((event) => {
      if (event.type === 'flow.updated' && isFlow(event.data)) {
        const flow = event.data
        setGraph((current) => applyLiveFlow(current, flow, event.timestamp))
        setSummary((current) => applyLiveSummary(current, flow, event.timestamp))
      }
      // Direct SSE mutation paints the active line immediately. A throttled
      // canonical refresh reconciles aggregate counters, nodes and metrics.
      if (refreshTimer.current === undefined) {
        refreshTimer.current = window.setTimeout(() => {
          refreshTimer.current = undefined
          void load()
        }, canonicalRefreshDelayMs)
      }
    }, setConnected)
    return () => {
      disconnect()
      window.clearTimeout(refreshTimer.current)
    }
  }, [load])

  useEffect(() => {
    if (!selectedNode) return
    const currentNode = graph.nodes.find((node) => node.id === selectedNode.id)
    if (!currentNode) setSelectedNode(undefined)
    else if (currentNode !== selectedNode) setSelectedNode(currentNode)
  }, [graph.nodes, selectedNode])

  const displayGraph = useMemo(() => vmTopologyOnly(graph), [graph])
  const vmCount = displayGraph.nodes.filter((node) => node.type === 'vm').length
  const vmIDs = new Set(displayGraph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  const relationshipCount = new Set(
    displayGraph.edges
      .filter((edge) => vmIDs.has(edge.source) && vmIDs.has(edge.target) && edge.source !== edge.target)
      .map((edge) => [edge.source, edge.target].sort().join('<->')),
  ).size
  const deepFlowConnectionCount = Array.isArray(deepFlowRaw?.l4) ? deepFlowRaw.l4.filter(isDeepFlowConnectionFlow).length : 0
  const deepFlowRequestCount = Array.isArray(deepFlowRaw?.l7) ? deepFlowRaw.l7.filter(isDeepFlowRequestFlow).length : 0
  const tableSignatures = useMemo<Record<ActivityView, string>>(() => {
    const l4Rows = Array.isArray(deepFlowRaw?.l4) ? deepFlowRaw.l4 : []
    const l7Rows = Array.isArray(deepFlowRaw?.l7) ? deepFlowRaw.l7 : []
    return {
      internal: internalActivity.slice(0, 5).map((item) => `${item.id}:${item.observed_at}`).join('|'),
      connection: l4Rows.filter(isDeepFlowConnectionFlow).slice(0, 5).map((item) => `${item.time}:${item.source_ip}:${item.dest_ip}:${item.server_port}`).join('|'),
      request: l7Rows.filter(isDeepFlowRequestFlow).slice(0, 5).map((item) => `${item.time}:${item.source_ip}:${item.dest_ip}:${item.request_resource}:${item.response_code}`).join('|'),
      l4: l4Rows.slice(0, 5).map((item) => `${item.time}:${item.source_ip}:${item.dest_ip}:${item.server_port}`).join('|'),
      l7: l7Rows.slice(0, 5).map((item) => `${item.time}:${item.source_ip}:${item.dest_ip}:${item.request_resource}:${item.response_code}`).join('|'),
    }
  }, [deepFlowRaw, internalActivity])
  const previousTableSignatures = useRef(tableSignatures)
  useEffect(() => {
    const changedTabs = (Object.keys(tableSignatures) as ActivityView[])
      .filter((tab) => previousTableSignatures.current[tab] !== tableSignatures[tab] && tableSignatures[tab] !== '')
    previousTableSignatures.current = tableSignatures
    if (changedTabs.length === 0) return
    setFreshTabs((current) => {
      const next = { ...current }
      changedTabs.forEach((tab) => { next[tab] = true })
      return next
    })
    const timeout = window.setTimeout(() => {
      setFreshTabs((current) => {
        const next = { ...current }
        changedTabs.forEach((tab) => { delete next[tab] })
        return next
      })
    }, tablePulseMs)
    return () => window.clearTimeout(timeout)
  }, [tableSignatures])
  const tableTabs: Array<{ id: ActivityView; label: string; count: number }> = [
    { id: 'internal', label: 'Internal Activity', count: internalActivity.length },
    { id: 'connection', label: 'Connection Flow', count: deepFlowConnectionCount },
    { id: 'request', label: 'Request Flow', count: deepFlowRequestCount },
    { id: 'l4', label: 'L4 Flows', count: Array.isArray(deepFlowRaw?.l4) ? deepFlowRaw.l4.length : 0 },
    { id: 'l7', label: 'L7 Requests', count: Array.isArray(deepFlowRaw?.l7) ? deepFlowRaw.l7.length : 0 },
  ]

  return <main className="app-shell">
    <header className="app-header">
      <div className="header-actions">
        <a className="grafana-link" href={GRAFANA_L4_URL} target="_blank" rel="noreferrer">Grafana L4</a>
        <a className="grafana-link" href={GRAFANA_L7_URL} target="_blank" rel="noreferrer">Grafana L7</a>
        <a className="grafana-link" href={GRAFANA_NETWORK_HOST_URL} target="_blank" rel="noreferrer">Network Host</a>
        <a className="grafana-link" href={GRAFANA_APPLICATION_HOST_URL} target="_blank" rel="noreferrer">App Host</a>
        <div className="live-state"><i className={connected ? 'connected' : ''} /><span>{connected ? 'Realtime connected' : 'Realtime reconnecting'}</span></div>
      </div>
    </header>
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <StatCards summary={summary} />
    <section className="workspace simple">
      <div className="graph-card">
        <div className="graph-heading">
          <div><small>VM TOPOLOGY</small></div>
          <div className="legend"><span className="vm-dot">Virtual machine</span><span className="edge-line idle-line">Connection</span><span className="edge-line active-line">Request traffic</span></div>
        </div>
        <GraphView graph={displayGraph} onNodeSelect={setSelectedNode} />
      </div>
      {selectedNode && <NodeDetailsPanel node={selectedNode} onClose={() => setSelectedNode(undefined)} />}
    </section>
    <section className="activity-switcher">
      <div className="activity-tabs" role="tablist" aria-label="Telemetry tables">
        {tableTabs.map((tab) => <button
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={activityView === tab.id}
          className={`activity-tab${activityView === tab.id ? ' active' : ''}${freshTabs[tab.id] ? ' updated' : ''}`}
          onClick={() => setActivityView(tab.id)}
        >
          <span>{tab.label}</span>
          <small>{tab.count}</small>
        </button>)}
      </div>
      {activityView === 'internal'
        ? <InternalActivityTable activity={internalActivity} windowLabel={internalActivityWindow} limit={internalActivityLimit} />
        : <DeepFlowFlowTable raw={deepFlowRaw} health={deepFlowHealth} mode={activityView} />}
    </section>
    <footer><span>Topology window {graphWindowLabel} · DeepFlow log window {deepFlowWindowLabel}</span><span>{vmCount} VMs · {relationshipCount} relationships</span></footer>
  </main>
}
