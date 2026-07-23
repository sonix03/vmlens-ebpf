import { useEffect, useMemo, useRef, useState } from 'react'
import type { InternalActivity } from '../types/internalActivity'
import { formatUTCClock } from '../utils/time'
import { formatBytes } from './StatCards'

function endpoint(role: string, name: string, ip: string) {
  return <span className="activity-endpoint">
    <em>{role}</em>
    <strong>{name}</strong>
    <small>{ip}</small>
  </span>
}

function metric(label: string, value: string) {
  return <span><strong>{value}</strong><small>{label}</small></span>
}

function activityTime(value: string) {
  return formatUTCClock(value)
}

function rowKey(item: InternalActivity) {
  return `${item.id}:${item.direction}:${item.observed_at}`
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
  const signature = useMemo(() => activity.slice(0, 5).map((item) => `${item.id}:${item.observed_at}`).join('|'), [activity])
  const previousSignature = useRef(signature)
  const [fresh, setFresh] = useState(false)
  const visibleRowKeys = useMemo(() => activity.slice(0, 20).map(rowKey), [activity])
  const previousRowKeys = useRef(new Set(visibleRowKeys))
  const [freshRows, setFreshRows] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (previousSignature.current === signature) return
    previousSignature.current = signature
    setFresh(true)
    const timeout = window.setTimeout(() => setFresh(false), 650)
    return () => window.clearTimeout(timeout)
  }, [signature])

  useEffect(() => {
    const nextFreshRows = visibleRowKeys.filter((key) => !previousRowKeys.current.has(key))
    previousRowKeys.current = new Set(visibleRowKeys)
    if (nextFreshRows.length === 0) return
    setFreshRows(new Set(nextFreshRows))
    const timeout = window.setTimeout(() => setFreshRows(new Set()), 900)
    return () => window.clearTimeout(timeout)
  }, [visibleRowKeys])

  return <section className={`activity-card${fresh ? ' table-fresh' : ''}`}>
    <div className="activity-heading">
      <div><small>INTERNAL ACTIVITY</small><span>Latest registered VM-to-VM traffic from the VMLens agent</span></div>
      <span>{activity.length}/{limit} latest · {windowLabel}</span>
    </div>
    <div className="activity-table-wrap">
      <table className="activity-table">
        <thead><tr><th>Observed UTC</th><th>Client → Server</th><th>Service</th><th>Observer side</th><th>Bytes</th><th>Frequency</th><th>Captured by</th></tr></thead>
        <tbody>
          {activity.map((item) => {
            const key = rowKey(item)
            return <tr key={`${item.id}-${item.direction}`} className={freshRows.has(key) ? 'log-row-fresh' : undefined}>
            <td className="activity-time">{activityTime(item.observed_at)}</td>
            <td><div className="activity-route">
              {endpoint('client', item.source_name, item.source_ip)}
              <span className={`activity-arrow ${item.direction}`} aria-label={item.direction}>→</span>
              {endpoint('server', item.destination_name, item.destination_ip)}
            </div></td>
            <td><span className="service-pill">{item.service}</span><small className="service-port">:{item.service_port}</small></td>
            <td><span className="protocol-pill">{item.protocol}</span><small className="direction-label">agent {item.direction}</small></td>
            <td><div className="metric-stack">
              {metric('client → server', formatBytes(item.bytes_sent))}
              {metric('server → client', formatBytes(item.bytes_received))}
            </div></td>
            <td><div className="metric-stack">
              {metric('requests', `${item.request_count}`)}
              {metric('rate', `${item.requests_per_second.toFixed(2)} req/s`)}
              {metric('connections', `${item.connections_per_second.toFixed(2)} conn/s`)}
            </div></td>
            <td>{endpoint('observer', item.observer_name, item.observer_ip)}</td>
          </tr>
          })}
          {activity.length === 0 && <tr><td colSpan={7} className="activity-empty">Waiting for internal VM activity…</td></tr>}
        </tbody>
      </table>
    </div>
  </section>
}
