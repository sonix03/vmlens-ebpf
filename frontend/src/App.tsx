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
      setGraph({ nodes: [], edges: [] })
      setSelectedNode(undefined)
      setError(reason instanceof Error ? reason.message : 'Unable to load VMLens data')
    } finally { setLoading(false) }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => connectRealtime(() => {
    window.clearTimeout(debounce.current)
    debounce.current = window.setTimeout(() => void load(), 300)
  }, setConnected), [load])

  useEffect(() => {
    if (selectedNode && !graph.nodes.some((node) => node.id === selectedNode.id)) {
      setSelectedNode(undefined)
    }
  }, [graph.nodes, selectedNode])

  const vmCount = graph.nodes.filter((node) => node.type === 'vm').length
  const vmIDs = new Set(graph.nodes.filter((node) => node.type === 'vm').map((node) => node.id))
  const relationshipCount = new Set(
    graph.edges
      .filter((edge) => vmIDs.has(edge.source) && vmIDs.has(edge.target) && edge.source !== edge.target)
      .map((edge) => [edge.source, edge.target].sort().join('<->')),
  ).size

  return <main className="app-shell">
    <header className="app-header">
      <div className="brand"><div className="brand-mark">V</div><div><h1>vmlens</h1></div></div>
      <div className="live-state"><i className={connected ? 'connected' : ''} /><span>{connected ? 'Realtime connected' : 'Realtime reconnecting'}</span></div>
    </header>
    {error && <div className="error-banner"><strong>Backend unavailable</strong><span>{error}</span></div>}
    <section className="workspace simple">
      <div className="graph-card">
        <div className="graph-heading">
          <div><small>VM TOPOLOGY</small></div>
          <div className="legend"><span className="vm-dot">Virtual machine</span><span className="edge-line">Communication</span></div>
        </div>
        {loading ? <div className="loading">Loading live topology...</div> : <GraphView graph={graph} onNodeSelect={setSelectedNode} />}
      </div>
      {selectedNode && <NodeDetailsPanel node={selectedNode} onClose={() => setSelectedNode(undefined)} />}
    </section>
    <footer><span></span><span>{vmCount} VMs · {relationshipCount} relationships</span></footer>
  </main>
}
