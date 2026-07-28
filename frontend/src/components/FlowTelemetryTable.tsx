import { useEffect, useMemo, useRef, useState } from 'react'
import type { Flow } from '../types/flow'
import type { GraphData, GraphEdge } from '../types/graph'
import { isConnectionFlow, isRequestFlow } from '../utils/flowFilters'
import { formatUTCClock } from '../utils/time'
import { formatBytes } from './StatCards'

export type FlowTableMode = 'connection' | 'request' | 'l4'
type RowSeverity = 'normal' | 'warning' | 'error'

const slowRTTThresholdMs = positiveNumberEnv(import.meta.env.VITE_SLOW_RTT_THRESHOLD_MS, 100)

function positiveNumberEnv(raw: unknown, fallback: number) {
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function endpoint(role: string, ip: string, detail: string) {
  return <span className="activity-endpoint">
    <em>{role}</em>
    <strong>{ip}</strong>
    <small>{detail}</small>
  </span>
}

function metric(label: string, value: string) {
  return <span><strong>{value}</strong><small>{label}</small></span>
}

function tableTitle(mode: FlowTableMode) {
  switch (mode) {
    case 'connection':
      return ['CONNECTION FLOW', 'TC/eBPF connectivity evidence used for the idle topology line'] as const
    case 'request':
      return ['REQUEST FLOW', 'TC/eBPF request/attempt evidence used for moving topology traffic'] as const
    case 'l4':
      return ['L4 FLOW', 'Raw network flow aggregates received from the VM TC/eBPF tracker'] as const
  }
}

function tableGuide(mode: FlowTableMode) {
  switch (mode) {
    case 'connection':
      return [
        ['Source', 'Captured by the agent TC/eBPF program on the VM NIC.'],
        ['Map behavior', 'Successful connectivity keeps one green idle line alive.'],
        ['Debug signal', 'FAILED means RST/error attempt; SLOW RTT comes from reachability probe evidence.'],
      ] as const
    case 'request':
      return [
        ['Source', 'Derived from connect/request counters emitted by the eBPF tracker.'],
        ['Map behavior', 'New request rows animate the same VM-to-VM edge for a short TTL.'],
        ['Boundary', 'No HTTP method/path/status here; that requires app logs or OpenTelemetry later.'],
      ] as const
    case 'l4':
      return [
        ['Source', 'Raw aggregate from network_flows, not external telemetry.'],
        ['Use for', 'Ports, bytes, packets, connections, request attempts, and error counters.'],
        ['Noise control', 'Tunnel/probe/control ports are excluded from the main graph by config.'],
      ] as const
  }
}

function flowKey(item: Flow) {
  return `${item.id}:${item.observed_at || item.last_seen}:${item.src_ip}:${item.dst_ip}:${item.dst_port}:${item.protocol}`
}

function edgeKey(edge: GraphEdge) {
  return `${edge.source}->${edge.target}:${edge.protocol}:${edge.dst_port}`
}

function reverseEdgeKey(edge: GraphEdge) {
  return `${edge.target}->${edge.source}:${edge.protocol}:${edge.dst_port}`
}

function buildEdgeLookup(graph: GraphData) {
  const lookup = new Map<string, GraphEdge>()
  graph.edges.forEach((edge) => {
    lookup.set(edgeKey(edge), edge)
    lookup.set(reverseEdgeKey(edge), edge)
  })
  return lookup
}

function edgeForFlow(flow: Flow, lookup: Map<string, GraphEdge>) {
  if (!flow.src_vm_id || !flow.dst_vm_id) return undefined
  return lookup.get(`${flow.src_vm_id}->${flow.dst_vm_id}:${flow.protocol}:${flow.dst_port}`)
    || lookup.get(`${flow.dst_vm_id}->${flow.src_vm_id}:${flow.protocol}:${flow.dst_port}`)
    || lookup.get(`${flow.src_vm_id}->${flow.dst_vm_id}:${flow.protocol}:0`)
    || lookup.get(`${flow.dst_vm_id}->${flow.src_vm_id}:${flow.protocol}:0`)
}

function severity(flow: Flow, edge?: GraphEdge): RowSeverity {
  if ((flow.error_count ?? 0) > 0 || edge?.failed) return 'error'
  if ((edge?.avg_rtt_ms ?? 0) >= slowRTTThresholdMs) return 'warning'
  return 'normal'
}

function signal(flow: Flow, rowSeverity: RowSeverity) {
  if (rowSeverity === 'error') return 'FAILED'
  if (rowSeverity === 'warning') return 'SLOW RTT'
  if ((flow.request_count ?? 0) > 0) return 'REQUEST'
  if ((flow.connection_count ?? 0) > 0) return 'CONNECTED'
  return 'OBSERVED'
}

function rowClassName(fresh: boolean, rowSeverity: RowSeverity) {
  const classes: string[] = []
  if (fresh) classes.push('log-row-fresh')
  if (rowSeverity === 'warning') classes.push('log-row-warning')
  if (rowSeverity === 'error') classes.push('log-row-error')
  return classes.length ? classes.join(' ') : undefined
}

function signalBadge(rowSeverity: RowSeverity, label: string) {
  return <span className={`severity-chip severity-${rowSeverity}`}>{label}</span>
}

function modeRows(flows: Flow[], mode: FlowTableMode) {
  switch (mode) {
    case 'connection':
      return flows.filter(isConnectionFlow)
    case 'request':
      return flows.filter(isRequestFlow)
    case 'l4':
      return flows
  }
}

export function FlowTelemetryTable({
  flows,
  graph,
  mode,
}: {
  flows: Flow[]
  graph: GraphData
  mode: FlowTableMode
}) {
  const edgeLookup = useMemo(() => buildEdgeLookup(graph), [graph])
  const rows = useMemo(() => modeRows(flows, mode).slice(0, 200), [flows, mode])
  const [title, subtitle] = tableTitle(mode)
  const guide = tableGuide(mode)
  const signature = useMemo(() => rows.slice(0, 5).map(flowKey).join('|'), [rows])
  const previousSignature = useRef(signature)
  const [fresh, setFresh] = useState(false)
  const visibleRowKeys = useMemo(() => rows.slice(0, 20).map(flowKey), [rows])
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

  return <section className={`activity-card telemetry-card${fresh ? ' table-fresh' : ''}`}>
    <div className="activity-heading">
      <div>
        <small>{title}</small>
        <span>{subtitle}</span>
      </div>
      <div className="telemetry-heading-meta">
        <small>{rows.length}/{flows.length} rows</small>
        <span className="telemetry-state telemetry-connected">ebpf</span>
      </div>
    </div>
    <div className="table-guide" aria-label={`${title} guide`}>
      {guide.map(([label, value]) => <span key={label}>
        <strong>{label}</strong>
        <small>{value}</small>
      </span>)}
    </div>
    <div className="activity-table-wrap">
      <table className="activity-table telemetry-table">
        <thead><tr><th>Signal</th><th>Observed UTC</th><th>Source → Destination</th><th>Protocol</th><th>Traffic bytes</th><th>Counters</th><th>Health evidence</th><th>Agent</th></tr></thead>
        <tbody>
          {rows.map((item, index) => {
            const edge = edgeForFlow(item, edgeLookup)
            const rowSeverity = severity(item, edge)
            const key = flowKey(item)
            const observedAt = item.observed_at || item.last_seen
            const rtt = edge?.avg_rtt_ms ?? 0
            return <tr key={`${item.id}-${index}`} className={rowClassName(freshRows.has(key), rowSeverity)}>
              <td>{signalBadge(rowSeverity, signal(item, rowSeverity))}</td>
              <td className="activity-time">{formatUTCClock(observedAt)}</td>
              <td><div className="activity-route">
                {endpoint('source', item.src_ip, `port ${item.src_port || '—'}`)}
                <span className="activity-arrow">→</span>
                {endpoint('destination', item.dst_ip, `port ${item.dst_port || '—'}`)}
              </div></td>
              <td><span className="protocol-pill">{item.protocol || 'L4'}</span><small className="direction-label">{item.scope}</small></td>
              <td><div className="metric-stack">
                {metric('sent', formatBytes(item.bytes_sent))}
                {metric('received', formatBytes(item.bytes_received))}
                {metric('total', formatBytes(item.bytes_sent + item.bytes_received))}
              </div></td>
              <td><div className="metric-stack">
                {metric('connections', `${item.connection_count ?? 0}`)}
                {metric('requests', `${item.request_count ?? 0}`)}
                {metric('errors', `${item.error_count ?? 0}`)}
              </div></td>
              <td><div className="metric-stack">
                {metric('rtt', rtt > 0 ? `${rtt.toFixed(2)} ms` : '—')}
                {metric('packets', `${item.packets}`)}
                {metric('state', edge?.failed ? 'failed' : edge?.reachable ? 'reachable' : item.error_count ? 'failed' : 'observed')}
              </div></td>
              <td><div className="metric-stack compact">
                {metric('agent id', item.agent_id || '—')}
                {metric('interface', item.interface_name || '—')}
              </div></td>
            </tr>
          })}
          {rows.length === 0 && <tr><td colSpan={8} className="activity-empty">Waiting for TC/eBPF {mode} telemetry…</td></tr>}
        </tbody>
      </table>
    </div>
  </section>
}
