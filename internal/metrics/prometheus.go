package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/vmlens/vmlens-ebpf/internal/collector"
	"github.com/vmlens/vmlens-ebpf/internal/metadata"
)

type Exporter struct {
	Registry           *prometheus.Registry
	AgentUp            *prometheus.GaugeVec
	networkBytes       *prometheus.CounterVec
	networkPackets     *prometheus.CounterVec
	networkConnections *prometheus.CounterVec
	activeConnections  *prometheus.GaugeVec
	processExec        *prometheus.CounterVec
	sshSessions        *prometheus.CounterVec
	vm                 metadata.VM
}

func New(vm metadata.VM) *Exporter {
	registry := prometheus.NewRegistry()
	// IPs, PIDs, command lines, domains and session IDs are intentionally absent:
	// they are unbounded/high-cardinality and belong in the optional JSONL log.
	networkLabels := []string{"vm_id", "hostname", "direction", "scope", "protocol", "process", "port_class", "interface"}
	e := &Exporter{
		Registry: registry, vm: vm,
		AgentUp:            prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_agent_up", Help: "Whether the VMLens data source is operating (1) or unavailable (0)."}, []string{"vm_id", "hostname"}),
		networkBytes:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_network_bytes_total", Help: "Observed TCP application bytes by low-cardinality dimensions."}, networkLabels),
		networkPackets:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_network_packets_total", Help: "Observed packet count when the event source provides it."}, networkLabels),
		networkConnections: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_network_connections_total", Help: "Observed TCP connection attempts/accepts."}, networkLabels),
		activeConnections:  prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_network_active_connections", Help: "Recent connection activity estimate over a 30-second window."}, networkLabels),
		processExec:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_process_exec_total", Help: "Observed process exec events."}, []string{"vm_id", "hostname", "process"}),
		sshSessions:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_ssh_sessions_total", Help: "Best-effort inbound SSH connection count."}, []string{"vm_id", "hostname"}),
	}
	registry.MustRegister(e.AgentUp, e.networkBytes, e.networkPackets, e.networkConnections, e.activeConnections, e.processExec, e.sshSessions)
	e.AgentUp.WithLabelValues(vm.VMID, vm.Hostname).Set(0)
	return e
}

func (e *Exporter) Observe(event collector.Event) error {
	if event.Kind == "process_exec" {
		e.processExec.WithLabelValues(e.vm.VMID, e.vm.Hostname, safe(event.Process)).Inc()
		return nil
	}
	labels := []string{
		e.vm.VMID, e.vm.Hostname, safe(event.Direction), safe(event.Scope),
		safe(event.Protocol), safe(event.Process), safe(event.PortClass), safe(event.Interface),
	}
	e.networkBytes.WithLabelValues(labels...).Add(float64(event.BytesSent + event.BytesReceived))
	e.networkPackets.WithLabelValues(labels...).Add(float64(event.PacketsSent + event.PacketsReceived))
	if event.ConnectionCount > 0 {
		count := float64(event.ConnectionCount)
		e.networkConnections.WithLabelValues(labels...).Add(count)
		gauge := e.activeConnections.WithLabelValues(labels...)
		gauge.Add(count)
		// The MVP does not observe close reliably, so this gauge represents recent
		// connection activity rather than a kernel socket-table census.
		time.AfterFunc(30*time.Second, func() { gauge.Sub(count) })
	}
	if event.Direction == "ingress" && event.DstPort == 22 && event.Process == "sshd" && event.ConnectionCount > 0 {
		e.sshSessions.WithLabelValues(e.vm.VMID, e.vm.Hostname).Add(float64(event.ConnectionCount))
	}
	return nil
}

func safe(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
