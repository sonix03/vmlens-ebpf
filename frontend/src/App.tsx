import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, GRAFANA_APPLICATION_HOST_URL, GRAFANA_L4_URL, GRAFANA_L7_URL, GRAFANA_NETWORK_HOST_URL } from './api/client'
import { connectRealtime } from './api/realtime'
import { EdgeDetailsPanel } from './components/EdgeDetailsPanel'
import { FlowTelemetryTable, type FlowTableMode } from './components/FlowTelemetryTable'
import { GraphView } from './components/GraphView'
import { InternalActivityTable } from './components/InternalActivityTable'
import { NodeDetailsPanel } from './components/NodeDetailsPanel'
import { StatCards } from './components/StatCards'
import type { Flow } from './types/flow'
import type { ConnectionSummary, GraphData, GraphEdge, GraphFilters, GraphNode } from './types/graph'
import type { InternalActivity } from './types/internalActivity'
import type { Summary } from './types/stats'
import type { VM } from './types/vm'
import { isConnectionFlow, isRequestFlow } from './utils/flowFilters'

const graphWindow: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '15m', min_bytes: '', status: '',
}

const internalActivityWindow = '5m'
const internalActivityLimit = 200

const graphWindowLabel = graphWindow.time_range

const categoryTabs = ['Orbit View', 'Compute', 'Network', 'Application', 'Diagnostics'] as const
const activeWindowMs = 4000
const canonicalRefreshDelayMs = 1000
const tablePulseMs = 900
type ActivityView = 'internal' | FlowTableMode
const graphExcludedPorts = new Set(
  ((import.meta.env.VITE_GRAPH_EXCLUDED_PORTS as string | undefined) ?? '22,53,123,8080,18080,18081,18082')
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
    private_ip: vm.private_ip,
    public_ip: vm.public_ip,
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
        private_ip: existing.private_ip || vmNode.private_ip,
        public_ip: existing.public_ip || vmNode.public_ip,
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
  const errorCount = flow.error_count ?? 0
  const animatesRequest = (flow.request_count ?? 0) > 0
  const activeUntil = animatesRequest
    ? new Date(Date.parse(observedAt) + activeWindowMs).toISOString()
    : previous?.active_until || observedAt
  const failedUntil = errorCount > 0
    ? new Date(Date.parse(observedAt) + activeWindowMs).toISOString()
    : previous?.failed_until
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
    error_count: (previous?.error_count ?? 0) + errorCount,
    first_seen: previous?.first_seen ?? flow.first_seen,
    last_seen: flow.last_seen,
    last_observed_at: observedAt,
    last_error_at: errorCount > 0 ? observedAt : previous?.last_error_at,
    active: animatesRequest || (previous?.active === true && Date.parse(previous.active_until) > Date.now()),
    active_until: activeUntil,
    failed: errorCount > 0 || (previous?.failed === true && previous.failed_until !== undefined && Date.parse(previous.failed_until) > Date.now()),
    failed_until: failedUntil,
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
  const [flowLog, setFlowLog] = useState<Flow[]>([])
  const [activityView, setActivityView] = useState<ActivityView>('internal')
  const [selectedNode, setSelectedNode] = useState<GraphNode>()
  const [selectedConnection, setSelectedConnection] = useState<ConnectionSummary>()
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

  const load = useCallback(async () => {
    const [nextGraph, nextSummary, nextActivity, nextFlows, nextVMs] = await Promise.allSettled([
      api.graph(graphWindow), api.summary(), api.internalActivity(internalActivityLimit, internalActivityWindow), api.flows(), api.vms(),
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
    if (nextFlows.status === 'fulfilled') {
      setFlowLog(nextFlows.value)
    }
  }, [])

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
        setFlowLog((current) => [flow, ...current.filter((item) => item.id !== flow.id)].slice(0, 500))
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

  useEffect(() => {
    if (!selectedConnection) return
    const sourceExists = graph.nodes.some((node) => node.id === selectedConnection.source)
    const targetExists = graph.nodes.some((node) => node.id === selectedConnection.target)
    if (!sourceExists || !targetExists) setSelectedConnection(undefined)
  }, [graph.nodes, selectedConnection])

  const handleNodeSelect = useCallback((node: GraphNode) => {
    setSelectedConnection(undefined)
    setSelectedNode(node)
  }, [])

  const handleConnectionSelect = useCallback((connection: ConnectionSummary) => {
    setSelectedNode(undefined)
    setSelectedConnection(connection)
  }, [])

  const displayGraph = useMemo(() => vmTopologyOnly(graph), [graph])
  const vmCount = displayGraph.nodes.filter((node) => node.type === 'vm').length
  const vmIDs = new Set(displayGraph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  const relationshipCount = new Set(
    displayGraph.edges
      .filter((edge) => vmIDs.has(edge.source) && vmIDs.has(edge.target) && edge.source !== edge.target)
      .map((edge) => [edge.source, edge.target].sort().join('<->')),
  ).size
  const tableSignatures = useMemo<Record<ActivityView, string>>(() => {
    return {
      internal: internalActivity.slice(0, 5).map((item) => `${item.id}:${item.observed_at}`).join('|'),
      connection: flowLog.filter(isConnectionFlow).slice(0, 5).map((item) => `${item.id}:${item.observed_at}:${item.src_ip}:${item.dst_ip}:${item.dst_port}`).join('|'),
      request: flowLog.filter(isRequestFlow).slice(0, 5).map((item) => `${item.id}:${item.observed_at}:${item.src_ip}:${item.dst_ip}:${item.request_count}:${item.error_count}`).join('|'),
      l4: flowLog.slice(0, 5).map((item) => `${item.id}:${item.observed_at}:${item.src_ip}:${item.dst_ip}:${item.dst_port}`).join('|'),
    }
  }, [flowLog, internalActivity])
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
    { id: 'connection', label: 'Connection Flow', count: flowLog.filter(isConnectionFlow).length },
    { id: 'request', label: 'Request Flow', count: flowLog.filter(isRequestFlow).length },
    { id: 'l4', label: 'L4 Flows', count: flowLog.length },
  ]

  return <main className="app-shell">
    <header className="app-header">
      <div className="brand-lockup" aria-label="VMLens Orbit">
        <span className="brand-mark">VL</span>
        <span className="brand-copy">
          <strong>VMLens</strong>
          <small>Orbit View Cloud</small>
        </span>
      </div>
      <div className="header-actions">
        <a className="grafana-link" href={GRAFANA_L4_URL} target="_blank" rel="noreferrer">Grafana L4</a>
        <a className="grafana-link" href={GRAFANA_L7_URL} target="_blank" rel="noreferrer">Grafana L7</a>
        <a className="grafana-link" href={GRAFANA_NETWORK_HOST_URL} target="_blank" rel="noreferrer">Network Host</a>
        <a className="grafana-link" href={GRAFANA_APPLICATION_HOST_URL} target="_blank" rel="noreferrer">App Host</a>
        <div className="live-state"><i className={connected ? 'connected' : ''} /><span>{connected ? 'Realtime connected' : 'Realtime reconnecting'}</span></div>
      </div>
    </header>
    <nav className="category-tabs" aria-label="Cloud product sections">
      {categoryTabs.map((tab) => <button
        key={tab}
        type="button"
        className={`category-tab${tab === 'Orbit View' ? ' active' : ''}`}
      >
        {tab}
      </button>)}
    </nav>
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <StatCards summary={summary} />
    <section className="workspace simple">
      <div className="graph-card">
        <div className="graph-heading graph-heading-compact">
          <div className="legend"><span className="vm-dot">Virtual machine</span><span className="edge-line idle-line">Stable RTT</span><span className="edge-line slow-line">Slow RTT</span><span className="edge-line active-line">Request traffic</span><span className="edge-line failed-line">Port refused</span></div>
        </div>
        <GraphView graph={displayGraph} onNodeSelect={handleNodeSelect} onConnectionSelect={handleConnectionSelect} />
      </div>
      {selectedNode && <NodeDetailsPanel node={selectedNode} graph={graph} onClose={() => setSelectedNode(undefined)} />}
      {selectedConnection && <EdgeDetailsPanel connection={selectedConnection} onClose={() => setSelectedConnection(undefined)} />}
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
        ? <InternalActivityTable activity={internalActivity} graph={graph} windowLabel={internalActivityWindow} limit={internalActivityLimit} />
        : <FlowTelemetryTable flows={flowLog} graph={graph} mode={activityView} />}
    </section>
    <footer><span>Topology window {graphWindowLabel} · TC/eBPF flow log</span><span>{vmCount} VMs · {relationshipCount} relationships</span></footer>
  </main>
}
