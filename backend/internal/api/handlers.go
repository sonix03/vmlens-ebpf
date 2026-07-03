package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vmlens/vmlens/backend/internal/model"
	"github.com/vmlens/vmlens/backend/internal/service"
)

type Handlers struct {
	Pool   *pgxpool.Pool
	Agents *service.AgentService
	VMs    *service.VMService
	Flows  *service.FlowService
	Graph  *service.GraphService
	Stats  *service.StatsService
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.Pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "database": "ok", "time": time.Now().UTC()})
}

func (h *Handlers) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var request model.AgentRegistration
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.Agents.Register(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var request model.AgentHeartbeat
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Agents.Heartbeat(r.Context(), request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	result, err := h.Agents.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) ListVMs(w http.ResponseWriter, r *http.Request) {
	result, err := h.VMs.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) IngestFlow(w http.ResponseWriter, r *http.Request) {
	var request model.FlowEvent
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.Flows.Ingest(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (h *Handlers) ListFlows(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.Flows.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GetGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	filter, err := graphFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.Graph.Get(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) Summary(w http.ResponseWriter, r *http.Request) {
	result, err := h.Stats.Summary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) TopTalkers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.Stats.TopTalkers(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func graphFilter(r *http.Request) (model.GraphFilter, error) {
	query := r.URL.Query()
	filter := model.GraphFilter{
		AgentID: query.Get("agent_id"), TenantID: query.Get("tenant_id"), VMID: query.Get("vm_id"),
		Scope: query.Get("scope"), Protocol: strings.ToLower(query.Get("protocol")), Status: query.Get("status"),
	}
	if raw := query.Get("port"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return filter, fmt.Errorf("invalid port")
		}
		filter.Port = port
	}
	if raw := query.Get("min_bytes"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return filter, fmt.Errorf("invalid min_bytes")
		}
		filter.MinBytes = value
	}
	if raw := query.Get("time_range"); raw != "" {
		duration, err := parseDuration(raw)
		if err != nil {
			return filter, fmt.Errorf("invalid time_range: %w", err)
		}
		filter.TimeRange = duration
	}
	return filter, nil
}

func parseDuration(raw string) (time.Duration, error) {
	if strings.HasSuffix(raw, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

func decodeJSON(r *http.Request, destination any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
