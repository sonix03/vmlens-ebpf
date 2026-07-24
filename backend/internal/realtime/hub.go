package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[chan []byte]struct{}
	throttleMu sync.Mutex
	throttled  map[string]throttledEvent
}

type throttledEvent struct {
	payload []byte
	timer   *time.Timer
}

func New() *Hub {
	return &Hub{
		clients:   make(map[chan []byte]struct{}),
		throttled: make(map[string]throttledEvent),
	}
}

func (h *Hub) Broadcast(eventType string, data any) {
	payload, err := json.Marshal(Event{Type: eventType, Data: data, Timestamp: time.Now().UTC()})
	if err != nil {
		return
	}
	h.broadcastPayload(payload)
}

func (h *Hub) BroadcastLatest(eventType string, data any, interval time.Duration) {
	if interval <= 0 {
		h.Broadcast(eventType, data)
		return
	}
	payload, err := json.Marshal(Event{Type: eventType, Data: data, Timestamp: time.Now().UTC()})
	if err != nil {
		return
	}

	h.throttleMu.Lock()
	if current, ok := h.throttled[eventType]; ok {
		current.payload = payload
		h.throttled[eventType] = current
		h.throttleMu.Unlock()
		return
	}
	h.throttled[eventType] = throttledEvent{payload: payload}
	timer := time.AfterFunc(interval, func() { h.flushLatest(eventType) })
	current := h.throttled[eventType]
	current.timer = timer
	h.throttled[eventType] = current
	h.throttleMu.Unlock()
}

func (h *Hub) flushLatest(eventType string) {
	h.throttleMu.Lock()
	current, ok := h.throttled[eventType]
	if ok {
		delete(h.throttled, eventType)
	}
	h.throttleMu.Unlock()
	if ok {
		h.broadcastPayload(current.payload)
	}
}

func (h *Hub) broadcastPayload(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client <- payload:
		default:
			// A slow browser must not block agent ingestion. The frontend will
			// refetch the current graph after its next event or reconnect.
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	client := make(chan []byte, 32)
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
		close(client)
	}()

	initial, _ := json.Marshal(Event{Type: "connected", Timestamp: time.Now().UTC()})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-client:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
