package deepflow

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vmlens/vmlens/backend/internal/config"
)

type Client struct {
	cfg        config.DeepFlowConfig
	httpClient *http.Client
}

func NewClient(cfg config.DeepFlowConfig) *Client {
	timeout := cfg.QueryTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Enabled() bool { return c.cfg.Enabled }

func (c *Client) Config() config.DeepFlowConfig { return c.cfg }

func (c *Client) QueryJSONEachRow(ctx context.Context, sql string, scan func(json.RawMessage) error) error {
	if !c.cfg.Enabled {
		return fmt.Errorf("deepflow is disabled")
	}
	if strings.TrimSpace(c.cfg.ClickHouseURL) == "" {
		return fmt.Errorf("DEEPFLOW_CLICKHOUSE_URL is not configured")
	}
	if !strings.Contains(strings.ToUpper(sql), "FORMAT JSONEACHROW") {
		sql = strings.TrimRight(sql, " ;\n\t") + " FORMAT JSONEachRow"
	}

	endpoint, err := url.Parse(c.cfg.ClickHouseURL)
	if err != nil {
		return fmt.Errorf("parse DEEPFLOW_CLICKHOUSE_URL: %w", err)
	}
	query := endpoint.Query()
	if c.cfg.ClickHouseDatabase != "" {
		query.Set("database", c.cfg.ClickHouseDatabase)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(sql))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if c.cfg.ClickHouseUsername != "" {
		req.SetBasicAuth(c.cfg.ClickHouseUsername, c.cfg.ClickHousePassword)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("query clickhouse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("clickhouse status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := scan(append(json.RawMessage(nil), line...)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (c *Client) PingClickHouse(ctx context.Context) error {
	return c.QueryJSONEachRow(ctx, `SELECT 1 AS ok`, func(raw json.RawMessage) error { return nil })
}

func (c *Client) PingHTTP(ctx context.Context, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("url is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(rawURL, "/"), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 500 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
