import { useEffect, useMemo, useRef, useState } from 'react'
import type { DeepFlowHealth, DeepFlowRawLogs } from '../types/deepflow'
import { isDeepFlowConnectionFlow, isDeepFlowRequestFlow } from '../utils/flowFilters'
import { formatUTCClock } from '../utils/time'
import { formatBytes } from './StatCards'

export type DeepFlowTableMode = 'connection' | 'request' | 'l4' | 'l7'

function timeLabel(value: string) {
  return formatUTCClock(value)
}

function healthState(health?: DeepFlowHealth) {
  if (!health) return 'waiting'
  if (!health.enabled) return 'disabled'
  if (health.errors?.length) return 'degraded'
  if (health.clickhouse_reachable && health.agent_list_not_empty) return 'connected'
  return 'waiting'
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

function tableTitle(mode: DeepFlowTableMode) {
  switch (mode) {
    case 'connection':
      return ['CONNECTION FLOW', 'Clean L4 connectivity rows used for static topology lines'] as const
    case 'request':
      return ['REQUEST FLOW', 'Clean L7 application request rows used for animated topology traffic'] as const
    case 'l4':
      return ['DEEPFLOW L4 FLOWS', 'Network flow rows after inventory filtering and tap-side dedupe'] as const
    case 'l7':
      return ['DEEPFLOW L7 REQUESTS', 'Application request rows after inventory filtering and tap-side dedupe'] as const
  }
}

function l4RowKey(item: DeepFlowRawLogs['l4'][number]) {
  return `${item.time}:${item.source_ip}:${item.dest_ip}:${item.client_port}:${item.server_port}:${item.protocol}:${item.agent_id}`
}

function l7RowKey(item: DeepFlowRawLogs['l7'][number]) {
  return `${item.time}:${item.source_ip}:${item.dest_ip}:${item.request_type}:${item.request_resource}:${item.response_code}:${item.agent_id}:${item.observation_point}`
}

export function DeepFlowFlowTable({
  raw,
  health,
  mode,
}: {
  raw?: DeepFlowRawLogs
  health?: DeepFlowHealth
  mode: DeepFlowTableMode
}) {
  const state = healthState(health)
  const rawL4Rows = Array.isArray(raw?.l4) ? raw.l4 : []
  const rawL7Rows = Array.isArray(raw?.l7) ? raw.l7 : []
  const l4Rows = (mode === 'connection' ? rawL4Rows.filter(isDeepFlowConnectionFlow) : rawL4Rows).slice(0, 200)
  const l7Rows = (mode === 'request' ? rawL7Rows.filter(isDeepFlowRequestFlow) : rawL7Rows).slice(0, 200)
  const isL4 = mode === 'l4' || mode === 'connection'
  const rows = isL4 ? l4Rows : l7Rows
  const [title, subtitle] = tableTitle(mode)
  const signature = useMemo(() => rows.slice(0, 5).map((item) => item.time).join('|'), [rows])
  const previousSignature = useRef(signature)
  const [fresh, setFresh] = useState(false)
  const visibleRowKeys = useMemo(() => rows.slice(0, 20).map((item) => isL4
    ? l4RowKey(item as DeepFlowRawLogs['l4'][number])
    : l7RowKey(item as DeepFlowRawLogs['l7'][number])), [isL4, rows])
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

  return <section className={`activity-card deepflow-card${fresh ? ' table-fresh' : ''}`}>
    <div className="activity-heading">
      <div>
        <small>{title}</small>
        <span>{subtitle}</span>
      </div>
      <div className="deepflow-heading-meta">
        {raw ? <small>{rows.length}/{raw.limit} rows · {raw.window}</small> : null}
        <span className={`deepflow-state deepflow-${state}`}>{state}</span>
      </div>
    </div>
    {health?.warnings?.length ? <div className="deepflow-note">{health.warnings.slice(0, 2).join(' · ')}</div> : null}
    {health?.errors?.length ? <div className="deepflow-note error">{health.errors.slice(0, 2).join(' · ')}</div> : null}
    <div className="activity-table-wrap">
      {isL4 ? <table className="activity-table deepflow-table">
        <thead><tr><th>CH Time</th><th>Client → Server</th><th>Protocol</th><th>Status</th><th>Tap bytes</th><th>Network delay</th><th>Agent</th></tr></thead>
        <tbody>
          {l4Rows.map((item, index) => {
            const key = l4RowKey(item)
            return <tr key={`l4-${item.time}-${index}`} className={freshRows.has(key) ? 'log-row-fresh' : undefined}>
            <td className="activity-time">{timeLabel(item.time)}</td>
            <td><div className="activity-route">
              {endpoint('client', item.source_ip, `port ${item.client_port}`)}
              <span className="activity-arrow">→</span>
              {endpoint('server', item.dest_ip, `port ${item.server_port}`)}
            </div></td>
            <td><span className="protocol-pill">{item.protocol || 'L4'}</span><small className="direction-label">{item.internet_direction}</small></td>
            <td><span className={item.status.toLowerCase().includes('error') ? 'deepflow-error-code' : 'deepflow-ok-code'}>{item.status || '—'}</span></td>
            <td><div className="metric-stack">
              {metric('tx at tap', formatBytes(item.byte_tx))}
              {metric('rx at tap', formatBytes(item.byte_rx))}
              {metric('total', formatBytes(item.total_bytes))}
            </div></td>
            <td><div className="metric-stack">
              {metric('rtt', `${item.rtt_ms.toFixed(2)} ms`)}
              {metric('retransmit', `${item.retrans_total}`)}
            </div></td>
            <td><div className="metric-stack compact"><span><strong>{item.agent_id || '—'}</strong><small>L4 agent id</small></span></div></td>
          </tr>
          })}
          {l4Rows.length === 0 && <tr><td colSpan={7} className="activity-empty">Waiting for {mode === 'connection' ? 'clean connection flow' : 'DeepFlow L4 telemetry'}…</td></tr>}
        </tbody>
      </table> : <table className="activity-table deepflow-table">
        <thead><tr><th>CH Time</th><th>Client → Server</th><th>Request</th><th>Response</th><th>Payload bytes</th><th>App delay</th><th>Agent / Tap point</th></tr></thead>
        <tbody>
          {l7Rows.map((item, index) => {
            const key = l7RowKey(item)
            return <tr key={`l7-${item.time}-${index}`} className={freshRows.has(key) ? 'log-row-fresh' : undefined}>
            <td className="activity-time">{timeLabel(item.time)}</td>
            <td><div className="activity-route">
              {endpoint('client', item.source_ip, item.request_domain || 'no-domain')}
              <span className="activity-arrow">→</span>
              {endpoint('server', item.dest_ip, item.request_resource || '/')}
            </div></td>
            <td><span className="protocol-pill">{item.l7_protocol_str || 'L7'}</span><small className="direction-label">{item.request_type}</small></td>
            <td><span className={item.response_code >= 400 ? 'deepflow-error-code' : 'deepflow-ok-code'}>{item.response_code || '—'}</span></td>
            <td><div className="metric-stack">
              {metric('request', formatBytes(item.request_length))}
              {metric('response', formatBytes(item.response_length))}
            </div></td>
            <td><div className="metric-stack">
              {metric('duration', `${item.response_duration_ms.toFixed(2)} ms`)}
              {metric('direction', item.internet_direction)}
            </div></td>
            <td><div className="metric-stack compact">
              {metric('agent id', item.agent_id || '—')}
              {metric('point', item.observation_point || '—')}
            </div></td>
          </tr>
          })}
          {l7Rows.length === 0 && <tr><td colSpan={7} className="activity-empty">Waiting for {mode === 'request' ? 'clean request flow' : 'DeepFlow L7 telemetry'}…</td></tr>}
        </tbody>
      </table>}
    </div>
  </section>
}
