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
  online: '#32ff75', stale: '#f59e0b', offline: '#64748b', unknown: '#94a3b8',
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

  // Only registered VMs belong on the main topology. Destination IPs and other
  // metadata remain available from the backend without becoming graph nodes.
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
        background: '#101927', border: online ? `2px solid ${color}` : `1px solid ${color}66`, color: '#e5edf7',
        borderRadius: 12, minWidth: 170,
        boxShadow: online ? `0 0 14px ${color}cc, 0 0 34px ${color}66` : '0 12px 35px #0008',
        opacity: online ? 1 : 0.62,
        padding: '14px 16px',
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
      const color = active ? '#5eead4' : '#475569'
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
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="#26354a" />
      <Controls position="bottom-left" />
    </ReactFlow>
  </div>
}
