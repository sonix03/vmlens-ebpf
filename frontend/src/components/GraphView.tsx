import { useMemo } from 'react'
import {
  Background, BackgroundVariant, Controls, MarkerType, ReactFlow,
  type Edge, type Node,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { GraphData, GraphNode } from '../types/graph'

interface Props {
  graph: GraphData
  onNodeSelect: (node: GraphNode) => void
}

const statusColors: Record<string, string> = {
  online: '#34d399', stale: '#f59e0b', offline: '#64748b', unknown: '#94a3b8',
}

function positionFor(index: number, count: number) {
  if (count === 1) return { x: 0, y: 0 }
  const angle = (index / count) * Math.PI * 2 - Math.PI / 2
  const radius = Math.max(210, count * 30)
  return { x: Math.cos(angle) * radius, y: Math.sin(angle) * radius }
}

export function GraphView({ graph, onNodeSelect }: Props) {
  // Only registered VMs belong on the main topology. Destination IPs and other
  // metadata remain available from the backend without becoming graph nodes.
  const vmNodes = useMemo(() => graph.nodes.filter((node) => node.type === 'vm'), [graph.nodes])
  const vmIDs = useMemo(() => new Set(vmNodes.map((node) => node.id)), [vmNodes])

  const nodes = useMemo<Node[]>(() => vmNodes.map((node, index) => {
    const color = statusColors[node.status || 'unknown'] || statusColors.unknown
    return {
      id: node.id,
      position: positionFor(index, vmNodes.length),
      data: {
        label: <div className="node-label"><i style={{ background: color }} /><strong>{node.label}</strong></div>,
      },
      style: {
        background: '#101927', border: `1px solid ${color}99`, color: '#e5edf7',
        borderRadius: 12, minWidth: 170,
        boxShadow: `0 12px 35px #0008, 0 0 20px ${color}12`, padding: '14px 16px',
      },
    }
  }), [vmNodes])

  const edges = useMemo<Edge[]>(() => {
    // Several ports/protocols between the same VM pair become one visual edge.
    // Individual aggregated flows remain available in the backend.
    const relationships = new Map<string, { source: string; target: string; weight: number }>()
    graph.edges.forEach((edge) => {
      if (!vmIDs.has(edge.source) || !vmIDs.has(edge.target)) return
	  if (edge.source === edge.target) return
	  const [source, target] = [edge.source, edge.target].sort()
	  const key = `${source}<->${target}`
	  const current = relationships.get(key) || { source, target, weight: 1 }
      current.weight = Math.max(current.weight, edge.weight)
      relationships.set(key, current)
    })

    return Array.from(relationships.entries()).map(([id, relationship]) => ({
      id,
      source: relationship.source,
      target: relationship.target,
      animated: true,
	  markerStart: { type: MarkerType.ArrowClosed, color: '#5eead4' },
      markerEnd: { type: MarkerType.ArrowClosed, color: '#5eead4' },
      style: {
        stroke: '#5eead4',
        strokeWidth: Math.min(4, 1.25 + relationship.weight * 0.4),
        opacity: 0.72,
      },
    }))
  }, [graph.edges, vmIDs])

  return <div className="graph-canvas">
    {vmNodes.length === 0 && <div className="graph-empty"><strong>Waiting for VM</strong><span></span></div>}
    <ReactFlow
      nodes={nodes}
      edges={edges}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      minZoom={0.15}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      onNodeClick={(_, node) => {
        const original = vmNodes.find((item) => item.id === node.id)
        if (original) onNodeSelect(original)
      }}
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="#26354a" />
      <Controls position="bottom-left" />
    </ReactFlow>
  </div>
}
