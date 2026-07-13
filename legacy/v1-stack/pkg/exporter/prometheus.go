package exporter

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

type Exporter struct {
	Registry                         *prometheus.Registry
	Active                           prometheus.Gauge
	Logins                           *prometheus.CounterVec
	Execs                            *prometheus.CounterVec
	CPU, Memory, DiskRead, DiskWrite *prometheus.GaugeVec
	NetRX, NetTX                     *prometheus.CounterVec
	Analysis                         *prometheus.CounterVec
	Errors                           prometheus.Counter
}

func New() *Exporter {
	e := &Exporter{Registry: prometheus.NewRegistry()}
	e.Active = prometheus.NewGauge(prometheus.GaugeOpts{Name: "vmlens_ssh_sessions_active", Help: "Current active SSH sessions."})
	e.Logins = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_ssh_logins_total", Help: "Observed SSH logins."}, []string{"auth_method"})
	e.Execs = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_process_exec_total", Help: "Observed process executions."}, []string{"user", "process"})
	e.CPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_process_cpu_percent", Help: "Latest process CPU usage."}, []string{"process"})
	e.Memory = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_process_memory_rss_bytes", Help: "Latest process RSS."}, []string{"process"})
	e.DiskRead = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_process_disk_read_bytes_total", Help: "Process read bytes from /proc."}, []string{"process"})
	e.DiskWrite = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmlens_process_disk_write_bytes_total", Help: "Process write bytes from /proc."}, []string{"process"})
	e.NetRX = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_network_rx_bytes_total", Help: "Best-effort network receive bytes."}, []string{"process", "protocol", "dst_port"})
	e.NetTX = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_network_tx_bytes_total", Help: "Best-effort network transmit bytes."}, []string{"process", "protocol", "dst_port"})
	e.Analysis = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmlens_analysis_events_total", Help: "Rule analysis events."}, []string{"reason", "severity"})
	e.Errors = prometheus.NewCounter(prometheus.CounterOpts{Name: "vmlens_agent_errors_total", Help: "Agent errors."})
	e.Registry.MustRegister(e.Active, e.Logins, e.Execs, e.CPU, e.Memory, e.DiskRead, e.DiskWrite, e.NetRX, e.NetTX, e.Analysis, e.Errors)
	return e
}
func (e *Exporter) SSH(s model.SSHSession) {
	if s.EventType == "ssh_login" {
		e.Active.Inc()
		e.Logins.WithLabelValues(s.AuthMethod).Inc()
	} else if s.EventType == "ssh_logout" {
		e.Active.Dec()
	}
}
func (e *Exporter) Process(p model.ProcessEvent) {
	if p.EventType == "process_exec" {
		e.Execs.WithLabelValues(p.User, p.Process).Inc()
	}
}
func (e *Exporter) Resource(r model.ResourceEvent) {
	e.CPU.WithLabelValues(r.Process).Set(r.CPUPercent)
	e.Memory.WithLabelValues(r.Process).Set(float64(r.RSSBytes))
	e.DiskRead.WithLabelValues(r.Process).Set(float64(r.DiskReadBytes))
	e.DiskWrite.WithLabelValues(r.Process).Set(float64(r.DiskWriteBytes))
}
func (e *Exporter) Network(n model.NetworkFlow) {
	port := strconv.Itoa(n.DstPort)
	e.NetRX.WithLabelValues(n.Process, n.Protocol, port).Add(float64(n.RXBytes))
	e.NetTX.WithLabelValues(n.Process, n.Protocol, port).Add(float64(n.TXBytes))
}
func (e *Exporter) Analyzed(a model.AnalysisSummary) {
	e.Analysis.WithLabelValues(a.Reason, a.Severity).Inc()
}
