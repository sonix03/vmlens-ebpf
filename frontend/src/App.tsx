import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api/client'
import { connectRealtime } from './api/realtime'
import { GraphView } from './components/GraphView'
import { InternalActivityTable } from './components/InternalActivityTable'
import { NodeDetailsPanel } from './components/NodeDetailsPanel'
import { StatCards } from './components/StatCards'
import type { Flow } from './types/flow'
import type { GraphData, GraphEdge, GraphFilters, GraphNode } from './types/graph'
import type { InternalActivity } from './types/internalActivity'
import type { Summary } from './types/stats'

const graphWindow: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '24h', min_bytes: '', status: '',
}

const activeWindowMs = 3000

function isFlow(value: unknown): value is Flow {
  if (!value || typeof value !== 'object') return false
  const flow = value as Partial<Flow>
  return typeof flow.src_ip === 'string'
    && typeof flow.dst_ip === 'string'
    && (flow.protocol === 'tcp' || flow.protocol === 'udp')
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
  if (flow.scope === 'external_public') {
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

function applyLiveFlow(graph: GraphData, flow: Flow, observedAt: string): GraphData {
  if (!flow.src_vm_id || !flow.dst_vm_id || flow.src_vm_id === flow.dst_vm_id) return graph
  const vmIDs = new Set(graph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  if (!vmIDs.has(flow.src_vm_id) || !vmIDs.has(flow.dst_vm_id)) return graph

  const id = `${flow.src_vm_id}->${flow.dst_vm_id}:${flow.dst_port}/${flow.protocol}`
  const index = graph.edges.findIndex((edge) => edge.id === id)
  const previous = index >= 0 ? graph.edges[index] : undefined
  const bytesSent = (previous?.bytes_sent ?? 0) + flow.bytes_sent
  const bytesReceived = (previous?.bytes_received ?? 0) + flow.bytes_received
  const edge: GraphEdge = {
    id,
    source: flow.src_vm_id,
    target: flow.dst_vm_id,
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
    active: true,
    active_until: new Date(Date.parse(observedAt) + activeWindowMs).toISOString(),
    weight: edgeWeight(bytesSent + bytesReceived),
  }
  const edges = index >= 0
    ? graph.edges.map((item, edgeIndex) => edgeIndex === index ? edge : item)
    : [...graph.edges, edge]
  const nodes = graph.nodes.map((node) => {
    if (node.id === flow.src_vm_id) {
      return { ...node, traffic_out: node.traffic_out + flow.bytes_sent, traffic_in: node.traffic_in + flow.bytes_received }
    }
    if (node.id === flow.dst_vm_id) {
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
  const [selectedNode, setSelectedNode] = useState<GraphNode>()
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const refreshTimer = useRef<number>()

  const load = useCallback(async () => {
    try {
      const [nextGraph, nextSummary, nextActivity] = await Promise.all([
        api.graph(graphWindow), api.summary(), api.internalActivity(),
      ])
      setGraph(nextGraph); setSummary(nextSummary); setInternalActivity(nextActivity); setError('')
    } catch (reason) {
      setGraph({ nodes: [], edges: [] })
      setSummary(undefined)
      setInternalActivity([])
      setSelectedNode(undefined)
      setError(reason instanceof Error ? reason.message : 'Unable to load network data')
    } finally { setLoading(false) }
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
      }
      // Direct SSE mutation paints the active line immediately. A throttled
      // canonical refresh reconciles aggregate counters, nodes and metrics.
      if (refreshTimer.current === undefined) {
        refreshTimer.current = window.setTimeout(() => {
          refreshTimer.current = undefined
          void load()
        }, 300)
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

  const vmCount = graph.nodes.filter((node) => node.type === 'vm').length
  const vmIDs = new Set(graph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  const relationshipCount = new Set(
    graph.edges
      .filter((edge) => vmIDs.has(edge.source) && vmIDs.has(edge.target) && edge.source !== edge.target)
      .map((edge) => [edge.source, edge.target].sort().join('<->')),
  ).size

  return <main className="app-shell">
    <header className="app-header">
      <div className="live-state"><i className={connected ? 'connected' : ''} /><span>{connected ? 'Realtime connected' : 'Realtime reconnecting'}</span></div>
    </header>
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <StatCards summary={summary} />
    <section className="workspace simple">
      <div className="graph-card">
        <div className="graph-heading">
          <div><small>VM TOPOLOGY</small></div>
          <div className="legend"><span className="vm-dot">Virtual machine</span><span className="edge-line active-line">Active traffic</span><span className="edge-line idle-line">Idle history</span></div>
        </div>
        {loading ? <div className="loading">Loading live topology...</div> : <GraphView graph={graph} onNodeSelect={setSelectedNode} />}
      </div>
      {selectedNode && <NodeDetailsPanel node={selectedNode} onClose={() => setSelectedNode(undefined)} />}
    </section>
    <InternalActivityTable activity={internalActivity} />
    <footer><span></span><span>{vmCount} VMs · {relationshipCount} relationships</span></footer>
  </main>
}
