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

function VMIcon() {
  return <span className="vm-node-icon" aria-hidden="true">
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none">
      <rect x="4.25" y="4.25" width="15.5" height="6.5" rx="2" />
      <rect x="4.25" y="13.25" width="15.5" height="6.5" rx="2" />
      <path d="M8 7.5h.01M8 16.5h.01M11 7.5h5M11 16.5h5" />
    </svg>
  </span>
}

function NodeStatusIcon({ status }: { status: string }) {
  if (status === 'online') {
    return <span className="vm-node-status" aria-label="online">
      <svg viewBox="0 0 17 17" width="17" height="17" fill="none" aria-hidden="true">
        <path d="M14.063 4.5 6.73 11.833 3.396 8.5" />
      </svg>
    </span>
  }
  return <span className="vm-node-status-dot" aria-label={status} />
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
    const status = node.status || 'unknown'
    const color = statusColors[status] || statusColors.unknown
    const online = node.status === 'online'
    return {
      id: node.id,
      position: positionFor(index, vmNodes.length),
      className: `vm-node vm-node-${status}`,
      data: {
        label: <div
          data-testid={`node-${node.id}`}
          className="vm-node-content"
          title={`${node.label} · ${node.ip || 'no IP'} · ${status}`}
        >
          <VMIcon />
          <strong>{node.label}</strong>
          <NodeStatusIcon status={status} />
        </div>,
      },
      style: {
        background: '#252a30', border: `1px solid ${online ? color : `${color}80`}`, color: '#f1f3f5',
        borderRadius: 999, minWidth: 210,
        boxShadow: 'none',
        opacity: online ? 1 : 0.68,
        padding: '12px 24px',
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
