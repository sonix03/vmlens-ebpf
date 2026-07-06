import { useEffect, useMemo, useState } from 'react'
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
  online: '#55a979', stale: '#c5964b', offline: '#6b7280', unknown: '#8b949e',
}

function positionFor(index: number, count: number) {
  if (count === 1) return { x: 0, y: 0 }
  const angle = (index / count) * Math.PI * 2 - Math.PI / 2
  const radius = Math.max(210, count * 30)
  return { x: Math.cos(angle) * radius, y: Math.sin(angle) * radius }
}

export function GraphView({ graph, onNodeSelect }: Props) {
  const [clock, setClock] = useState(() => Date.now())

  useEffect(() => {
    const interval = window.setInterval(() => setClock(Date.now()), 250)
    return () => window.clearInterval(interval)
  }, [])

  // The main topology is VM-only. Unknown/private and public endpoints remain
  // available through the API without becoming graph nodes.
  const vmNodes = useMemo(() => graph.nodes.filter((node) => node.type === 'vm'), [graph.nodes])
  const vmIDs = useMemo(() => new Set(vmNodes.map((node) => node.id)), [vmNodes])

  const nodes = useMemo<Node[]>(() => vmNodes.map((node, index) => {
    const color = statusColors[node.status || 'unknown'] || statusColors.unknown
    const online = node.status === 'online'
    return {
      id: node.id,
      position: positionFor(index, vmNodes.length),
      data: {
        label: <div className="node-label"><i style={{ background: color }} /><strong>{node.label}</strong></div>,
      },
      style: {
        background: '#151a20', border: `1px solid ${online ? color : `${color}66`}`, color: '#e5e7eb',
        borderRadius: 5, minWidth: 168,
        boxShadow: 'none',
        opacity: online ? 1 : 0.58,
        padding: '12px 14px',
      },
    }
  }), [vmNodes])

  const edges = useMemo<Edge[]>(() => {
    // Several ports/protocols between the same VM pair become one visual edge.
    // Individual aggregated flows remain available in the backend.
    const relationships = new Map<string, { source: string; target: string; weight: number; activeUntil: number }>()
    graph.edges.forEach((edge) => {
      if (!vmIDs.has(edge.source) || !vmIDs.has(edge.target)) return
      if (edge.source === edge.target) return
      const [source, target] = [edge.source, edge.target].sort()
      const key = `${source}<->${target}`
      const parsedActiveUntil = Date.parse(edge.active_until)
      const activeUntil = Number.isFinite(parsedActiveUntil)
        ? parsedActiveUntil
        : (edge.active ? Date.parse(edge.last_observed_at) + 3000 : 0)
      const current = relationships.get(key) || { source, target, weight: 1, activeUntil: 0 }
      current.weight = Math.max(current.weight, edge.weight)
      current.activeUntil = Math.max(current.activeUntil, activeUntil)
      relationships.set(key, current)
    })

    return Array.from(relationships.entries()).map(([id, relationship]) => {
      const active = relationship.activeUntil > clock
      const color = active ? '#6fa88b' : '#4a515a'
      return {
        id,
        source: relationship.source,
        target: relationship.target,
        animated: active,
        className: active ? 'traffic-active' : 'traffic-idle',
        markerStart: { type: MarkerType.ArrowClosed, color },
        markerEnd: { type: MarkerType.ArrowClosed, color },
        style: {
          stroke: color,
          strokeWidth: active ? Math.min(4, 1.25 + relationship.weight * 0.4) : 1,
          opacity: active ? 0.9 : 0.3,
        },
      }
    })
  }, [clock, graph.edges, vmIDs])

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
      <Background variant={BackgroundVariant.Dots} gap={24} size={1} color="#2b3138" />
      <Controls position="bottom-left" />
    </ReactFlow>
  </div>
}
