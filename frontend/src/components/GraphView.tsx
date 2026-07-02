import { useMemo } from 'react'
import {
  Background, BackgroundVariant, Controls, MarkerType, MiniMap, ReactFlow,
  type Edge, type Node,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { GraphData, GraphNode } from '../types/graph'
import { formatBytes } from './StatCards'

interface Props {
  graph: GraphData
  onNodeSelect: (node: GraphNode) => void
}

const nodeColors: Record<string, string> = {
  vm: '#34d399', unknown_internal: '#f59e0b', external: '#60a5fa', unknown: '#94a3b8',
}
const scopeColors: Record<string, string> = {
  internal_same_tenant: '#34d399', internal_cross_tenant: '#a78bfa',
  unknown_internal: '#f59e0b', external_public: '#60a5fa', unknown: '#94a3b8',
}

export function GraphView({ graph, onNodeSelect }: Props) {
  const nodes = useMemo<Node[]>(() => graph.nodes.map((node, index) => {
    const angle = (index / Math.max(graph.nodes.length, 1)) * Math.PI * 2
    const radius = Math.max(220, graph.nodes.length * 24)
    const color = nodeColors[node.type] || nodeColors.unknown
    return {
      id: node.id,
      position: { x: Math.cos(angle) * radius + radius, y: Math.sin(angle) * radius + radius },
      data: { label: <div className="node-label"><strong>{node.label}</strong><span>{node.ip || node.type}</span></div> },
      style: {
        background: '#101927', border: `2px solid ${color}`, color: '#e5edf7',
        borderRadius: node.type === 'external' ? 28 : 12, minWidth: 142,
        boxShadow: `0 0 24px ${color}22`, padding: '11px 14px',
      },
    }
  }), [graph.nodes])

  const edges = useMemo<Edge[]>(() => graph.edges.map((edge) => {
    const color = scopeColors[edge.scope] || scopeColors.unknown
    return {
      id: edge.id, source: edge.source, target: edge.target,
      label: `${edge.protocol.toUpperCase()}:${edge.dst_port} · ${formatBytes(edge.bytes_sent + edge.bytes_received)}`,
      animated: edge.weight >= 4,
      markerEnd: { type: MarkerType.ArrowClosed, color },
      style: { stroke: color, strokeWidth: 1 + edge.weight * 0.65 },
      labelStyle: { fill: '#b9c8da', fontSize: 11 },
      labelBgStyle: { fill: '#0b1220', fillOpacity: 0.9 },
    }
  }), [graph.edges])

  return <div className="graph-canvas">
    {graph.nodes.length === 0 && <div className="graph-empty"><strong>Waiting for agents</strong><span>Nodes appear automatically after registration.</span></div>}
    <ReactFlow
      nodes={nodes} edges={edges} fitView fitViewOptions={{ padding: 0.2 }} minZoom={0.15}
      onNodeClick={(_, node) => { const original = graph.nodes.find((item) => item.id === node.id); if (original) onNodeSelect(original) }}
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="#26354a" />
      <Controls position="bottom-left" />
      <MiniMap nodeColor={(node) => graph.nodes.find((item) => item.id === node.id) ? nodeColors[graph.nodes.find((item) => item.id === node.id)!.type] : '#64748b'} maskColor="#080d16cc" />
    </ReactFlow>
  </div>
}
