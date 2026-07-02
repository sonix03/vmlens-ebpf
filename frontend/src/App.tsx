import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api/client'
import { connectRealtime } from './api/realtime'
import { GraphView } from './components/GraphView'
import { NodeDetailsPanel } from './components/NodeDetailsPanel'
import type { GraphData, GraphFilters, GraphNode } from './types/graph'

const graphWindow: GraphFilters = {
  vm_id: '', scope: '', protocol: '', port: '', time_range: '24h', min_bytes: '', status: '',
}

export function App() {
  const [graph, setGraph] = useState<GraphData>({ nodes: [], edges: [] })
  const [selectedNode, setSelectedNode] = useState<GraphNode>()
  const [connected, setConnected] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const debounce = useRef<number>()

  const load = useCallback(async () => {
    try {
      const nextGraph = await api.graph(graphWindow)
      setGraph(nextGraph); setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to load VMLens data')
    } finally { setLoading(false) }
  }, [])

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
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <section className="workspace simple">
      <div className="graph-card">
        <div className="graph-heading">
          <div><small>RELATIONSHIP MAP</small><h2>VM communication graph</h2></div>
          <div className="legend"><span className="vm-dot">VM</span><span className="internal-dot">Unknown internal</span><span className="external-dot">External</span></div>
        </div>
        {loading ? <div className="loading">Loading live topology...</div> : <GraphView graph={graph} onNodeSelect={setSelectedNode} />}
      </div>
      {selectedNode && <NodeDetailsPanel node={selectedNode} onClose={() => setSelectedNode(undefined)} />}
    </section>
    <footer><span>No packet payloads. No HTTP bodies. Metadata only.</span><span>{graph.nodes.length} nodes - {graph.edges.length} edges</span></footer>
  </main>
}
