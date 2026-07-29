package exporter

import (
	"time"

	"github.com/vmlens/vmlens/agent/internal/flow"
)

type FlowEvent struct {
	AgentID         string    `json:"agent_id"`
	SrcIP           string    `json:"src_ip"`
	DstIP           string    `json:"dst_ip"`
	SrcPort         int       `json:"src_port"`
	DstPort         int       `json:"dst_port"`
	Protocol        string    `json:"protocol"`
	Direction       string    `json:"direction"`
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	Packets         int64     `json:"packets"`
	ConnectionCount int64     `json:"connection_count"`
	RequestCount    int64     `json:"request_count"`
	ErrorCount      int64     `json:"error_count"`
	Retransmissions int64     `json:"retransmission_count,omitempty"`
	AvgRTTMs        float64   `json:"avg_rtt_ms,omitempty"`
	AvgAppDelayMs   float64   `json:"avg_app_delay_ms,omitempty"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Interface       string    `json:"interface"`
}

func FromFlowState(state flow.State) FlowEvent {
	return FlowEvent{
		AgentID:         state.Key.AgentID,
		SrcIP:           state.Key.SrcIP,
		DstIP:           state.Key.DstIP,
		SrcPort:         state.SrcPort,
		DstPort:         state.Key.DstPort,
		Protocol:        state.Key.Protocol,
		Direction:       state.Key.Direction,
		BytesSent:       int64(state.Traffic.Bytes.Sent),
		BytesReceived:   int64(state.Traffic.Bytes.Received),
		Packets:         int64(state.Traffic.Packets.Count),
		ConnectionCount: int64(state.TCP.Connection.OpenCount),
		RequestCount:    int64(state.TCP.Connection.RequestHint),
		ErrorCount:      int64(state.TCP.Connection.ErrorCount),
		Retransmissions: int64(state.TCP.Retrans.Count),
		AvgRTTMs:        state.TCP.RTT.AvgMs,
		AvgAppDelayMs:   state.Application.AvgDelayMs,
		FirstSeen:       state.FirstSeen,
		LastSeen:        state.LastSeen,
		Interface:       state.Key.Interface,
	}
}
