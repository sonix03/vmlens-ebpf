import { API_URL } from './client'

export interface RealtimeEvent {
  type: string
  data?: unknown
  timestamp: string
}

export function connectRealtime(onEvent: (event: RealtimeEvent) => void, onStatus: (connected: boolean) => void): () => void {
  const source = new EventSource(`${API_URL}/api/realtime`)
  source.onopen = () => onStatus(true)
  source.onerror = () => onStatus(false)
  source.onmessage = (message) => {
    try {
      onEvent(JSON.parse(message.data) as RealtimeEvent)
    } catch {
      // Ignore malformed events; EventSource reconnect remains active.
    }
  }
  return () => source.close()
}

