import { useEffect, useMemo, useRef, useState } from 'react'
import type { GraphData } from '../types/graph'
import type { InternalActivity } from '../types/internalActivity'
import { formatUTCClock } from '../utils/time'
import { formatBytes } from './StatCards'

const slowRTTThresholdMs = positiveNumberEnv(import.meta.env.VITE_SLOW_RTT_THRESHOLD_MS, 100)

function positiveNumberEnv(raw: unknown, fallback: number) {
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

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

const ephemeralPortMin = 32768

// service_port is the server side. The client side is whichever of the two
// observed ports it is not.
function clientPort(item: InternalActivity) {
  return item.service_port === item.local_port ? item.peer_port : item.local_port
}

// An ephemeral service_port means the backend had no known service to anchor
// on, so the client/server split is inferred rather than observed.
function portResolved(item: InternalActivity) {
  return item.service_port > 0 && item.service_port < ephemeralPortMin
}

function portBox(value: number, resolved: boolean) {
  if (!value) return <span className="port-box port-box-empty">—</span>
  return <span className={`port-box${resolved ? '' : ' port-box-guess'}`} title={resolved ? undefined : 'Both ends are ephemeral; the client/server split is inferred, not observed.'}>{value}</span>
}

function rowKey(item: InternalActivity) {
  return `${item.id}:${item.direction}:${item.observed_at}`
}

function pairKey(a?: string, b?: string) {
  if (!a || !b) return ''
  return [a, b].sort().join('<->')
}

function buildRTTLookup(graph: GraphData) {
  const lookup = new Map<string, number>()
  graph.edges.forEach((edge) => {
    const rtt = edge.avg_rtt_ms ?? 0
    if (rtt <= 0) return
    const vmKey = pairKey(edge.source, edge.target)
    if (vmKey) lookup.set(vmKey, Math.max(lookup.get(vmKey) ?? 0, rtt))
    const ipKey = pairKey(edge.source_ip, edge.dest_ip)
    if (ipKey) lookup.set(ipKey, Math.max(lookup.get(ipKey) ?? 0, rtt))
  })
  return lookup
}

function rttForActivity(item: InternalActivity, lookup: Map<string, number>) {
  return lookup.get(pairKey(item.source_vm_id, item.destination_vm_id))
    ?? lookup.get(pairKey(item.source_ip, item.destination_ip))
    ?? 0
}

function rowClassName(item: InternalActivity, rtt: number, fresh: boolean) {
  const classes: string[] = []
  if (fresh) classes.push('log-row-fresh')
  if (item.error_count > 0) classes.push('log-row-error')
  else if (rtt >= slowRTTThresholdMs) classes.push('log-row-warning')
  return classes.length ? classes.join(' ') : undefined
}

export function InternalActivityTable({
  activity,
  graph,
  windowLabel,
  limit,
}: {
  activity: InternalActivity[]
  graph: GraphData
  windowLabel: string
  limit: number
}) {
  const rttLookup = useMemo(() => buildRTTLookup(graph), [graph])
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
    <div className="table-guide" aria-label="Internal activity guide">
      <span>
        <strong>Purpose</strong>
        <small>Fast local signal from VMLens agent for VM-to-VM activity.</small>
      </span>
      <span>
        <strong>Map behavior</strong>
        <small>Used to refresh topology from direct TC/eBPF agent telemetry.</small>
      </span>
      <span>
        <strong>Debug signal</strong>
        <small>Errors turn rows red; RTT above {slowRTTThresholdMs} ms turns rows yellow.</small>
      </span>
    </div>
    <div className="activity-table-wrap">
      <table className="activity-table">
        <thead>
          <tr className="column-groups">
            <th colSpan={6}>Connection</th>
            <th colSpan={3}>Performance</th>
            <th>Evidence</th>
          </tr>
          <tr className="column-labels">
            <th>Observed UTC</th>
            <th>Client → Server</th>
            <th>Service</th>
            <th className="numeric">Client port</th>
            <th className="numeric">Server port</th>
            <th>Observer side</th>
            <th>RTT (ms)</th>
            <th>Bytes</th>
            <th>Frequency</th>
            <th>Captured by</th>
          </tr>
        </thead>
        <tbody>
          {activity.map((item) => {
            const key = rowKey(item)
            const rtt = rttForActivity(item, rttLookup)
            return <tr key={`${item.id}-${item.direction}`} className={rowClassName(item, rtt, freshRows.has(key))}>
            <td className="activity-time">{activityTime(item.observed_at)}</td>
            <td><div className="activity-route">
              {endpoint('client', item.source_name, item.source_ip)}
              <span className={`activity-arrow ${item.direction}`} aria-label={item.direction}>→</span>
              {endpoint('server', item.destination_name, item.destination_ip)}
            </div></td>
            <td><span className="service-pill">{item.service}</span></td>
            <td className="port-cell">{portBox(clientPort(item), portResolved(item))}</td>
            <td className="port-cell">{portBox(item.service_port, portResolved(item))}</td>
            <td><span className="protocol-pill">{item.protocol}</span><small className="direction-label">agent {item.direction}</small></td>
            <td><div className="metric-stack compact">
              <span className={`rtt-value${rtt >= slowRTTThresholdMs ? ' rtt-slow' : ''}`}><strong>{rtt > 0 ? rtt.toFixed(2) : '—'}</strong><small>threshold {slowRTTThresholdMs} ms</small></span>
            </div></td>
            <td><div className="metric-stack">
              {metric('client → server', formatBytes(item.bytes_sent))}
              {metric('server → client', formatBytes(item.bytes_received))}
            </div></td>
            <td><div className="metric-stack">
              {metric('requests', `${item.request_count}`)}
              {metric('errors', `${item.error_count}`)}
              {metric('rate', `${item.requests_per_second.toFixed(2)} req/s`)}
              {metric('connections', `${item.connections_per_second.toFixed(2)} conn/s`)}
            </div></td>
            <td>{endpoint('observer', item.observer_name, item.observer_ip)}</td>
          </tr>
          })}
          {activity.length === 0 && <tr><td colSpan={10} className="activity-empty">Waiting for internal VM activity…</td></tr>}
        </tbody>
      </table>
    </div>
  </section>
}
