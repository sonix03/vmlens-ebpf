import type { ConnectionHealth, ConnectionSummary } from '../types/graph'
import { formatBytes } from './StatCards'

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
  return connection.direction === 'two_way' ? 'Configured/observed both ways' : `${connection.source_label} → ${connection.target_label}`
}

export function EdgeDetailsPanel({ connection, onClose }: { connection: ConnectionSummary; onClose: () => void }) {
  const protocolPort = [
    connection.protocols.length ? connection.protocols.map((item) => item.toUpperCase()).join('/') : 'UNKNOWN',
    connection.ports.length ? connection.ports.map((port) => `:${port}`).join(', ') : '',
  ].join(' ')

  return <aside className="detail-panel">
    <button className="close" onClick={onClose}>×</button>
    <small className="eyebrow">CONNECTION OBJECT</small>
    <h2>{connection.source_label} → {connection.target_label}</h2>
    <div className="connection-status-row">
      <span className={`connection-health connection-health-${connection.health}`}>{healthLabels[connection.health]}</span>
      <span className={`scope-pill scope-${connection.scope}`}>{connection.scope.replaceAll('_', ' ')}</span>
    </div>
    <dl>
      <dt>Source host</dt><dd>{connection.source_label}<br /><small>{connection.source_ip || connection.source}</small></dd>
      <dt>Target host</dt><dd>{connection.target_label}<br /><small>{connection.target_ip || connection.target}</small></dd>
      <dt>Direction</dt><dd>{directionLabel(connection)}</dd>
      <dt>Protocol / port</dt><dd>{protocolPort}</dd>
      <dt>Observed state</dt><dd>{connection.active ? 'Request active' : connection.connected ? 'Connected idle' : 'Not connected'}</dd>
      <dt>Traffic</dt><dd>{formatBytes(connection.total_bytes)}</dd>
      <dt>Requests</dt><dd>{connection.request_count.toLocaleString()}</dd>
      <dt>Errors</dt><dd>{connection.error_count.toLocaleString()}</dd>
      <dt>Packets</dt><dd>{connection.packets.toLocaleString()}</dd>
      <dt>Connections</dt><dd>{connection.connection_count.toLocaleString()}</dd>
      <dt>Avg RTT</dt><dd>{connection.avg_rtt_ms > 0 ? `${connection.avg_rtt_ms.toFixed(2)} ms` : '—'}</dd>
      <dt>P95 RTT</dt><dd>{connection.p95_rtt_ms > 0 ? `${connection.p95_rtt_ms.toFixed(2)} ms` : '—'}</dd>
      <dt>Avg app delay</dt><dd>{connection.avg_response_duration_ms > 0 ? `${connection.avg_response_duration_ms.toFixed(2)} ms` : '—'}</dd>
      <dt>Last response</dt><dd>{connection.last_response_code ?? '—'}</dd>
      <dt>Last seen</dt><dd>{formatTime(connection.last_seen)}</dd>
      <dt>Last observed</dt><dd>{formatTime(connection.last_observed_at)}</dd>
      <dt>Tracker agents</dt><dd>{connection.agent_ids.length ? connection.agent_ids.join(', ') : '—'}</dd>
      <dt>Evidence points</dt><dd>{connection.observation_points.length ? connection.observation_points.join(', ') : '—'}</dd>
    </dl>
    <div className="connection-next-step">
      <small>NEXT ACTION</small>
      <p>{connection.recommendation}</p>
    </div>
  </aside>
}
