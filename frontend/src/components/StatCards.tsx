import type { Summary } from '../types/stats'

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

export function StatCards({ summary }: { summary?: Summary }) {
  const cards = [
    ['VMs', summary?.total_vms ?? 0, `${summary?.online_vms ?? 0} online`],
    ['Internal VM traffic', formatBytes(summary?.internal_bytes ?? 0), `↑ ${formatBytes(summary?.internal_sent_bytes ?? 0)} · ↓ ${formatBytes(summary?.internal_received_bytes ?? 0)}`],
    ['External traffic', formatBytes(summary?.external_bytes ?? 0), `↑ ${formatBytes(summary?.external_sent_bytes ?? 0)} · ↓ ${formatBytes(summary?.external_received_bytes ?? 0)}`],
    ['Request frequency', `${(summary?.network_requests_per_second ?? 0).toFixed(2)} req/s`, `${summary?.network_requests_last_minute ?? 0} last 60s · ${(summary?.network_connections_per_second ?? 0).toFixed(2)} conn/s`],
    ['Unknown internal', summary?.unknown_internal_hosts ?? 0, 'awaiting agent'],
  ]
  return <section className="stat-grid">
    {cards.map(([label, value, note]) => <article className="stat-card" key={label}>
      <span>{label}</span><strong>{value}</strong><small>{note}</small>
    </article>)}
  </section>
}
