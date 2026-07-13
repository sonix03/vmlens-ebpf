import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent as ReactWheelEvent,
} from 'react'
import type { GraphData, GraphEdge, GraphNode } from '../types/graph'

interface Props {
  graph: GraphData
  onNodeSelect: (node: GraphNode) => void
}

const statusColors: Record<string, string> = {
  online: '#55a979', stale: '#c5964b', offline: '#6b7280', unknown: '#8b949e',
}
const nodeWidth = 270
const nodeHeight = 64
const canvasPadding = 90
const minCanvasWidth = 3200
const minCanvasHeight = 2000
const minZoom = 0.15
const maxZoom = 2.5

type Point = { x: number; y: number }
type Viewport = Point & { zoom: number }
type VisualRelationship = {
  source: string
  target: string
  weight: number
  activeUntil: number
}

function clampZoom(zoom: number) {
  return Math.min(maxZoom, Math.max(minZoom, zoom))
}

function curvedPath(source: Point, target: Point) {
  const dx = target.x - source.x
  const dy = target.y - source.y
  const length = Math.hypot(dx, dy) || 1
  const curve = Math.min(40, Math.max(10, length * 0.04))
  const mid = {
    x: (source.x + target.x) / 2 - (dy / length) * curve,
    y: (source.y + target.y) / 2 + (dx / length) * curve,
  }
  return `M ${source.x} ${source.y} Q ${mid.x} ${mid.y} ${target.x} ${target.y}`
}

function edgePoint(from: Point, to: Point) {
  const dx = to.x - from.x
  const dy = to.y - from.y
  const absDx = Math.abs(dx) || 1
  const absDy = Math.abs(dy) || 1
  const scale = Math.min((nodeWidth / 2) / absDx, (nodeHeight / 2) / absDy)
  return {
    x: from.x + dx * scale,
    y: from.y + dy * scale,
  }
}

function activeUntil(edge: GraphEdge) {
  const parsedActiveUntil = Date.parse(edge.active_until)
  if (Number.isFinite(parsedActiveUntil)) return parsedActiveUntil
  return edge.active ? Date.parse(edge.last_observed_at) + 3000 : 0
}

function isLikelyEphemeralPort(port: number) {
  return port >= 32768
}

function hasOppositeServiceEdge(edge: GraphEdge, edges: GraphEdge[], vmIDs: Set<string>) {
  if (!isLikelyEphemeralPort(edge.dst_port)) return false
  return edges.some((candidate) => {
    if (!vmIDs.has(candidate.source) || !vmIDs.has(candidate.target)) return false
    return candidate.source === edge.target
      && candidate.target === edge.source
      && candidate.protocol === edge.protocol
      && !isLikelyEphemeralPort(candidate.dst_port)
  })
}

function visualDirection(edge: GraphEdge, edges: GraphEdge[], vmIDs: Set<string>) {
  // TC ingress sees the server receiving packets and can produce a reverse edge
  // to the client's ephemeral port. For the topology animation, fold that
  // response-only edge into the opposite service-port edge so the visible arrow
  // follows the request path: client -> server.
  if (hasOppositeServiceEdge(edge, edges, vmIDs)) {
    return { source: edge.target, target: edge.source }
  }
  return { source: edge.source, target: edge.target }
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

function nodeSortKey(node: GraphNode) {
  return `${node.ip || '999.999.999.999'}-${node.label}-${node.id}`
}

function positionForSlot(slot: number, total: number) {
  if (total === 1) return { x: canvasPadding + 330, y: canvasPadding + 230 }
  if (total === 2) return { x: canvasPadding + slot * 560, y: canvasPadding + 240 }
  const columns = Math.min(3, Math.ceil(Math.sqrt(total)))
  const column = slot % columns
  const row = Math.floor(slot / columns)
  return { x: canvasPadding + column * 470, y: canvasPadding + row * 240 }
}

export function GraphView({ graph, onNodeSelect }: Props) {
  const [clock, setClock] = useState(() => Date.now())
  const [viewport, setViewport] = useState<Viewport>({ x: 0, y: 0, zoom: 1 })
  const [panning, setPanning] = useState(false)
  const canvasRef = useRef<HTMLDivElement>(null)
  const lastVMNodes = useRef<GraphNode[]>([])
  const panState = useRef<{
    pointerId: number
    startPointer: Point
    startViewport: Viewport
  }>()

  useEffect(() => {
    const interval = window.setInterval(() => setClock(Date.now()), 250)
    return () => window.clearInterval(interval)
  }, [])

  function zoomAt(clientPoint: Point, nextZoom: number) {
    const rect = canvasRef.current?.getBoundingClientRect()
    if (!rect) return
    setViewport((current) => {
      const zoom = clampZoom(nextZoom)
      const graphPoint = {
        x: (clientPoint.x - rect.left - current.x) / current.zoom,
        y: (clientPoint.y - rect.top - current.y) / current.zoom,
      }
      return {
        zoom,
        x: clientPoint.x - rect.left - graphPoint.x * zoom,
        y: clientPoint.y - rect.top - graphPoint.y * zoom,
      }
    })
  }

  function zoomFromCenter(factor: number) {
    const rect = canvasRef.current?.getBoundingClientRect()
    if (!rect) return
    zoomAt({ x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }, viewport.zoom * factor)
  }

  function handleWheel(event: ReactWheelEvent<HTMLDivElement>) {
    event.preventDefault()
    const factor = event.deltaY < 0 ? 1.12 : 0.88
    zoomAt({ x: event.clientX, y: event.clientY }, viewport.zoom * factor)
  }

  function handlePanPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return
    const target = event.target instanceof HTMLElement ? event.target : undefined
    if (target?.closest('button')) return
    event.currentTarget.setPointerCapture(event.pointerId)
    panState.current = {
      pointerId: event.pointerId,
      startPointer: { x: event.clientX, y: event.clientY },
      startViewport: viewport,
    }
    setPanning(true)
  }

  function handlePanPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    const pan = panState.current
    if (!pan || pan.pointerId !== event.pointerId) return
    setViewport({
      ...pan.startViewport,
      x: pan.startViewport.x + event.clientX - pan.startPointer.x,
      y: pan.startViewport.y + event.clientY - pan.startPointer.y,
    })
  }

  function finishPan(event: ReactPointerEvent<HTMLDivElement>) {
    const pan = panState.current
    if (!pan || pan.pointerId !== event.pointerId) return
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    panState.current = undefined
    setPanning(false)
  }

  // The main topology is VM-only. Unknown/private and public endpoints remain
  // available through the API without becoming graph nodes.
  const vmNodes = useMemo(() => graph.nodes
    .filter((node) => node.type === 'vm')
    .sort((a, b) => nodeSortKey(a).localeCompare(nodeSortKey(b))), [graph.nodes])
  if (vmNodes.length > 0) {
    lastVMNodes.current = vmNodes
  }
  const visibleVMNodes = vmNodes.length > 0 ? vmNodes : lastVMNodes.current
  const vmIDs = useMemo(() => new Set(visibleVMNodes.map((node) => node.id)), [visibleVMNodes])

  const nodes = useMemo(() => visibleVMNodes.map((node, index) => {
    const status = node.status || 'unknown'
    const color = statusColors[status] || statusColors.unknown
    const position = positionForSlot(index, visibleVMNodes.length)
    return {
      node,
      color,
      status,
      position,
      center: { x: position.x + nodeWidth / 2, y: position.y + nodeHeight / 2 },
    }
  }), [visibleVMNodes])
  const nodeByID = useMemo(() => new Map(nodes.map((item) => [item.node.id, item])), [nodes])

  const edges = useMemo(() => {
    // Several ports/protocols in the same request direction become one visual
    // edge. Individual aggregated flows remain available in the backend.
    const relationships = new Map<string, VisualRelationship>()
    graph.edges.forEach((edge) => {
      if (!vmIDs.has(edge.source) || !vmIDs.has(edge.target)) return
      if (edge.source === edge.target) return
      const { source, target } = visualDirection(edge, graph.edges, vmIDs)
      const key = `${source}->${target}`
      const current = relationships.get(key) || {
        source,
        target,
        weight: 1,
        activeUntil: 0,
      }
      current.weight = Math.max(current.weight, edge.weight)
      current.activeUntil = Math.max(current.activeUntil, activeUntil(edge))
      relationships.set(key, current)
    })

    return Array.from(relationships.entries()).flatMap(([id, relationship]) => {
      const source = nodeByID.get(relationship.source)
      const target = nodeByID.get(relationship.target)
      if (!source || !target) return []
      const active = relationship.activeUntil > clock
      const color = active ? '#6fa88b' : '#4a515a'
      const start = edgePoint(source.center, target.center)
      const end = edgePoint(target.center, source.center)
      return {
        id,
        active,
        color,
        width: active ? Math.min(4, 1.25 + relationship.weight * 0.4) : 1,
        opacity: active ? 0.95 : 0.36,
        path: curvedPath(start, end),
      }
    })
  }, [clock, graph.edges, nodeByID, vmIDs])

  const canvas = useMemo(() => {
    const maxX = Math.max(...nodes.map((item) => item.position.x + nodeWidth + canvasPadding), minCanvasWidth)
    const maxY = Math.max(...nodes.map((item) => item.position.y + nodeHeight + canvasPadding), minCanvasHeight)
    return { width: maxX, height: maxY }
  }, [nodes])

  return <div
    ref={canvasRef}
    className={`graph-canvas${panning ? ' panning' : ''}`}
    onWheel={handleWheel}
    onPointerDown={handlePanPointerDown}
    onPointerMove={handlePanPointerMove}
    onPointerUp={finishPan}
    onPointerCancel={finishPan}
  >
    {visibleVMNodes.length === 0 && <div className="graph-empty"><strong>Waiting for VM</strong><span></span></div>}
    <div
      className="graph-map"
      style={{
        width: canvas.width,
        height: canvas.height,
        transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})`,
      }}
    >
      <div className="graph-grid" />
      <svg className="graph-edges" viewBox={`0 0 ${canvas.width} ${canvas.height}`} aria-hidden="true">
        <defs>
          <marker id="edge-arrow-active" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#6fa88b" />
          </marker>
          <marker id="edge-arrow-idle" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#4a515a" />
          </marker>
        </defs>
        {edges.map((edge) => <path
          key={edge.id}
          className={`graph-edge ${edge.active ? 'graph-edge-active' : 'graph-edge-idle'}`}
          d={edge.path}
          markerEnd={`url(#${edge.active ? 'edge-arrow-active' : 'edge-arrow-idle'})`}
          style={{ stroke: edge.color, strokeWidth: edge.width, opacity: edge.opacity }}
        />)}
      </svg>
      {nodes.map(({ node, color, status, position }) => <button
        key={node.id}
        type="button"
        data-testid={`node-${node.id}`}
        className={`graph-node-button vm-node-${status}`}
        title={`${node.label} · ${node.ip || 'no IP'} · ${status}`}
        style={{ left: position.x, top: position.y, borderColor: node.status === 'online' ? color : `${color}80` }}
        onClick={() => onNodeSelect(node)}
      >
        <div className="vm-node-content">
          <VMIcon />
          <span className="vm-node-text">
            <strong>{node.label}</strong>
            <small>{node.ip || 'no IP'} · {status}</small>
          </span>
          <NodeStatusIcon status={status} />
        </div>
      </button>)}
    </div>
    <div className="graph-controls" aria-label="Map controls">
      <button type="button" onClick={() => zoomFromCenter(1.18)}>+</button>
      <button type="button" onClick={() => zoomFromCenter(0.82)}>−</button>
      <button type="button" onClick={() => setViewport({ x: 0, y: 0, zoom: 1 })}>Reset</button>
      <span>{Math.round(viewport.zoom * 100)}%</span>
    </div>
  </div>
}
