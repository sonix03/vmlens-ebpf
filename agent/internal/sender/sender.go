package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vmlens/vmlens/agent/internal/model"
)

type Sender struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Sender {
	return &Sender{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}}
}

func (s *Sender) Register(ctx context.Context, registration model.Registration) (model.RegistrationResult, error) {
	var result model.RegistrationResult
	err := s.post(ctx, "/api/agents/register", registration, &result)
	return result, err
}

func (s *Sender) Heartbeat(ctx context.Context, heartbeat model.Heartbeat) error {
	return s.post(ctx, "/api/agents/heartbeat", heartbeat, nil)
}

func (s *Sender) Flow(ctx context.Context, flow model.FlowEvent) error {
	return s.post(ctx, "/api/flows/ingest", flow, nil)
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
