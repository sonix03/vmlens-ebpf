import type { GraphEdge } from '../types/graph'
import { formatBytes } from './StatCards'

export function EdgeDetailsPanel({ edge, onClose }: { edge: GraphEdge; onClose: () => void }) {
  return <aside className="detail-panel">
    <button className="close" onClick={onClose}>×</button>
    <small className="eyebrow">network relationship</small>
    <h2>{edge.protocol.toUpperCase()} :{edge.dst_port}</h2>
    <span className={`scope-pill scope-${edge.scope}`}>{edge.scope.replaceAll('_', ' ')}</span>
    <dl>
      <dt>Source</dt><dd>{edge.source}</dd>
      <dt>Target</dt><dd>{edge.target}</dd>
      <dt>Sent</dt><dd>{formatBytes(edge.bytes_sent)}</dd>
      <dt>Received</dt><dd>{formatBytes(edge.bytes_received)}</dd>
      <dt>Packets</dt><dd>{edge.packets.toLocaleString()}</dd>
      {edge.total_bytes !== undefined && <><dt>Total bytes</dt><dd>{formatBytes(edge.total_bytes)}</dd></>}
      {edge.error_count !== undefined && <><dt>Errors</dt><dd>{edge.error_count.toLocaleString()}</dd></>}
      {edge.avg_rtt_ms !== undefined && <><dt>Avg RTT</dt><dd>{edge.avg_rtt_ms.toFixed(2)} ms</dd></>}
      {edge.p95_rtt_ms !== undefined && <><dt>P95 RTT</dt><dd>{edge.p95_rtt_ms.toFixed(2)} ms</dd></>}
      {edge.avg_response_duration_ms !== undefined && <><dt>Avg response</dt><dd>{edge.avg_response_duration_ms.toFixed(2)} ms</dd></>}
      {edge.agent_ids?.length ? <><dt>DeepFlow agents</dt><dd>{edge.agent_ids.join(', ')}</dd></> : null}
      {edge.observation_points?.length ? <><dt>Tap sides</dt><dd>{edge.observation_points.join(', ')}</dd></> : null}
      <dt>Connections</dt><dd>{edge.connection_count.toLocaleString()}</dd>
      <dt>First seen</dt><dd>{new Date(edge.first_seen).toLocaleString()}</dd>
      <dt>Last seen</dt><dd>{new Date(edge.last_seen).toLocaleString()}</dd>
      <dt>Last observed</dt><dd>{new Date(edge.last_observed_at).toLocaleString()}</dd>
      <dt>Traffic state</dt><dd>{Date.parse(edge.active_until) > Date.now() ? 'active' : 'idle'}</dd>
      <dt>Weight</dt><dd>{edge.weight} / 5</dd>
    </dl>
  </aside>
}
