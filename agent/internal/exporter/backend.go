package exporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Sender struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Sender {
	return &Sender{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}}
}

func (s *Sender) Register(ctx context.Context, registration Registration) (RegistrationResult, error) {
	var result RegistrationResult
	err := s.post(ctx, "/api/agents/register", registration, &result)
	return result, err
}

func (s *Sender) Heartbeat(ctx context.Context, heartbeat Heartbeat) error {
	return s.post(ctx, "/api/agents/heartbeat", heartbeat, nil)
}

func (s *Sender) Flow(ctx context.Context, flow FlowEvent) error {
	return s.post(ctx, "/api/flows/ingest", flow, nil)
}

func (s *Sender) ConnectionTargets(ctx context.Context, agentID string) ([]ConnectionProbeTarget, error) {
	var result []ConnectionProbeTarget
	err := s.get(ctx, "/api/connections/targets?agent_id="+url.QueryEscape(agentID), &result)
	return result, err
}

func (s *Sender) ConnectionProbe(ctx context.Context, probe ConnectionProbeEvent) error {
	return s.post(ctx, "/api/connections/probe", probe, nil)
}

func (s *Sender) get(ctx context.Context, path string, response any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("backend %s returned %s: %s", path, res.Status, strings.TrimSpace(string(body)))
	}
	if response != nil && len(body) > 0 {
		if err := json.Unmarshal(body, response); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sender) post(ctx context.Context, path string, request, response any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("backend %s returned %s: %s", path, res.Status, strings.TrimSpace(string(body)))
	}
	if response != nil && len(body) > 0 {
		if err := json.Unmarshal(body, response); err != nil {
			return err
		}
	}
	return nil
}
