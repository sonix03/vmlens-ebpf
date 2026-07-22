package deepflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vmlens/vmlens/backend/internal/model"
)

type QueryFilter struct {
	Window     time.Duration
	Limit      int
	AllowedIPs []string
}

func (c *Client) QueryL4Flows(ctx context.Context, filter QueryFilter) ([]model.DeepFlowL4Flow, error) {
	where := deepFlowWhere(filter)
	sql := fmt.Sprintf(`
SELECT
  time,
  toString(ip4_0) AS source_ip,
  toString(ip4_1) AS dest_ip,
  concat(toString(if(l3_epc_id_0=-2,1,0)), ' -> ', toString(if(l3_epc_id_1=-2,1,0))) AS internet_direction,
  client_port,
  server_port,
  multiIf(protocol=1, 'icmp', protocol=6, 'tcp', protocol=17, 'udp', toString(protocol)) AS protocol,
  toString(status) AS status,
  byte_tx,
  byte_rx,
  byte_tx + byte_rx AS total_bytes,
  round(rtt/1000,3) AS rtt_ms,
  retrans_tx + retrans_rx AS retrans_total,
  toString(agent_id) AS agent_id,
  l3_epc_id_0,
  l3_epc_id_1
FROM flow_log.l4_flow_log
WHERE %s
ORDER BY time DESC
LIMIT %d`, where, safeLimit(filter.Limit, c.cfg.MaxLimit))

	rows := []model.DeepFlowL4Flow{}
	err := c.QueryJSONEachRow(ctx, sql, func(raw json.RawMessage) error {
		var item rawL4Flow
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		rows = append(rows, item.model())
		return nil
	})
	return rows, err
}

func (c *Client) QueryL7Requests(ctx context.Context, filter QueryFilter) ([]model.DeepFlowL7Request, error) {
	where := deepFlowWhere(filter)
	sql := fmt.Sprintf(`
SELECT
  time,
  toString(ip4_0) AS source_ip,
  toString(ip4_1) AS dest_ip,
  concat(toString(if(l3_epc_id_0=-2,1,0)), ' -> ', toString(if(l3_epc_id_1=-2,1,0))) AS internet_direction,
  request_type,
  request_domain,
  request_resource,
  response_code,
  round(response_duration/1000,3) AS response_duration_ms,
  request_length,
  response_length,
  l7_protocol_str,
  toString(agent_id) AS agent_id,
  observation_point,
  l3_epc_id_0,
  l3_epc_id_1
FROM flow_log.l7_flow_log
WHERE %s
ORDER BY time DESC
LIMIT %d`, where, safeLimit(filter.Limit, c.cfg.MaxLimit))

	rows := []model.DeepFlowL7Request{}
	err := c.QueryJSONEachRow(ctx, sql, func(raw json.RawMessage) error {
		var item rawL7Request
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		rows = append(rows, item.model())
		return nil
	})
	return rows, err
}

func (c *Client) QueryAgentMappings(ctx context.Context) ([]model.DeepFlowAgentMapping, error) {
	sql := `
SELECT
  toString(v.id) AS agent_id,
  v.name AS agent_name,
  ifNull(nullIf(p.device_name, ''), v.name) AS vm_name,
  ifNull(p.name, '') AS interface_name,
  ifNull(toString(p.tap_port), '') AS tap_port
FROM flow_tag.vtap_map AS v
LEFT JOIN flow_tag.vtap_port_map AS p
  ON v.id = p.vtap_id AND p.name != 'lo'
ORDER BY v.id
LIMIT 5000`

	rows := []model.DeepFlowAgentMapping{}
	err := c.QueryJSONEachRow(ctx, sql, func(raw json.RawMessage) error {
		var item rawAgentMapping
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		rows = append(rows, item.model())
		return nil
	})
	return rows, err
}

func (c *Client) LatestTimestamp(ctx context.Context, table string) (*time.Time, error) {
	if table != "flow_log.l4_flow_log" && table != "flow_log.l7_flow_log" {
		return nil, fmt.Errorf("unsupported table %s", table)
	}
	sql := fmt.Sprintf(`SELECT time AS latest FROM %s ORDER BY time DESC LIMIT 1`, table)
	var latest *time.Time
	err := c.QueryJSONEachRow(ctx, sql, func(raw json.RawMessage) error {
		var item struct {
			Latest flexibleTime `json:"latest"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		value := time.Time(item.Latest)
		if !value.IsZero() {
			latest = &value
		}
		return nil
	})
	return latest, err
}

type rawL4Flow struct {
	Time              flexibleTime   `json:"time"`
	SourceIP          string         `json:"source_ip"`
	DestIP            string         `json:"dest_ip"`
	ClientPort        flexibleInt    `json:"client_port"`
	ServerPort        flexibleInt    `json:"server_port"`
	Protocol          flexibleString `json:"protocol"`
	Status            flexibleString `json:"status"`
	ByteTX            flexibleInt64  `json:"byte_tx"`
	ByteRX            flexibleInt64  `json:"byte_rx"`
	TotalBytes        flexibleInt64  `json:"total_bytes"`
	RTTMs             flexibleFloat  `json:"rtt_ms"`
	RetransTotal      flexibleInt64  `json:"retrans_total"`
	AgentID           flexibleString `json:"agent_id"`
	L3EPCID0          flexibleInt    `json:"l3_epc_id_0"`
	L3EPCID1          flexibleInt    `json:"l3_epc_id_1"`
	InternetDirection string         `json:"internet_direction"`
}

func (r rawL4Flow) model() model.DeepFlowL4Flow {
	return model.DeepFlowL4Flow{
		Time:              time.Time(r.Time),
		SourceIP:          strings.TrimSpace(r.SourceIP),
		DestIP:            strings.TrimSpace(r.DestIP),
		ClientPort:        int(r.ClientPort),
		ServerPort:        int(r.ServerPort),
		Protocol:          normalizeProtocol(string(r.Protocol)),
		Status:            string(r.Status),
		ByteTX:            int64(r.ByteTX),
		ByteRX:            int64(r.ByteRX),
		TotalBytes:        int64(r.TotalBytes),
		RTTMs:             float64(r.RTTMs),
		RetransTotal:      int64(r.RetransTotal),
		AgentID:           string(r.AgentID),
		L3EPCID0:          int(r.L3EPCID0),
		L3EPCID1:          int(r.L3EPCID1),
		InternetDirection: r.InternetDirection,
	}
}

type rawL7Request struct {
	Time               flexibleTime   `json:"time"`
	SourceIP           string         `json:"source_ip"`
	DestIP             string         `json:"dest_ip"`
	RequestType        flexibleString `json:"request_type"`
	RequestDomain      flexibleString `json:"request_domain"`
	RequestResource    flexibleString `json:"request_resource"`
	ResponseCode       flexibleInt    `json:"response_code"`
	ResponseDurationMs flexibleFloat  `json:"response_duration_ms"`
	RequestLength      flexibleInt64  `json:"request_length"`
	ResponseLength     flexibleInt64  `json:"response_length"`
	L7Protocol         flexibleString `json:"l7_protocol_str"`
	AgentID            flexibleString `json:"agent_id"`
	ObservationPoint   flexibleString `json:"observation_point"`
	InternetDirection  string         `json:"internet_direction"`
	L3EPCID0           flexibleInt    `json:"l3_epc_id_0"`
	L3EPCID1           flexibleInt    `json:"l3_epc_id_1"`
}

func (r rawL7Request) model() model.DeepFlowL7Request {
	return model.DeepFlowL7Request{
		Time:               time.Time(r.Time),
		SourceIP:           strings.TrimSpace(r.SourceIP),
		DestIP:             strings.TrimSpace(r.DestIP),
		RequestType:        string(r.RequestType),
		RequestDomain:      string(r.RequestDomain),
		RequestResource:    string(r.RequestResource),
		ResponseCode:       int(r.ResponseCode),
		ResponseDurationMs: float64(r.ResponseDurationMs),
		RequestLength:      int64(r.RequestLength),
		ResponseLength:     int64(r.ResponseLength),
		L7Protocol:         string(r.L7Protocol),
		AgentID:            string(r.AgentID),
		ObservationPoint:   normalizeObservationPoint(string(r.ObservationPoint)),
		InternetDirection:  r.InternetDirection,
		L3EPCID0:           int(r.L3EPCID0),
		L3EPCID1:           int(r.L3EPCID1),
	}
}

type rawAgentMapping struct {
	AgentID       flexibleString `json:"agent_id"`
	AgentName     flexibleString `json:"agent_name"`
	VMName        flexibleString `json:"vm_name"`
	InterfaceName flexibleString `json:"interface_name"`
	TapPort       flexibleInt    `json:"tap_port"`
}

func (r rawAgentMapping) model() model.DeepFlowAgentMapping {
	return model.DeepFlowAgentMapping{
		AgentID:       string(r.AgentID),
		AgentName:     string(r.AgentName),
		VMName:        string(r.VMName),
		InterfaceName: string(r.InterfaceName),
		TapPort:       int(r.TapPort),
	}
}

type flexibleString string

func (v *flexibleString) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*v = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*v = flexibleString(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		*v = flexibleString(n.String())
		return nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		*v = flexibleString(strconv.FormatBool(b))
		return nil
	}
	return nil
}

type flexibleInt int

func (v *flexibleInt) UnmarshalJSON(raw []byte) error {
	value, _ := parseFloatJSON(raw)
	*v = flexibleInt(int(value))
	return nil
}

type flexibleInt64 int64

func (v *flexibleInt64) UnmarshalJSON(raw []byte) error {
	value, _ := parseFloatJSON(raw)
	*v = flexibleInt64(int64(value))
	return nil
}

type flexibleFloat float64

func (v *flexibleFloat) UnmarshalJSON(raw []byte) error {
	value, _ := parseFloatJSON(raw)
	*v = flexibleFloat(value)
	return nil
}

type flexibleTime time.Time

func (v *flexibleTime) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" || string(raw) == `""` {
		*v = flexibleTime(time.Time{})
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	parsed, err := parseDeepFlowTime(s)
	if err != nil {
		return err
	}
	*v = flexibleTime(parsed)
	return nil
}

func parseFloatJSON(raw []byte) (float64, error) {
	if string(raw) == "null" || string(raw) == `""` {
		return 0, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, nil
		}
		return f, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func parseDeepFlowTime(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "0000-00-00") {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse deepflow time %q", raw)
}

func deepFlowWhere(filter QueryFilter) string {
	window := int(filter.Window.Seconds())
	if window <= 0 {
		window = int((30 * time.Minute).Seconds())
	}
	conditions := []string{fmt.Sprintf("time > now() - INTERVAL %d SECOND", window)}
	allowed := quoteStrings(filter.AllowedIPs)
	if len(allowed) > 0 {
		conditions = append(conditions, fmt.Sprintf("(toString(ip4_0) IN (%s) OR toString(ip4_1) IN (%s))", strings.Join(allowed, ","), strings.Join(allowed, ",")))
	}
	return strings.Join(conditions, " AND ")
}

func quoteStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, "'"+strings.ReplaceAll(value, "'", "''")+"'")
	}
	sort.Strings(out)
	return out
}

func safeLimit(value, max int) int {
	if max <= 0 {
		max = 1000
	}
	if value <= 0 {
		value = 100
	}
	if value > max {
		return max
	}
	return value
}

func normalizeProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "icmp":
		return "icmp"
	case "6", "tcp":
		return "tcp"
	case "17", "udp":
		return "udp"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeObservationPoint(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "server process", "server_process", "sp", "s-p":
		return "s-p"
	case "server nic", "server_nic", "s":
		return "s"
	case "client process", "client_process", "cp", "c-p":
		return "c-p"
	case "client nic", "client_nic", "c":
		return "c"
	default:
		return value
	}
}
