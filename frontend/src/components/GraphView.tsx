import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import type { GraphData, GraphEdge, GraphNode } from '../types/graph'

interface Props {
  graph: GraphData
  onNodeSelect: (node: GraphNode) => void
}

const statusColors: Record<string, string> = {
  online: '#55a979', stale: '#c5964b', offline: '#6b7280', external: '#608fbd', unknown: '#8b949e',
}
const nodeWidth = 270
const nodeHeight = 64
const canvasPadding = 90
const minCanvasWidth = 3200
const minCanvasHeight = 2000
const minZoom = 0.15
const maxZoom = 2.5
const requestAnimationWindowMs = 4000
const tcpConnectionWindowMs = 30_000
const datagramConnectionWindowMs = 15_000
const edgePulseWindowMs = 900

type Point = { x: number; y: number }
type Viewport = Point & { zoom: number }
type VisualRelationship = {
  source: string
  target: string
  forwardKey: string
  reverseKey: string
  hasForward: boolean
  hasReverse: boolean
  hasTraffic: boolean
  hasReachability: boolean
  connectionForwardUntil: number
  connectionReverseUntil: number
  pulseForwardUntil: number
  pulseReverseUntil: number
  activeForwardUntil: number
  activeReverseUntil: number
  failedForwardUntil: number
  failedReverseUntil: number
  weight: number
  totalBytes: number
  requestCount: number
  errorCount: number
  avgRTTMs: number
  protocols: Set<string>
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
  return edge.active ? Date.parse(edge.last_observed_at) + requestAnimationWindowMs : 0
}

function failedUntil(edge: GraphEdge) {
  const parsedFailedUntil = Date.parse(edge.failed_until || '')
  if (Number.isFinite(parsedFailedUntil)) return parsedFailedUntil
  const parsedLastError = Date.parse(edge.last_error_at || '')
  if (Number.isFinite(parsedLastError)) return parsedLastError + requestAnimationWindowMs
  if (edge.failed) {
    const observedAt = Date.parse(edge.last_observed_at || edge.last_seen)
    return Number.isFinite(observedAt) ? observedAt + requestAnimationWindowMs : 0
  }
  return 0
}

function connectionUntil(edge: GraphEdge) {
  const observedAt = Date.parse(edge.last_observed_at || edge.last_seen)
  if (!Number.isFinite(observedAt)) return 0
  const windowMs = edge.protocol === 'tcp' ? tcpConnectionWindowMs : datagramConnectionWindowMs
  return observedAt + windowMs
}

function pulseUntil(edge: GraphEdge) {
  const observedAt = Date.parse(edge.last_observed_at || edge.last_seen)
  if (!Number.isFinite(observedAt)) return 0
  return observedAt + edgePulseWindowMs
}

function isRequestEdge(edge: GraphEdge) {
  return (edge.request_count ?? 0) > 0 && edge.kind !== 'reachability' && edge.protocol !== 'icmp'
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

function ExternalIcon() {
  return <span className="vm-node-icon external-icon" aria-hidden="true">
    <svg viewBox="0 0 24 24" width="24" height="24" fill="none">
      <circle cx="12" cy="12" r="8" />
      <path d="M4 12h16M12 4c2 2.2 3 4.8 3 8s-1 5.8-3 8M12 4c-2 2.2-3 4.8-3 8s1 5.8 3 8" />
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

function formatCompactBytes(value: number) {
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)}GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)}MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)}KB`
  return `${value}B`
}

function relationshipKey(source: string, target: string) {
  return [source, target].sort().join('<->')
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

  const graphNodes = useMemo(() => [...graph.nodes]
    .sort((a, b) => nodeSortKey(a).localeCompare(nodeSortKey(b))), [graph.nodes])
  if (graphNodes.length > 0) {
    lastVMNodes.current = graphNodes
  }
  const visibleGraphNodes = graphNodes.length > 0 ? graphNodes : lastVMNodes.current
  const nodeIDs = useMemo(() => new Set(visibleGraphNodes.map((node) => node.id)), [visibleGraphNodes])

  const nodes = useMemo(() => visibleGraphNodes.map((node, index) => {
    const status = node.status || 'unknown'
    const color = statusColors[status] || statusColors.unknown
    const position = positionForSlot(index, visibleGraphNodes.length)
    return {
      node,
      color,
      status,
      position,
      center: { x: position.x + nodeWidth / 2, y: position.y + nodeHeight / 2 },
    }
  }), [visibleGraphNodes])
  const nodeByID = useMemo(() => new Map(nodes.map((item) => [item.node.id, item])), [nodes])

  const edges = useMemo(() => {
    // Several ports/protocols between the same two nodes become one visual
    // relationship. The base line represents known connectivity/history.
    // Active overlays show traffic direction on the same geometry.
    const relationships = new Map<string, VisualRelationship>()
    graph.edges.forEach((edge) => {
      if (edge.source === edge.target) return
      if (!nodeIDs.has(edge.source) || !nodeIDs.has(edge.target)) return
      const { source, target } = visualDirection(edge, graph.edges, nodeIDs)
      const key = relationshipKey(source, target)
      const [endpointA, endpointB] = key.split('<->')
      const forwardKey = `${endpointA}->${endpointB}`
      const reverseKey = `${endpointB}->${endpointA}`
      const directionKey = `${source}->${target}`
      const current = relationships.get(key) || {
        source: endpointA,
        target: endpointB,
        forwardKey,
        reverseKey,
        hasForward: false,
        hasReverse: false,
        hasTraffic: false,
        hasReachability: false,
        connectionForwardUntil: 0,
        connectionReverseUntil: 0,
        pulseForwardUntil: 0,
        pulseReverseUntil: 0,
        activeForwardUntil: 0,
        activeReverseUntil: 0,
        failedForwardUntil: 0,
        failedReverseUntil: 0,
        weight: 1,
        totalBytes: 0,
        requestCount: 0,
        errorCount: 0,
        avgRTTMs: 0,
        protocols: new Set<string>(),
      }
      const isReachability = edge.kind === 'reachability' || edge.reachable === true || edge.protocol === 'icmp'
      const animates = isRequestEdge(edge)
      const until = isReachability || !animates ? 0 : activeUntil(edge)
      const failedUntilValue = isReachability || (edge.error_count ?? 0) <= 0 ? 0 : failedUntil(edge)
      const connectedUntil = Math.max(connectionUntil(edge), until)
      const freshUntil = pulseUntil(edge)
      current.hasTraffic = current.hasTraffic || !isReachability
      current.hasReachability = current.hasReachability || isReachability
      if (edge.protocol) {
        current.protocols.add(edge.protocol)
      }
      if (directionKey === forwardKey) {
        current.hasForward = true
        current.connectionForwardUntil = Math.max(current.connectionForwardUntil, connectedUntil)
        current.pulseForwardUntil = Math.max(current.pulseForwardUntil, freshUntil)
        current.activeForwardUntil = Math.max(current.activeForwardUntil, until)
        current.failedForwardUntil = Math.max(current.failedForwardUntil, failedUntilValue)
      } else {
        current.hasReverse = true
        current.connectionReverseUntil = Math.max(current.connectionReverseUntil, connectedUntil)
        current.pulseReverseUntil = Math.max(current.pulseReverseUntil, freshUntil)
        current.activeReverseUntil = Math.max(current.activeReverseUntil, until)
        current.failedReverseUntil = Math.max(current.failedReverseUntil, failedUntilValue)
      }
      if (!isReachability) {
        current.weight = Math.max(current.weight, edge.weight)
        current.totalBytes += edge.total_bytes ?? edge.bytes_sent + edge.bytes_received
        current.requestCount += edge.request_count ?? 0
        current.errorCount += edge.error_count ?? 0
      }
      current.avgRTTMs = Math.max(current.avgRTTMs, edge.avg_rtt_ms ?? 0)
      relationships.set(key, current)
    })

    return Array.from(relationships.entries()).flatMap(([id, relationship]) => {
      const source = nodeByID.get(relationship.source)
      const target = nodeByID.get(relationship.target)
      if (!source || !target) return []
      const failedForward = relationship.failedForwardUntil > clock
      const failedReverse = relationship.failedReverseUntil > clock
      const failed = failedForward || failedReverse
      const activeForward = relationship.activeForwardUntil > clock && !failedForward
      const activeReverse = relationship.activeReverseUntil > clock && !failedReverse
      const active = activeForward || activeReverse
      const connectionForward = relationship.connectionForwardUntil > clock
      const connectionReverse = relationship.connectionReverseUntil > clock
      const connected = connectionForward || connectionReverse || active || failed
      const fresh = relationship.pulseForwardUntil > clock || relationship.pulseReverseUntil > clock || failed
      const reachabilityOnly = relationship.hasReachability && !relationship.hasTraffic
      const start = edgePoint(source.center, target.center)
      const end = edgePoint(target.center, source.center)
      const protocolLabel = Array.from(relationship.protocols).filter(Boolean).sort().join('/')
      const label = reachabilityOnly
        ? ['connected', protocolLabel, relationship.hasForward && relationship.hasReverse ? '2-way' : '', relationship.avgRTTMs ? `${relationship.avgRTTMs.toFixed(1)}ms` : ''].filter(Boolean).join(' · ')
        : [
          formatCompactBytes(relationship.totalBytes),
          relationship.requestCount ? `${relationship.requestCount} req` : '',
          protocolLabel,
          relationship.hasForward && relationship.hasReverse ? '2-way' : '',
          relationship.avgRTTMs ? `${relationship.avgRTTMs.toFixed(1)}ms` : '',
          relationship.errorCount ? `${relationship.errorCount} err` : '',
        ].filter(Boolean).join(' · ')
      return {
        id,
        active,
        connected,
        fresh,
        hasTraffic: relationship.hasTraffic,
        reachabilityOnly,
        activeForward,
        activeReverse,
        failedForward,
        failedReverse,
        connectionForward,
        connectionReverse,
        hasForward: connected ? relationship.hasForward : false,
        hasReverse: connected ? relationship.hasReverse : false,
        width: Math.min(4, 1.25 + relationship.weight * 0.4),
        path: curvedPath(start, end),
        label,
        labelX: (start.x + end.x) / 2,
        labelY: (start.y + end.y) / 2 - 10,
      }
    })
  }, [clock, graph.edges, nodeByID, nodeIDs])

  const canvas = useMemo(() => {
    const maxX = Math.max(...nodes.map((item) => item.position.x + nodeWidth + canvasPadding), minCanvasWidth)
    const maxY = Math.max(...nodes.map((item) => item.position.y + nodeHeight + canvasPadding), minCanvasHeight)
    return { width: maxX, height: maxY }
  }, [nodes])

  return <div
    ref={canvasRef}
    className={`graph-canvas${panning ? ' panning' : ''}`}
    onPointerDown={handlePanPointerDown}
    onPointerMove={handlePanPointerMove}
    onPointerUp={finishPan}
    onPointerCancel={finishPan}
  >
    {visibleGraphNodes.length === 0 && <div className="graph-empty"><strong>Waiting for VM</strong><span></span></div>}
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
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#5fae7e" />
          </marker>
          <marker id="edge-arrow-reachability" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#79add1" />
          </marker>
          <marker id="edge-arrow-failed" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#d89b45" />
          </marker>
        </defs>
        {edges.map((edge) => edge.connected ? <path
          key={edge.id}
          className={`graph-edge graph-edge-connection graph-edge-idle${edge.fresh ? ' graph-edge-pulse' : ''}`}
          d={edge.path}
          markerStart={edge.connectionReverse ? 'url(#edge-arrow-idle)' : undefined}
          markerEnd={edge.connectionForward ? 'url(#edge-arrow-idle)' : undefined}
          style={{ strokeWidth: edge.active ? Math.max(1.25, edge.width * 0.75) : 1.35, opacity: edge.active ? 0.58 : 0.72 }}
        /> : null)}
        {edges.map((edge) => edge.activeForward ? <path
          key={`${edge.id}-forward-active`}
          className="graph-edge graph-edge-active graph-edge-forward"
          d={edge.path}
          markerEnd="url(#edge-arrow-active)"
          style={{ strokeWidth: edge.width }}
        /> : null)}
        {edges.map((edge) => edge.activeReverse ? <path
          key={`${edge.id}-reverse-active`}
          className="graph-edge graph-edge-active graph-edge-reverse"
          d={edge.path}
          markerStart="url(#edge-arrow-active)"
          style={{ strokeWidth: edge.width }}
        /> : null)}
        {edges.map((edge) => edge.failedForward ? <path
          key={`${edge.id}-forward-failed`}
          className="graph-edge graph-edge-failed graph-edge-forward"
          d={edge.path}
          markerEnd="url(#edge-arrow-failed)"
          style={{ strokeWidth: Math.max(edge.width, 2.2) }}
        /> : null)}
        {edges.map((edge) => edge.failedReverse ? <path
          key={`${edge.id}-reverse-failed`}
          className="graph-edge graph-edge-failed graph-edge-reverse"
          d={edge.path}
          markerStart="url(#edge-arrow-failed)"
          style={{ strokeWidth: Math.max(edge.width, 2.2) }}
        /> : null)}
        {edges.map((edge) => edge.connected && edge.label ? <text
          key={`${edge.id}-label`}
          className="graph-edge-label"
          x={edge.labelX}
          y={edge.labelY}
          textAnchor="middle"
        >{edge.label}</text> : null)}
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
          {node.type === 'external' ? <ExternalIcon /> : <VMIcon />}
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
