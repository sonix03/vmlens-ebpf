package api

import (
	"net/http"

	"github.com/vmlens/vmlens/backend/internal/realtime"
)

func Routes(handlers *Handlers, hub *realtime.Hub, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("POST /api/agents/register", handlers.RegisterAgent)
	mux.HandleFunc("POST /api/agents/heartbeat", handlers.Heartbeat)
	mux.HandleFunc("GET /api/agents", handlers.ListAgents)
	mux.HandleFunc("GET /api/vms", handlers.ListVMs)
	mux.HandleFunc("GET /api/flows", handlers.ListFlows)
	mux.HandleFunc("POST /api/flows/ingest", handlers.IngestFlow)
	mux.HandleFunc("GET /api/graph", handlers.GetGraph)
	mux.HandleFunc("GET /api/stats/summary", handlers.Summary)
	mux.HandleFunc("GET /api/stats/top-talkers", handlers.TopTalkers)
	mux.Handle("GET /api/realtime", hub)
	return Middleware(mux, allowedOrigins)
}
