import { useMemo } from 'react'
import type { GraphData, GraphEdge, GraphNode } from '../types/graph'
import { formatBytes } from './StatCards'
import { StatusBadge } from './StatusBadge'

const slowRTTThresholdMs = positiveNumberEnv(import.meta.env.VITE_SLOW_RTT_THRESHOLD_MS, 100)

function positiveNumberEnv(raw: unknown, fallback: number) {
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function endpointName(nodes: Map<string, GraphNode>, id: string) {
  const node = nodes.get(id)
  if (!node) return id
  return node.ip ? `${node.label} (${node.ip})` : node.label
}

function edgeHealth(edge: GraphEdge) {
  if (edge.failed || (edge.error_count ?? 0) > 0) return 'failed'
  if ((edge.avg_rtt_ms ?? 0) >= slowRTTThresholdMs) return 'degraded'
  if (edge.active || edge.request_count > 0 || edge.connection_count > 0 || edge.packets > 0) return 'healthy'
  return 'inactive'
}

function edgeTime(edge: GraphEdge) {
  const parsed = Date.parse(edge.last_observed_at || edge.last_seen)
  if (!Number.isFinite(parsed)) return '—'
  return new Date(parsed).toLocaleTimeString()
}

function serviceLabel(edge: GraphEdge) {
  const port = edge.server_port || edge.dst_port
  return `${edge.protocol.toUpperCase()}${port > 0 ? ` :${port}` : ''}`
}

function HostConnectionRows({
  title,
  empty,
  edges,
  nodes,
  mode,
}: {
  title: string
  empty: string
  edges: GraphEdge[]
  nodes: Map<string, GraphNode>
  mode: 'incoming' | 'outgoing'
}) {
  return <section className="host-connection-section">
    <div className="host-connection-title">
      <small>{title}</small>
      <span>{edges.length}</span>
    </div>
    {edges.length === 0 ? <p className="host-connection-empty">{empty}</p> : <div className="host-connection-list">
      {edges.slice(0, 8).map((edge) => {
        const peerID = mode === 'incoming' ? edge.source : edge.target
        const health = edgeHealth(edge)
        return <div key={`${mode}-${edge.id}`} className={`host-connection-row host-connection-${health}`}>
          <span>
            <strong>{endpointName(nodes, peerID)}</strong>
            <small>{mode === 'incoming' ? 'accesses this host' : 'accessed by this host'}</small>
          </span>
          <span>
            <strong>{serviceLabel(edge)}</strong>
            <small>{edge.scope.replaceAll('_', ' ')}</small>
          </span>
          <span>
            <strong>{edge.avg_rtt_ms ? `${edge.avg_rtt_ms.toFixed(1)} ms` : '—'}</strong>
            <small>RTT</small>
          </span>
          <span>
            <strong>{formatBytes(edge.total_bytes ?? edge.bytes_sent + edge.bytes_received)}</strong>
            <small>{edgeTime(edge)}</small>
          </span>
        </div>
      })}
    </div>}
  </section>
}

export function NodeDetailsPanel({ node, graph, onClose }: { node: GraphNode; graph: GraphData; onClose: () => void }) {
  const nodes = useMemo(() => new Map(graph.nodes.map((item) => [item.id, item])), [graph.nodes])
  const incoming = useMemo(() => graph.edges
    .filter((edge) => edge.target === node.id && edge.source !== node.id)
    .sort((a, b) => Date.parse(b.last_observed_at || b.last_seen) - Date.parse(a.last_observed_at || a.last_seen)), [graph.edges, node.id])
  const outgoing = useMemo(() => graph.edges
    .filter((edge) => edge.source === node.id && edge.target !== node.id)
    .sort((a, b) => Date.parse(b.last_observed_at || b.last_seen) - Date.parse(a.last_observed_at || a.last_seen)), [graph.edges, node.id])
  const publicExposure = Boolean(node.public_ip) || incoming.some((edge) => edge.scope === 'external_public')
  const criticalDependency = outgoing.find((edge) => edge.scope.includes('internal') && edge.request_count > 0)

  return <aside className="detail-panel">
    <button className="close" onClick={onClose}>×</button>
    <small className="eyebrow">VIRTUAL MACHINE</small>
    <h2>{node.label}</h2>
    <StatusBadge status={node.status} />
    <dl>
      <dt>VM ID</dt><dd>{node.id}</dd>
      <dt>Primary IP</dt><dd>{node.ip || '—'}</dd>
      <dt>Private IP</dt><dd>{node.private_ip || node.ip || '—'}</dd>
      <dt>Public IP</dt><dd>{node.public_ip || 'none observed'}</dd>
      <dt>Tenant</dt><dd>{node.tenant_id || 'unassigned'}</dd>
      <dt>Role</dt><dd>{node.role || 'unassigned'}</dd>
      <dt>Exposure</dt><dd>{publicExposure ? 'public/external traffic observed' : 'private-only from current evidence'}</dd>
      <dt>Incoming</dt><dd>{incoming.length.toLocaleString()} connections</dd>
      <dt>Outgoing</dt><dd>{outgoing.length.toLocaleString()} connections</dd>
      <dt>Impact hint</dt><dd>{incoming.length > 0 ? `${incoming.length} observed upstream path(s) may be affected if this host stops.` : 'No observed upstream dependency in current window.'}</dd>
      <dt>Dependency hint</dt><dd>{criticalDependency ? `Depends on ${endpointName(nodes, criticalDependency.target)} via ${serviceLabel(criticalDependency)}.` : 'No critical downstream dependency inferred in current window.'}</dd>
      <dt>Traffic in</dt><dd>{formatBytes(node.traffic_in)}</dd>
      <dt>Traffic out</dt><dd>{formatBytes(node.traffic_out)}</dd>
    </dl>
    <HostConnectionRows
      title="Incoming connections"
      empty="No incoming connection observed in the current graph window."
      edges={incoming}
      nodes={nodes}
      mode="incoming"
    />
    <HostConnectionRows
      title="Outgoing connections"
      empty="No outgoing connection observed in the current graph window."
      edges={outgoing}
      nodes={nodes}
      mode="outgoing"
    />
  </aside>
}
