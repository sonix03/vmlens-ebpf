import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import type { ConnectionHealth, ConnectionSummary, GraphData, GraphEdge, GraphNode } from '../types/graph'

interface Props {
  graph: GraphData
  onNodeSelect: (node: GraphNode) => void
  onConnectionSelect: (connection: ConnectionSummary) => void
  selectedNodeID?: string
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
const slowRTTThresholdMs = positiveNumberEnv(import.meta.env.VITE_SLOW_RTT_THRESHOLD_MS, 100)

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
  packets: number
  connectionCount: number
  requestCount: number
  errorCount: number
  retransmissionCount: number
  avgRTTMs: number
  p95RTTMs: number
  avgResponseDurationMs: number
  lastResponseCode?: number
  lastSeen?: string
  lastObservedAt?: string
  scopes: Set<string>
  protocols: Set<string>
  ports: Set<number>
  agentIDs: Set<string>
  observationPoints: Set<string>
}

function clampZoom(zoom: number) {
  return Math.min(maxZoom, Math.max(minZoom, zoom))
}

function positiveNumberEnv(raw: unknown, fallback: number) {
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
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

function latestTimestamp(current: string | undefined, next: string | undefined) {
  if (!next) return current
  if (!current) return next
  return Date.parse(next) > Date.parse(current) ? next : current
}

function connectionHealth(connected: boolean, failed: boolean, slow: boolean): ConnectionHealth {
  if (failed) return 'failed'
  if (slow) return 'degraded'
  if (connected) return 'healthy'
  return 'inactive'
}

function activeDirection(activeForward: boolean, activeReverse: boolean): ConnectionSummary['active_direction'] {
  if (activeForward && activeReverse) return 'bidirectional'
  if (activeForward) return 'source_to_target'
  if (activeReverse) return 'target_to_source'
  return 'none'
}

function connectionRecommendation(connection: Pick<ConnectionSummary, 'health' | 'active' | 'connected' | 'request_count' | 'error_count'>) {
  if (connection.health === 'failed') {
    return 'Check destination host health, route, firewall rule, and whether the target service is listening on the observed port. Retest after fixing the failing layer.'
  }
  if (connection.health === 'degraded') {
    return 'Inspect RTT trend, retransmission, packet loss, cross-zone routing, and destination host load. Compare this path with another healthy path.'
  }
  if (connection.active) {
    return 'Request traffic is active. Use Request Flow or L7 Requests to inspect method, path, status code, and app delay.'
  }
  if (connection.connected) {
    return 'Connection is reachable but idle. Keep it as a dependency edge, or run a validation request if this path should carry application traffic.'
  }
  return 'No recent observed connectivity. Run a reachability probe or application request before treating this connection as working.'
}

type ParticleEdge = {
  failed: boolean
  slow: boolean
  activeForward: boolean
  activeReverse: boolean
}

function useReducedMotion() {
  const [reduced, setReduced] = useState(() =>
    typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches)

  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => setReduced(query.matches)
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])

  return reduced
}

// Dots mean request traffic is in flight right now, and they only run on the
// direction that traffic actually took. A path that is merely connected shows
// its line and nothing else, so motion on the map always means a live request.
function EdgeParticles({ pathID, edge, calm }: { pathID: string; edge: ParticleEdge; calm: boolean }) {
  if (edge.failed) return null
  const directions: Array<'forward' | 'reverse'> = []
  if (edge.activeForward) directions.push('forward')
  if (edge.activeReverse) directions.push('reverse')
  if (directions.length === 0) return null

  // Under prefers-reduced-motion the stream is slowed and thinned rather than
  // removed, so the direction stays readable.
  const duration = 2.2 * (calm ? 2.6 : 1)
  const count = calm ? 3 : 6
  const radius = 3
  const tone = edge.slow ? 'slow' : 'active'

  return <>
    {directions.flatMap((direction) => Array.from({ length: count }, (_, index) => {
      const offset = -(duration / count) * index
      return <circle
        key={`${pathID}-${direction}-${index}`}
        r={radius}
        className={`graph-particle graph-particle-${tone}`}
      >
        <animateMotion
          dur={`${duration}s`}
          begin={`${offset}s`}
          repeatCount="indefinite"
          calcMode="linear"
          keyPoints={direction === 'forward' ? '0;1' : '1;0'}
          keyTimes="0;1"
        >
          {/* xlink:href as well: Safari only gained plain href on mpath late. */}
          <mpath href={`#${pathID}`} xlinkHref={`#${pathID}`} />
        </animateMotion>
      </circle>
    }))}
  </>
}

export function GraphView({ graph, onNodeSelect, onConnectionSelect, selectedNodeID }: Props) {
  const [clock, setClock] = useState(() => Date.now())
  const calmMotion = useReducedMotion()
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
        packets: 0,
        connectionCount: 0,
        requestCount: 0,
        errorCount: 0,
        retransmissionCount: 0,
        avgRTTMs: 0,
        p95RTTMs: 0,
        avgResponseDurationMs: 0,
        scopes: new Set<string>(),
        protocols: new Set<string>(),
        ports: new Set<number>(),
        agentIDs: new Set<string>(),
        observationPoints: new Set<string>(),
      }
      const isReachability = edge.kind === 'reachability' || edge.reachable === true || edge.protocol === 'icmp'
      const animates = isRequestEdge(edge)
      const until = isReachability || !animates ? 0 : activeUntil(edge)
      const failedUntilValue = edge.failed || (!isReachability && (edge.error_count ?? 0) > 0) ? failedUntil(edge) : 0
      const connectedUntil = Math.max(connectionUntil(edge), until)
      const freshUntil = pulseUntil(edge)
      current.hasTraffic = current.hasTraffic || !isReachability
      current.hasReachability = current.hasReachability || isReachability
      if (edge.protocol) {
        current.protocols.add(edge.protocol)
      }
      if (edge.scope) {
        current.scopes.add(edge.scope)
      }
      const servicePort = edge.server_port || edge.dst_port
      if (servicePort > 0) {
        current.ports.add(servicePort)
      }
      edge.agent_ids?.forEach((agentID) => current.agentIDs.add(agentID))
      edge.observation_points?.forEach((point) => current.observationPoints.add(point))
      current.lastSeen = latestTimestamp(current.lastSeen, edge.last_seen)
      current.lastObservedAt = latestTimestamp(current.lastObservedAt, edge.last_observed_at)
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
        current.packets += edge.packets ?? 0
        current.connectionCount += edge.connection_count ?? 0
        current.requestCount += edge.request_count ?? 0
        current.errorCount += edge.error_count ?? 0
        current.retransmissionCount += edge.retransmission_count ?? 0
      }
      current.avgRTTMs = Math.max(current.avgRTTMs, edge.avg_rtt_ms ?? 0)
      current.p95RTTMs = Math.max(current.p95RTTMs, edge.p95_rtt_ms ?? 0)
      current.avgResponseDurationMs = Math.max(current.avgResponseDurationMs, edge.avg_response_duration_ms ?? edge.avg_app_delay_ms ?? 0)
      if (edge.last_response_code !== undefined) {
        current.lastResponseCode = edge.last_response_code
      }
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
      const slow = connected && !failed && relationship.avgRTTMs >= slowRTTThresholdMs
      const reachabilityOnly = relationship.hasReachability && !relationship.hasTraffic
      const start = edgePoint(source.center, target.center)
      const end = edgePoint(target.center, source.center)
      const protocolLabel = Array.from(relationship.protocols).filter(Boolean).sort().join('/')
      const healthLabel = failed ? 'failed' : slow ? 'slow RTT' : connected ? 'stable' : ''
      const label = reachabilityOnly
        ? [healthLabel, protocolLabel, relationship.hasForward && relationship.hasReverse ? '2-way' : '', relationship.avgRTTMs ? `${relationship.avgRTTMs.toFixed(1)}ms` : ''].filter(Boolean).join(' · ')
        : [
          healthLabel,
          formatCompactBytes(relationship.totalBytes),
          relationship.requestCount ? `${relationship.requestCount} req` : '',
          protocolLabel,
          relationship.hasForward && relationship.hasReverse ? '2-way' : '',
          relationship.avgRTTMs ? `${relationship.avgRTTMs.toFixed(1)}ms` : '',
          relationship.errorCount ? `${relationship.errorCount} err` : '',
        ].filter(Boolean).join(' · ')
      const health = connectionHealth(connected, failed, slow)
      const summaryBase = {
        health,
        active,
        connected,
        request_count: relationship.requestCount,
        error_count: relationship.errorCount,
      }
      const connection: ConnectionSummary = {
        id,
        source: relationship.source,
        target: relationship.target,
        source_label: source.node.label,
        target_label: target.node.label,
        source_ip: source.node.ip,
        target_ip: target.node.ip,
        direction: relationship.hasForward && relationship.hasReverse ? 'two_way' : 'one_way',
        active_direction: activeDirection(activeForward, activeReverse),
        health,
        scope: Array.from(relationship.scopes).sort()[0] || (reachabilityOnly ? 'reachability' : 'unknown'),
        protocols: Array.from(relationship.protocols).filter(Boolean).sort(),
        ports: Array.from(relationship.ports).filter((port) => port > 0).sort((a, b) => a - b),
        connected,
        active,
        failed,
        slow,
        request_count: relationship.requestCount,
        error_count: relationship.errorCount,
        retransmission_count: relationship.retransmissionCount,
        total_bytes: relationship.totalBytes,
        packets: relationship.packets,
        connection_count: relationship.connectionCount,
        avg_rtt_ms: relationship.avgRTTMs,
        p95_rtt_ms: relationship.p95RTTMs,
        avg_response_duration_ms: relationship.avgResponseDurationMs,
        last_response_code: relationship.lastResponseCode,
        last_seen: relationship.lastSeen,
        last_observed_at: relationship.lastObservedAt,
        agent_ids: Array.from(relationship.agentIDs).sort(),
        observation_points: Array.from(relationship.observationPoints).sort(),
        recommendation: connectionRecommendation(summaryBase),
      }
      return {
        id,
        active,
        failed,
        connected,
        fresh,
        slow,
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
        connection,
      }
    })
  }, [clock, graph.edges, nodeByID, nodeIDs])

  // Selecting a node lights its own paths and mutes everything else, so the
  // relationship stands out without losing the surrounding topology.
  const focus = useMemo(() => {
    if (!selectedNodeID) return undefined
    const neighbours = new Set<string>([selectedNodeID])
    edges.forEach((edge) => {
      if (edge.connection.source === selectedNodeID) neighbours.add(edge.connection.target)
      else if (edge.connection.target === selectedNodeID) neighbours.add(edge.connection.source)
    })
    return { id: selectedNodeID, neighbours }
  }, [edges, selectedNodeID])

  function edgeFocusClass(edge: { connection: ConnectionSummary }) {
    if (!focus) return ''
    const touchesFocus = edge.connection.source === focus.id || edge.connection.target === focus.id
    return touchesFocus ? ' graph-edge-focus' : ' graph-edge-muted'
  }

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
          <marker id="edge-arrow-slow" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#d9ad55" />
          </marker>
          <marker id="edge-arrow-reachability" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#79add1" />
          </marker>
          <marker id="edge-arrow-failed" markerWidth="10" markerHeight="10" refX="5" refY="5" orient="auto-start-reverse">
            <path d="M 1 1 L 9 5 L 1 9 z" fill="#e15757" />
          </marker>
        </defs>
        {edges.map((edge) => {
          if (!edge.connected) return null
          const markerID = edge.failed
            ? 'edge-arrow-failed'
            : edge.active
              ? edge.slow ? 'edge-arrow-slow' : 'edge-arrow-active'
              : edge.slow ? 'edge-arrow-slow' : 'edge-arrow-idle'
          const showStartMarker = edge.failed
            ? edge.failedReverse
            : edge.active
              ? edge.activeReverse
              : edge.connectionReverse
          const showEndMarker = edge.failed
            ? edge.failedForward
            : edge.active
              ? edge.activeForward
              : edge.connectionForward
          const classNames = ['graph-edge', 'graph-edge-single']
          if (edge.failed) {
            classNames.push('graph-edge-failed')
            if (edge.failedReverse && !edge.failedForward) classNames.push('graph-edge-reverse')
            if (edge.failedForward && edge.failedReverse) classNames.push('graph-edge-bidirectional')
          } else if (edge.active) {
            classNames.push('graph-edge-active')
            if (edge.slow) classNames.push('graph-edge-active-slow')
            if (edge.activeReverse && !edge.activeForward) classNames.push('graph-edge-reverse')
            if (edge.activeForward && edge.activeReverse) classNames.push('graph-edge-bidirectional')
          } else {
            classNames.push('graph-edge-connection', edge.slow ? 'graph-edge-slow' : 'graph-edge-idle')
            if (edge.fresh) classNames.push('graph-edge-pulse')
          }
          const pathID = `edge-path-${edge.id.replace(/[^A-Za-z0-9_-]/g, '_')}`
          return <g key={edge.id} className={`graph-edge-group${edgeFocusClass(edge)}`}>
            <path
              id={pathID}
              className={classNames.join(' ')}
              d={edge.path}
              onClick={(event) => {
                event.stopPropagation()
                onConnectionSelect(edge.connection)
              }}
              onPointerDown={(event) => event.stopPropagation()}
              markerStart={showStartMarker ? `url(#${markerID})` : undefined}
              markerEnd={showEndMarker ? `url(#${markerID})` : undefined}
              style={{
                // The dots carry motion and activity now, so the line itself
                // stays thin and quiet and only failure is allowed to shout.
                strokeWidth: edge.failed ? Math.max(edge.width, 2.2) : Math.min(edge.width, 1.8),
                opacity: 1,
              }}
            />
            <EdgeParticles pathID={pathID} edge={edge} calm={calmMotion} />
          </g>
        })}
        {edges.map((edge) => edge.connected && edge.label ? <text
          key={`${edge.id}-label`}
          className={`graph-edge-label${edgeFocusClass(edge)}`}
          x={edge.labelX}
          y={edge.labelY}
          textAnchor="middle"
        >{edge.label}</text> : null)}
      </svg>
      {nodes.map(({ node, color, status, position }) => <button
        key={node.id}
        type="button"
        data-testid={`node-${node.id}`}
        className={`graph-node-button vm-node-${status}${focus ? (focus.id === node.id ? ' vm-node-selected' : focus.neighbours.has(node.id) ? '' : ' vm-node-muted') : ''}`}
        title={`${node.label} · ${node.ip || 'no IP'} · ${status}`}
        style={{ left: position.x, top: position.y, borderColor: node.status === 'online' ? color : `${color}80` }}
        onClick={() => onNodeSelect(node)}
      >
        <div className="vm-node-content">
          {node.type === 'external' ? <ExternalIcon /> : <VMIcon />}
          <span className="vm-node-text">
            <strong>{node.label}</strong>
            <small>{node.ip || 'no IP'} · {status}</small>
            <em>{node.role || node.type || 'host'}</em>
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
