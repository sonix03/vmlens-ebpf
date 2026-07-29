import { useState } from 'react'
import type { ConnectionHealth, ConnectionSummary } from '../types/graph'
import { formatBytes } from './StatCards'

type ConnectionInfoTab = 'overview' | 'evidence' | 'metrics' | 'action'

const healthLabels: Record<ConnectionHealth, string> = {
  healthy: 'Healthy',
  degraded: 'Degraded',
  failed: 'Failed',
  inactive: 'Inactive',
  unknown: 'Unknown',
}

function formatTime(value?: string) {
  if (!value) return '—'
  const parsed = Date.parse(value)
  if (!Number.isFinite(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function directionLabel(connection: ConnectionSummary) {
  if (connection.active_direction === 'bidirectional') return 'Active both ways'
  if (connection.active_direction === 'source_to_target') return `${connection.source_label} → ${connection.target_label}`
  if (connection.active_direction === 'target_to_source') return `${connection.target_label} → ${connection.source_label}`
  return connection.direction === 'two_way' ? 'Observed both ways' : `${connection.source_label} → ${connection.target_label}`
}

function observedState(connection: ConnectionSummary) {
  if (connection.failed) return 'Request failed'
  if (connection.active) return 'Request active'
  if (connection.connected) return 'Connected idle'
  return 'Not connected'
}

function healthTone(connection: ConnectionSummary) {
  if (connection.health === 'failed') return 'danger'
  if (connection.health === 'degraded') return 'warning'
  if (connection.health === 'healthy') return 'healthy'
  return 'neutral'
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

export function EdgeDetailsPanel({ connection, onClose }: { connection: ConnectionSummary; onClose: () => void }) {
  const [activeTab, setActiveTab] = useState<ConnectionInfoTab>('overview')
  const protocolPort = [
    connection.protocols.length ? connection.protocols.map((item) => item.toUpperCase()).join('/') : 'UNKNOWN',
    connection.ports.length ? connection.ports.map((port) => `:${port}`).join(', ') : '',
  ].join(' ')
  const tone = healthTone(connection)
  const tabs: Array<{ id: ConnectionInfoTab; label: string }> = [
    { id: 'overview', label: 'Overview' },
    { id: 'evidence', label: 'Evidence' },
    { id: 'metrics', label: 'Metrics' },
    { id: 'action', label: 'Next Action' },
  ]

  return <aside className="detail-panel orbit-info-panel">
    <div className="detail-panel-header">
      <div className="detail-panel-title">
        <small className="eyebrow">Connection Object</small>
        <div className="detail-title-row">
          <h2>{connection.source_label} → {connection.target_label}</h2>
          <span className={`connection-health connection-health-${connection.health}`}>{healthLabels[connection.health]}</span>
        </div>
      </div>
      <button className="close" onClick={onClose}>×</button>
    </div>

    <div className="connection-status-row detail-status-row">
      <span className={`scope-pill scope-${connection.scope}`}>{connection.scope.replaceAll('_', ' ')}</span>
      <span className={`detail-health detail-health-${tone}`}>{observedState(connection)}</span>
    </div>

    <div className="detail-traffic-line">
      <span><em>↔</em><strong>{formatBytes(connection.total_bytes)}</strong> traffic</span>
      <i />
      <span><em>●</em><strong>{connection.request_count.toLocaleString()}</strong> requests</span>
    </div>

    <section className="detail-system-check">
      <div className="detail-section-title">
        <span>Connection Check</span>
        <b className={`detail-health detail-health-${tone}`}>{healthLabels[connection.health]}</b>
      </div>
      <div className="detail-check-grid">
        <CheckCard label="RTT avg" value={connection.avg_rtt_ms > 0 ? `${connection.avg_rtt_ms.toFixed(1)} ms` : '—'} note="network delay" tone={connection.slow ? 'warning' : connection.avg_rtt_ms > 0 ? 'healthy' : 'neutral'} />
        <CheckCard label="Errors" value={connection.error_count.toString()} note="current window" tone={connection.error_count > 0 ? 'danger' : 'healthy'} />
        <CheckCard label="Packets" value={connection.packets.toLocaleString()} note="L4 evidence" tone={connection.packets > 0 ? 'info' : 'neutral'} />
        <CheckCard label="Last seen" value={connection.last_observed_at ? new Date(Date.parse(connection.last_observed_at)).toLocaleTimeString() : '—'} note="latest evidence" tone={connection.last_observed_at ? 'healthy' : 'neutral'} />
      </div>
    </section>

    <div className="detail-tabs" role="tablist" aria-label="Connection information sections">
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
          <div className="detail-section-title"><span>Path</span></div>
          <div className="detail-meta-grid">
            <MetaRow label="Source host" value={connection.source_label} />
            <MetaRow label="Source IP" value={connection.source_ip || connection.source} />
            <MetaRow label="Target host" value={connection.target_label} />
            <MetaRow label="Target IP" value={connection.target_ip || connection.target} />
            <MetaRow label="Direction" value={directionLabel(connection)} />
            <MetaRow label="Protocol / port" value={protocolPort} />
          </div>
        </section>
        <section className="detail-card">
          <div className="detail-section-title"><span>Observed behavior</span></div>
          <p className="detail-copy">Idle green line means the path is considered connected. Moving line means request traffic is active. Yellow means slow RTT. Red means a failed attempt or refused service path.</p>
        </section>
      </div>}

      {activeTab === 'evidence' && <section className="detail-card">
        <div className="detail-section-title"><span>Telemetry evidence</span></div>
        <div className="detail-meta-grid">
          <MetaRow label="Observed state" value={observedState(connection)} />
          <MetaRow label="Scope" value={connection.scope.replaceAll('_', ' ')} />
          <MetaRow label="Last seen" value={formatTime(connection.last_seen)} />
          <MetaRow label="Last observed" value={formatTime(connection.last_observed_at)} />
          <MetaRow label="Tracker agents" value={connection.agent_ids.length ? connection.agent_ids.join(', ') : '—'} />
          <MetaRow label="Evidence points" value={connection.observation_points.length ? connection.observation_points.join(', ') : '—'} />
        </div>
      </section>}

      {activeTab === 'metrics' && <section className="detail-card">
        <div className="detail-section-title"><span>Metrics</span></div>
        <div className="detail-meta-grid">
          <MetaRow label="Traffic" value={formatBytes(connection.total_bytes)} />
          <MetaRow label="Requests" value={connection.request_count.toLocaleString()} />
          <MetaRow label="Errors" value={connection.error_count.toLocaleString()} />
          <MetaRow label="Packets" value={connection.packets.toLocaleString()} />
          <MetaRow label="Connections" value={connection.connection_count.toLocaleString()} />
          <MetaRow label="Avg RTT" value={connection.avg_rtt_ms > 0 ? `${connection.avg_rtt_ms.toFixed(2)} ms` : '—'} />
          <MetaRow label="P95 RTT" value={connection.p95_rtt_ms > 0 ? `${connection.p95_rtt_ms.toFixed(2)} ms` : '—'} />
          <MetaRow label="Avg app delay" value={connection.avg_response_duration_ms > 0 ? `${connection.avg_response_duration_ms.toFixed(2)} ms` : '—'} />
          <MetaRow label="Last response" value={connection.last_response_code ? String(connection.last_response_code) : '—'} />
        </div>
      </section>}

      {activeTab === 'action' && <div className="connection-next-step detail-action-card">
        <small>NEXT ACTION</small>
        <p>{connection.recommendation}</p>
      </div>}
    </div>
  </aside>
}
