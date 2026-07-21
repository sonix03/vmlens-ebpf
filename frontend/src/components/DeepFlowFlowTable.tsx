import type { DeepFlowHealth, DeepFlowRawLogs } from '../types/deepflow'
import { formatBytes } from './StatCards'

function timeLabel(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

function healthState(health?: DeepFlowHealth) {
  if (!health?.enabled) return 'disabled'
  if (health.errors?.length) return 'degraded'
  if (health.clickhouse_reachable && health.agent_list_not_empty) return 'connected'
  return 'waiting'
}

export function DeepFlowFlowTable({ raw, health }: { raw?: DeepFlowRawLogs; health?: DeepFlowHealth }) {
  const l7 = raw?.l7.slice(0, 80) ?? []
  const l4 = raw?.l4.slice(0, 80) ?? []
  const state = healthState(health)
  return <section className="activity-card deepflow-card">
    <div className="activity-heading">
      <div><small>DEEPFLOW RAW LOGS</small><span>L7 requests and L4 flows after inventory filtering</span></div>
      <span className={`deepflow-state deepflow-${state}`}>{state}</span>
    </div>
    {health?.warnings?.length ? <div className="deepflow-note">{health.warnings.slice(0, 2).join(' · ')}</div> : null}
    {health?.errors?.length ? <div className="deepflow-note error">{health.errors.slice(0, 2).join(' · ')}</div> : null}
    <div className="activity-table-wrap">
      <table className="activity-table deepflow-table">
        <thead><tr><th>Time</th><th>Route</th><th>Layer</th><th>Status</th><th>Traffic</th><th>Latency</th><th>Agent / Point</th></tr></thead>
        <tbody>
          {l7.map((item, index) => <tr key={`l7-${item.time}-${index}`}>
            <td className="activity-time">{timeLabel(item.time)}</td>
            <td><div className="activity-route">
              <span className="activity-endpoint"><strong>{item.source_ip}</strong><small>{item.request_domain || 'no-domain'}</small></span>
              <span className="activity-arrow">→</span>
              <span className="activity-endpoint"><strong>{item.dest_ip}</strong><small>{item.request_resource || '/'}</small></span>
            </div></td>
            <td><span className="protocol-pill">{item.l7_protocol_str || 'L7'}</span><small className="direction-label">{item.request_type}</small></td>
            <td><span className={item.response_code >= 400 ? 'deepflow-error-code' : 'deepflow-ok-code'}>{item.response_code || '—'}</span></td>
            <td><div className="activity-bytes"><span>req {formatBytes(item.request_length)}</span><span>res {formatBytes(item.response_length)}</span></div></td>
            <td><div className="activity-frequency"><span>{item.response_duration_ms.toFixed(2)} ms</span><small>{item.internet_direction}</small></div></td>
            <td><div className="activity-frequency"><span>{item.agent_id || '—'}</span><small>{item.observation_point || '—'}</small></div></td>
          </tr>)}
          {l7.length === 0 && l4.map((item, index) => <tr key={`l4-${item.time}-${index}`}>
            <td className="activity-time">{timeLabel(item.time)}</td>
            <td><div className="activity-route">
              <span className="activity-endpoint"><strong>{item.source_ip}</strong><small>:{item.client_port}</small></span>
              <span className="activity-arrow">→</span>
              <span className="activity-endpoint"><strong>{item.dest_ip}</strong><small>:{item.server_port}</small></span>
            </div></td>
            <td><span className="protocol-pill">{item.protocol || 'L4'}</span><small className="direction-label">{item.internet_direction}</small></td>
            <td><span className={item.status.toLowerCase().includes('error') ? 'deepflow-error-code' : 'deepflow-ok-code'}>{item.status || '—'}</span></td>
            <td><div className="activity-bytes"><span>tx {formatBytes(item.byte_tx)}</span><span>rx {formatBytes(item.byte_rx)}</span></div></td>
            <td><div className="activity-frequency"><span>{item.rtt_ms.toFixed(2)} ms</span><small>{item.retrans_total} retrans</small></div></td>
            <td><div className="activity-frequency"><span>{item.agent_id || '—'}</span><small>L4</small></div></td>
          </tr>)}
          {l7.length === 0 && l4.length === 0 && <tr><td colSpan={7} className="activity-empty">Waiting for DeepFlow L4/L7 telemetry…</td></tr>}
        </tbody>
      </table>
    </div>
  </section>
}
