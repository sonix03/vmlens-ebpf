import type { TopTalker } from '../types/stats'
import { formatBytes } from './StatCards'

export function TopTalkers({ talkers }: { talkers: TopTalker[] }) {
  const max = Math.max(...talkers.map((item) => item.total_bytes), 1)
  return <section className="side-card top-talkers">
    <div className="section-title"><span>Top talkers</span><small>total observed</small></div>
    {talkers.length === 0 && <p className="empty">Waiting for flows…</p>}
    {talkers.map((talker) => <div className="talker" key={talker.vm_id}>
      <div><strong>{talker.name}</strong><span>{formatBytes(talker.total_bytes)}</span></div>
      <div className="bar"><i style={{ width: `${Math.max(4, talker.total_bytes / max * 100)}%` }} /></div>
    </div>)}
  </section>
}

