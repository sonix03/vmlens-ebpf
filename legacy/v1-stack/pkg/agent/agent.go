package agent

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmlens/vmlens-ebpf/pkg/collector"
	"github.com/vmlens/vmlens-ebpf/pkg/config"
	"github.com/vmlens/vmlens-ebpf/pkg/correlator"
	"github.com/vmlens/vmlens-ebpf/pkg/exporter"
	jsonlog "github.com/vmlens/vmlens-ebpf/pkg/logger"
	"github.com/vmlens/vmlens-ebpf/pkg/model"
)

type Agent struct {
	cfg           config.Config
	log           *jsonlog.JSONLogger
	corr          *correlator.SessionCorrelator
	metrics       *exporter.Exporter
	mu            sync.RWMutex
	pids          map[int]bool
	cpuSince      map[int]time.Time
	previous      map[int]model.ResourceEvent
	networkWindow map[int]networkPoint
}

type networkPoint struct {
	at    time.Time
	bytes uint64
}

func New(cfg config.Config) (*Agent, error) {
	l, err := jsonlog.New(cfg.Logging.SSHLogPath, cfg.Logging.ProcessLogPath, cfg.Logging.ResourceLogPath, cfg.Logging.NetworkLogPath, cfg.Logging.AnalysisLogPath)
	if err != nil {
		return nil, err
	}
	return &Agent{cfg: cfg, log: l, corr: correlator.New(), metrics: exporter.New(), pids: map[int]bool{}, cpuSince: map[int]time.Time{}, previous: map[int]model.ResourceEvent{}, networkWindow: map[int]networkPoint{}}, nil
}

func (a *Agent) Close() error { 
	return a.log.Close() 
}

func (a *Agent) PIDs() []int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]int, 0, len(a.pids))
	for p := range a.pids {
		out = append(out, p)
	}
	return out
}

func (a *Agent) Run(ctx context.Context) error {
	bpf, err := collector.LoadBPF(a.cfg.Collection.BPFDir)
	var processEvents <-chan model.ProcessEvent
	var networkEvents <-chan model.NetworkFlow
	var bpfErrors <-chan error
	if err != nil {
		log.Printf("eBPF unavailable: %v", err)
	} else {
		log.Printf("eBPF probes loaded from %s", a.cfg.Collection.BPFDir)
		defer bpf.Close()
		processEvents = bpf.ProcessEvents
		networkEvents = bpf.NetworkEvents
		bpfErrors = bpf.Errors
	}
	ssh := collector.NewSSH(a.cfg)
	resources := collector.NewResource(a.cfg.Collection.SampleInterval, a.PIDs)
	go ssh.Run(ctx)
	go resources.Run(ctx)
	if bpf == nil {
		execs := collector.NewExec(a.cfg.Collection.SampleInterval, a.cfg.Privacy.SanitizeCommandArgs)
		network := collector.NewNet(a.cfg.Collection.SampleInterval, a.PIDs)
		go execs.Run(ctx)
		go network.Run(ctx)
		processEvents = execs.Events
		networkEvents = network.Events
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(a.metrics.Registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: a.cfg.Server.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server: %v", err)
			a.metrics.Errors.Inc()
		}
	}()
	defer srv.Shutdown(context.Background())
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-ssh.Errors:
			if err != nil {
				log.Printf("ssh collector: %v", err)
				a.metrics.Errors.Inc()
			}
		case err := <-bpfErrors:
			if err != nil {
				log.Printf("eBPF reader: %v", err)
				a.metrics.Errors.Inc()
			}
		case s, ok := <-ssh.Events:
			if ok {
				a.handleSSH(s)
			}
		case p, ok := <-processEvents:
			if ok {
				a.handleProcess(p)
			}
		case r, ok := <-resources.Events:
			if ok {
				a.handleResource(r)
			}
		case n, ok := <-networkEvents:
			if ok {
				a.handleNetwork(n)
			}
		}
	}
}

func (a *Agent) handleSSH(s model.SSHSession) {
	if s.EventType == "ssh_logout" {
		for _, old := range a.corr.Sessions() {
			if old.SSHPID == s.SSHPID && old.Status == "active" {
				s.SessionID = old.SessionID
				s.RemoteIP = old.RemoteIP
				s.RemotePort = old.RemotePort
				s.AuthMethod = old.AuthMethod
				s.StartTime = old.StartTime
				s.TTY = old.TTY
				if s.User == "" {
					s.User = old.User
				}
				break
			}
		}
	}
	if !a.cfg.Privacy.LogRemoteIP {
		s.RemoteIP = ""
	} else if a.cfg.Privacy.AnonymizeIP {
		s.RemoteIP = anonymizeIP(s.RemoteIP)
	}
	a.corr.UpsertSession(s)
	a.metrics.SSH(s)
	if err := a.log.Write(a.cfg.Logging.SSHLogPath, s); err != nil {
		a.err(err)
	}
}

func (a *Agent) handleProcess(p model.ProcessEvent) {
	a.corr.AttributeProcess(&p)
	if s, ok := a.corr.SetTTY(p.SessionID, p.TTY); ok {
		s.EventType = "ssh_session_update"
		s.Timestamp = p.Timestamp
		if err := a.log.Write(a.cfg.Logging.SSHLogPath, s); err != nil {
			a.err(err)
		}
	}
	a.mu.Lock()
	if p.EventType == "process_exec" {
		a.pids[p.PID] = true
	} else {
		delete(a.pids, p.PID)
	}
	a.mu.Unlock()
	a.metrics.Process(p)
	if err := a.log.Write(a.cfg.Logging.ProcessLogPath, p); err != nil {
		a.err(err)
	}
}

func (a *Agent) handleResource(r model.ResourceEvent) {
	r.SessionID = a.corr.SessionForPID(r.PID)
	a.metrics.Resource(r)
	if err := a.log.Write(a.cfg.Logging.ResourceLogPath, r); err != nil {
		a.err(err)
	}
	for _, summary := range a.analyze(r) {
		a.metrics.Analyzed(summary)
		if err := a.log.Write(a.cfg.Logging.AnalysisLogPath, summary); err != nil {
			a.err(err)
		}
	}
}

func (a *Agent) handleNetwork(n model.NetworkFlow) {
	n.SessionID = a.corr.SessionForPID(n.PID)
	if !a.cfg.Privacy.LogDestinationIP {
		n.DstIP = ""
	} else if a.cfg.Privacy.AnonymizeIP {
		n.DstIP = anonymizeIP(n.DstIP)
	}
	a.metrics.Network(n)
	if err := a.log.Write(a.cfg.Logging.NetworkLogPath, n); err != nil {
		a.err(err)
	}
	bytesNow := n.RXBytes + n.TXBytes
	point := a.networkWindow[n.PID]
	if point.at.IsZero() || n.Timestamp.Sub(point.at) >= time.Minute {
		a.networkWindow[n.PID] = networkPoint{n.Timestamp, bytesNow}
	} else if bytesNow >= point.bytes && bytesNow-point.bytes >= a.cfg.Analysis.NetworkHighBytesPerMinute {
		session, _ := a.corr.Session(n.SessionID)
		summary := model.AnalysisSummary{EventType: "analysis_summary", Timestamp: n.Timestamp, Severity: "warning", Reason: "network_spike", SessionID: n.SessionID, User: session.User, RemoteIP: session.RemoteIP, TopProcess: n.Process, PID: n.PID, Summary: fmt.Sprintf("Process %s transferred %d network bytes within one minute.", n.Process, bytesNow-point.bytes)}
		a.metrics.Analyzed(summary)
		if err := a.log.Write(a.cfg.Logging.AnalysisLogPath, summary); err != nil {
			a.err(err)
		}
		a.networkWindow[n.PID] = networkPoint{n.Timestamp, bytesNow}
	}
}
func (a *Agent) analyze(r model.ResourceEvent) []model.AnalysisSummary {
	now := r.Timestamp
	var out []model.AnalysisSummary
	session, _ := a.corr.Session(r.SessionID)
	makeOne := func(reason, msg string) model.AnalysisSummary {
		return model.AnalysisSummary{EventType: "analysis_summary", Timestamp: now, Severity: "warning", Reason: reason, SessionID: r.SessionID, User: session.User, RemoteIP: session.RemoteIP, TopProcess: r.Process, PID: r.PID, Summary: msg}
	}
	if r.CPUPercent > a.cfg.Analysis.CPUHighPercent {
		if a.cpuSince[r.PID].IsZero() {
			a.cpuSince[r.PID] = now
		} else if now.Sub(a.cpuSince[r.PID]) >= time.Duration(a.cfg.Analysis.CPUHighDurationSeconds)*time.Second {
			out = append(out, makeOne("cpu_high", fmt.Sprintf("Process %s sustained %.1f%% CPU.", r.Process, r.CPUPercent)))
			a.cpuSince[r.PID] = now
		}
	} else {
		delete(a.cpuSince, r.PID)
	}
	if r.RSSBytes > a.cfg.Analysis.MemoryHighBytes {
		out = append(out, makeOne("memory_high", fmt.Sprintf("Process %s RSS is %d bytes.", r.Process, r.RSSBytes)))
	}
	if prev, ok := a.previous[r.PID]; ok {
		readDelta := delta(r.DiskReadBytes, prev.DiskReadBytes)
		writeDelta := delta(r.DiskWriteBytes, prev.DiskWriteBytes)
		elapsed := r.Timestamp.Sub(prev.Timestamp).Seconds()
		perMinute := uint64(0)
		if elapsed > 0 {
			perMinute = uint64(float64(readDelta+writeDelta) * 60 / elapsed)
		}
		if perMinute > a.cfg.Analysis.DiskHighBytesPerMinute {
			out = append(out, makeOne("disk_high", fmt.Sprintf("Process %s transferred %d disk bytes in the sample window.", r.Process, readDelta+writeDelta)))
		}
	}
	a.previous[r.PID] = r
	return out
}
func delta(v, old uint64) uint64 {
	if v >= old {
		return v - old
	}
	return 0
}
func anonymizeIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		parts[3] = "0"
		return strings.Join(parts, ".")
	}
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		return ip[:i] + ":0"
	}
	return ip
}
func (a *Agent) err(err error) { log.Printf("vmlens: %v", err); a.metrics.Errors.Inc() }
