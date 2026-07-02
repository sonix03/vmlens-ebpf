import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api/client'
import { connectRealtime } from './api/realtime'
import { EdgeDetailsPanel } from './components/EdgeDetailsPanel'
import { FilterBar } from './components/FilterBar'
import { GraphView } from './components/GraphView'
import { NodeDetailsPanel } from './components/NodeDetailsPanel'
import { StatCards } from './components/StatCards'
import { TopTalkers } from './components/TopTalkers'
import type { GraphData, GraphEdge, GraphFilters, GraphNode } from './types/graph'
import type { Summary, TopTalker } from './types/stats'

const initialFilters: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '15m', min_bytes: '', status: '',
}

export function App() {
  const [graph, setGraph] = useState<GraphData>({ nodes: [], edges: [] })
  const [summary, setSummary] = useState<Summary>()
  const [talkers, setTalkers] = useState<TopTalker[]>([])
  const [filters, setFilters] = useState(initialFilters)
  const [selectedNode, setSelectedNode] = useState<GraphNode>()
  const [selectedEdge, setSelectedEdge] = useState<GraphEdge>()
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const debounce = useRef<number>()

  const load = useCallback(async () => {
    try {
      const [nextGraph, nextSummary, nextTalkers] = await Promise.all([
        api.graph(filters), api.summary(), api.topTalkers(),
      ])
      setGraph(nextGraph); setSummary(nextSummary); setTalkers(nextTalkers); setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to load VMLens data')
    } finally { setLoading(false) }
  }, [filters])

  useEffect(() => { void load() }, [load])
  useEffect(() => connectRealtime(() => {
    window.clearTimeout(debounce.current)
    debounce.current = window.setTimeout(() => void load(), 300)
  }, setConnected), [load])

  return <main className="app-shell">
    <header className="app-header">
      <div className="brand"><div className="brand-mark">V</div><div><h1>VMLens</h1><span>Live network relationships</span></div></div>
      <div className="live-state"><i className={connected ? 'connected' : ''} /><span>{connected ? 'Realtime connected' : 'Realtime reconnecting'}</span></div>
    </header>
    <StatCards summary={summary} />
    <FilterBar filters={filters} onChange={setFilters} onRefresh={() => void load()} />
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <section className="workspace">
      <div className="graph-card">
        <div className="graph-heading">
          <div><small>RELATIONSHIP MAP</small><h2>VM communication graph</h2></div>
          <div className="legend"><span className="vm-dot">VM</span><span className="internal-dot">Unknown internal</span><span className="external-dot">External</span></div>
        </div>
        {loading ? <div className="loading">Loading live topology...</div> : <GraphView graph={graph} onNodeSelect={(node) => { setSelectedNode(node); setSelectedEdge(undefined) }} onEdgeSelect={(edge) => { setSelectedEdge(edge); setSelectedNode(undefined) }} />}
      </div>
      <TopTalkers talkers={talkers} />
      {selectedNode && <NodeDetailsPanel node={selectedNode} onClose={() => setSelectedNode(undefined)} />}
      {selectedEdge && <EdgeDetailsPanel edge={selectedEdge} onClose={() => setSelectedEdge(undefined)} />}
    </section>
    <footer><span>No packet payloads. No HTTP bodies. Metadata only.</span><span>{graph.nodes.length} nodes - {graph.edges.length} edges</span></footer>
  </main>
}
