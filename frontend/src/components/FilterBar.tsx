import type { GraphFilters } from '../types/graph'

interface Props {
  filters: GraphFilters
  onChange: (filters: GraphFilters) => void
  onRefresh: () => void
}

export function FilterBar({ filters, onChange, onRefresh }: Props) {
  const update = (key: keyof GraphFilters, value: string) => onChange({ ...filters, [key]: value })
  return <section className="filter-bar">
    <label>Window<select value={filters.time_range} onChange={(event) => update('time_range', event.target.value)}>
      <option value="5m">5 minutes</option><option value="15m">15 minutes</option>
      <option value="1h">1 hour</option><option value="24h">24 hours</option>
    </select></label>
    <label>Scope<select value={filters.scope} onChange={(event) => update('scope', event.target.value)}>
      <option value="">All scopes</option><option value="internal_same_tenant">Same tenant</option>
      <option value="internal_cross_tenant">Cross tenant</option><option value="unknown_internal">Unknown internal</option>
      <option value="external_public">External public</option>
      <option value="external_private">External private</option>
    </select></label>
    <label>Protocol<select value={filters.protocol} onChange={(event) => update('protocol', event.target.value)}>
      <option value="">TCP + UDP</option><option value="tcp">TCP</option><option value="udp">UDP</option>
    </select></label>
    <label>Status<select value={filters.status} onChange={(event) => update('status', event.target.value)}>
      <option value="">Any status</option><option value="online">Online</option>
      <option value="stale">Stale</option><option value="offline">Offline</option>
    </select></label>
    <label>Port<input value={filters.port} inputMode="numeric" placeholder="443" onChange={(event) => update('port', event.target.value)} /></label>
    <label>Minimum bytes<input value={filters.min_bytes} inputMode="numeric" placeholder="100000" onChange={(event) => update('min_bytes', event.target.value)} /></label>
    <button type="button" onClick={onRefresh}>Refresh</button>
  </section>
}
