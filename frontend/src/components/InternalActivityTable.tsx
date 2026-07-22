import type { InternalActivity } from '../types/internalActivity'
import { formatBytes } from './StatCards'

function endpoint(name: string, ip: string) {
  return <span className="activity-endpoint"><strong>{name}</strong><small>{ip}</small></span>
}

function activityTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
}

export function InternalActivityTable({
  activity,
  windowLabel,
  limit,
}: {
  activity: InternalActivity[]
  windowLabel: string
  limit: number
}) {
  return <section className="activity-card">
    <div className="activity-heading">
      <div><small>INTERNAL ACTIVITY</small><span>Registered VM-to-VM observations · short realtime window</span></div>
      <span>{activity.length}/{limit} latest · {windowLabel}</span>
    </div>
    <div className="activity-table-wrap">
      <table className="activity-table">
        <thead><tr><th>Observed</th><th>Route</th><th>Service</th><th>Protocol</th><th>Traffic</th><th>Frequency</th><th>Observer</th></tr></thead>
        <tbody>
          {activity.map((item) => <tr key={`${item.id}-${item.direction}`}>
            <td className="activity-time">{activityTime(item.observed_at)}</td>
            <td><div className="activity-route">
              {endpoint(item.source_name, item.source_ip)}
              <span className={`activity-arrow ${item.direction}`} aria-label={item.direction}>→</span>
              {endpoint(item.destination_name, item.destination_ip)}
            </div></td>
            <td><span className="service-pill">{item.service}</span><small className="service-port">:{item.service_port}</small></td>
            <td><span className="protocol-pill">{item.protocol}</span><small className="direction-label">{item.direction}</small></td>
            <td><div className="activity-bytes"><span>↑ {formatBytes(item.bytes_sent)}</span><span>↓ {formatBytes(item.bytes_received)}</span></div></td>
            <td><div className="activity-frequency"><span>{item.request_count} req</span><small>{item.requests_per_second.toFixed(2)} req/s · {item.connections_per_second.toFixed(2)} conn/s</small></div></td>
            <td>{endpoint(item.observer_name, item.observer_ip)}</td>
          </tr>)}
          {activity.length === 0 && <tr><td colSpan={7} className="activity-empty">Waiting for internal VM activity…</td></tr>}
        </tbody>
      </table>
    </div>
  </section>
}
