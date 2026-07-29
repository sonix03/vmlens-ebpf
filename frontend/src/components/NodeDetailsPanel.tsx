import { useMemo, useState } from 'react'
import type { GraphData, GraphEdge, GraphNode } from '../types/graph'
import { formatBytes } from './StatCards'
import { StatusBadge } from './StatusBadge'

const slowRTTThresholdMs = positiveNumberEnv(import.meta.env.VITE_SLOW_RTT_THRESHOLD_MS, 100)

type NodeInfoTab = 'overview' | 'connections' | 'evidence' | 'info'

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

function formatTime(value?: string) {
  if (!value) return '—'
  const parsed = Date.parse(value)
  if (!Number.isFinite(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function serviceLabel(edge: GraphEdge) {
  const port = edge.server_port || edge.dst_port
  return `${edge.protocol.toUpperCase()}${port > 0 ? ` :${port}` : ''}`
}

function nodeHealthLabel(node: GraphNode, incoming: GraphEdge[], outgoing: GraphEdge[]) {
  const edges = [...incoming, ...outgoing]
  if ((node.status ?? '').toLowerCase() === 'offline') return 'Offline'
  if (edges.some((edge) => edgeHealth(edge) === 'failed')) return 'Failed path observed'
  if (edges.some((edge) => edgeHealth(edge) === 'degraded')) return 'Degraded RTT'
  if (edges.some((edge) => edgeHealth(edge) === 'healthy')) return 'Connected'
  return 'No recent edge'
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return <div className="detail-meta-row">
    <span>{label}</span>
    <strong>{value}</strong>
  </div>
}

function CheckCard({
  label,
  value,
  note,
  tone = 'neutral',
}: {
  label: string
  value: string
  note: string
  tone?: 'healthy' | 'warning' | 'danger' | 'info' | 'neutral'
}) {
  return <div className={`detail-check-card detail-check-${tone}`}>
    <span>{label}</span>
    <strong>{value}</strong>
    <small>{note}</small>
  </div>
}

function EvidenceRows({
  edges,
  nodes,
}: {
  edges: GraphEdge[]
  nodes: Map<string, GraphNode>
}) {
  if (edges.length === 0) {
    return <p className="host-connection-empty">No evidence row is available in the current graph window.</p>
  }

  return <div className="detail-evidence-list">
    {edges.slice(0, 10).map((edge) => {
      const health = edgeHealth(edge)
      return <div key={`evidence-${edge.id}`} className={`detail-evidence-row detail-evidence-${health}`}>
        <div>
          <strong>{endpointName(nodes, edge.source)} → {endpointName(nodes, edge.target)}</strong>
          <small>{serviceLabel(edge)} · {edge.scope.replaceAll('_', ' ')}</small>
        </div>
        <div>
          <span>{edge.avg_rtt_ms ? `${edge.avg_rtt_ms.toFixed(1)} ms` : '—'}</span>
          <small>RTT</small>
        </div>
        <div>
          <span>{(edge.error_count ?? 0).toLocaleString()}</span>
          <small>errors</small>
        </div>
        <div>
          <span>{edgeTime(edge)}</span>
          <small>last seen</small>
        </div>
      </div>
    })}
  </div>
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
  const [activeTab, setActiveTab] = useState<NodeInfoTab>('overview')
  const nodes = useMemo(() => new Map(graph.nodes.map((item) => [item.id, item])), [graph.nodes])
  const incoming = useMemo(() => graph.edges
    .filter((edge) => edge.target === node.id && edge.source !== node.id)
    .sort((a, b) => Date.parse(b.last_observed_at || b.last_seen) - Date.parse(a.last_observed_at || a.last_seen)), [graph.edges, node.id])
  const outgoing = useMemo(() => graph.edges
    .filter((edge) => edge.source === node.id && edge.target !== node.id)
    .sort((a, b) => Date.parse(b.last_observed_at || b.last_seen) - Date.parse(a.last_observed_at || a.last_seen)), [graph.edges, node.id])
  const evidence = useMemo(() => [...incoming, ...outgoing]
    .sort((a, b) => Date.parse(b.last_observed_at || b.last_seen) - Date.parse(a.last_observed_at || a.last_seen)), [incoming, outgoing])
  const publicExposure = Boolean(node.public_ip) || incoming.some((edge) => edge.scope === 'external_public')
  const criticalDependency = outgoing.find((edge) => edge.scope.includes('internal') && edge.request_count > 0)
  const healthLabel = nodeHealthLabel(node, incoming, outgoing)
  const statusTone = healthLabel.includes('Failed') ? 'danger' : healthLabel.includes('Degraded') ? 'warning' : healthLabel === 'Connected' ? 'healthy' : 'neutral'
  const lastEvidence = evidence[0]?.last_observed_at || evidence[0]?.last_seen

  const tabs: Array<{ id: NodeInfoTab; label: string }> = [
    { id: 'overview', label: 'Overview' },
    { id: 'connections', label: 'Connections' },
    { id: 'evidence', label: 'Evidence' },
    { id: 'info', label: 'Information' },
  ]

  return <aside className="detail-panel orbit-info-panel">
    <div className="detail-panel-header">
      <div className="detail-panel-title">
        <small className="eyebrow">Virtual Machine</small>
        <div className="detail-title-row">
          <h2>{node.label}</h2>
          <StatusBadge status={node.status} />
        </div>
      </div>
      <button className="close" onClick={onClose}>×</button>
    </div>

    <div className="detail-traffic-line">
      <span><em>↓</em><strong>{formatBytes(node.traffic_in)}</strong> in</span>
      <i />
      <span><em>↑</em><strong>{formatBytes(node.traffic_out)}</strong> out</span>
    </div>

    <section className="detail-system-check">
      <div className="detail-section-title">
        <span>System Check</span>
        <b className={`detail-health detail-health-${statusTone}`}>{healthLabel}</b>
      </div>
      <div className="detail-check-grid">
        <CheckCard label="Incoming" value={incoming.length.toString()} note="observed paths" tone={incoming.length > 0 ? 'info' : 'neutral'} />
        <CheckCard label="Outgoing" value={outgoing.length.toString()} note="observed paths" tone={outgoing.length > 0 ? 'info' : 'neutral'} />
        <CheckCard label="Exposure" value={publicExposure ? 'Public' : 'Private'} note={publicExposure ? 'external evidence' : 'current evidence'} tone={publicExposure ? 'warning' : 'healthy'} />
        <CheckCard label="Last data" value={lastEvidence ? new Date(Date.parse(lastEvidence)).toLocaleTimeString() : '—'} note="latest evidence" tone={lastEvidence ? 'healthy' : 'neutral'} />
      </div>
    </section>

    <div className="detail-tabs" role="tablist" aria-label="VM information sections">
      {tabs.map((tab) => <button
        key={tab.id}
        type="button"
        role="tab"
        aria-selected={activeTab === tab.id}
        className={`detail-tab${activeTab === tab.id ? ' active' : ''}`}
        onClick={() => setActiveTab(tab.id)}
      >
        {tab.label}
      </button>)}
    </div>

    <div className="detail-tab-body">
      {activeTab === 'overview' && <div className="detail-section-stack">
        <section className="detail-card">
          <div className="detail-section-title"><span>Connectivity summary</span></div>
          <div className="detail-meta-grid">
            <MetaRow label="Observed status" value={healthLabel} />
            <MetaRow label="Primary IP" value={node.ip || '—'} />
            <MetaRow label="Private IP" value={node.private_ip || node.ip || '—'} />
            <MetaRow label="Public IP" value={node.public_ip || 'none observed'} />
          </div>
        </section>
        <section className="detail-card">
          <div className="detail-section-title"><span>Impact</span></div>
          <p className="detail-copy">{incoming.length > 0 ? `${incoming.length} observed upstream path(s) may be affected if this host stops.` : 'No observed upstream dependency in the current window.'}</p>
          <p className="detail-copy">{criticalDependency ? `Likely downstream dependency: ${endpointName(nodes, criticalDependency.target)} via ${serviceLabel(criticalDependency)}.` : 'No critical downstream dependency inferred in the current window.'}</p>
        </section>
      </div>}

      {activeTab === 'connections' && <div className="detail-section-stack">
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
      </div>}

      {activeTab === 'evidence' && <EvidenceRows edges={evidence} nodes={nodes} />}

      {activeTab === 'info' && <div className="detail-section-stack">
        <section className="detail-card">
          <div className="detail-section-title"><span>VM Details</span></div>
          <div className="detail-meta-grid">
            <MetaRow label="VM ID" value={node.id} />
            <MetaRow label="VM Name" value={node.label} />
            <MetaRow label="Status" value={node.status || 'unknown'} />
            <MetaRow label="Tenant" value={node.tenant_id || 'unassigned'} />
            <MetaRow label="Role" value={node.role || 'unassigned'} />
            <MetaRow label="Type" value={node.type} />
          </div>
        </section>
        <section className="detail-card">
          <div className="detail-section-title"><span>Operational note</span></div>
          <p className="detail-copy">This panel shows inventory and observed telemetry. Raw rows remain in Internal Activity, Connection Flow, Request Flow, and L4 Flow tables.</p>
        </section>
      </div>}
    </div>
  </aside>
}
