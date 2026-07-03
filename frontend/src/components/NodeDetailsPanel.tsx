import type { GraphNode } from '../types/graph'
import { formatBytes } from './StatCards'
import { StatusBadge } from './StatusBadge'

export function NodeDetailsPanel({ node, onClose }: { node: GraphNode; onClose: () => void }) {
  return <aside className="detail-panel">
    <button className="close" onClick={onClose}>×</button>
    <small className="eyebrow">VIRTUAL MACHINE</small>
    <h2>{node.label}</h2>
    <StatusBadge status={node.status} />
    <dl>
      <dt>VM ID</dt><dd>{node.id}</dd>
      <dt>Primary IP</dt><dd>{node.ip || '—'}</dd>
      <dt>Tenant</dt><dd>{node.tenant_id || 'unassigned'}</dd>
      <dt>Role</dt><dd>{node.role || 'unassigned'}</dd>
      <dt>Traffic in</dt><dd>{formatBytes(node.traffic_in)}</dd>
      <dt>Traffic out</dt><dd>{formatBytes(node.traffic_out)}</dd>
    </dl>
  </aside>
}
