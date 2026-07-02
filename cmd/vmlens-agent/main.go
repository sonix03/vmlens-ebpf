package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vmlens/vmlens-ebpf/internal/classifier"
	"github.com/vmlens/vmlens-ebpf/internal/collector"
	"github.com/vmlens/vmlens-ebpf/internal/config"
	vebpf "github.com/vmlens/vmlens-ebpf/internal/ebpf"
	"github.com/vmlens/vmlens-ebpf/internal/metadata"
	"github.com/vmlens/vmlens-ebpf/internal/metrics"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath   string
		demoMode     bool
		forceFlowLog bool
		listen       string
	)
	flag.StringVar(&configPath, "config", "config/vmlens.example.yaml", "path to YAML configuration")
	flag.BoolVar(&demoMode, "demo", false, "generate realistic demo events without loading eBPF")
	flag.BoolVar(&forceFlowLog, "flowlog", false, "force-enable JSONL flow logging")
	flag.StringVar(&listen, "listen", "", "override metrics listen address")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if listen != "" {
		cfg.Agent.ListenAddr = listen
	}
	if forceFlowLog {
		cfg.FlowLog.Enabled = true
	}
	trafficClassifier, err := classifier.New(cfg.Network.InternalCIDRs)
	if err != nil {
		return err
	}
	vm := metadata.Collect(cfg.VM)
	vmJSON, _ := json.Marshal(vm)
	log.Printf("VM metadata: %s", vmJSON)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exporter := metrics.New(vm)
	sinks := []collector.Sink{exporter}
	var flowLogger *collector.FlowLogger
	if cfg.FlowLog.Enabled {
		flowLogger, err = collector.NewFlowLogger(cfg.FlowLog.Path)
		if err != nil {
			return err
		}
		defer flowLogger.Close()
		sinks = append(sinks, flowLogger)
		log.Printf("JSONL flow log enabled: %s", cfg.FlowLog.Path)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(exporter.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, "ok")
	})
	server := &http.Server{
		Addr: cfg.Agent.ListenAddr, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("VMLens %s listening on %s", version, cfg.Agent.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	var rawEvents <-chan vebpf.Event
	var handle *vebpf.Handle
	if demoMode {
		rawEvents = collector.Demo(ctx, vm)
		exporter.AgentUp.WithLabelValues(vm.VMID, vm.Hostname).Set(1)
		log.Printf("demo mode enabled; no kernel eBPF privileges are required")
	} else {
		handle, err = vebpf.Load(cfg.Network.BPFObject)
		if err != nil {
			// Missing privilege/object is non-fatal: metrics and health remain
			// reachable with vmlens_agent_up=0 for operational diagnosis.
			log.Printf("eBPF unavailable: %v", err)
			log.Printf("agent remains online without events; use --demo to test the dashboard")
			idle := make(chan vebpf.Event)
			rawEvents = idle
		} else {
			defer handle.Close()
			rawEvents = handle.Events
			exporter.AgentUp.WithLabelValues(vm.VMID, vm.Hostname).Set(1)
			go func() {
				for err := range handle.Errors {
					log.Printf("eBPF reader: %v", err)
				}
			}()
			log.Printf("eBPF object loaded: %s", cfg.Network.BPFObject)
		}
	}

	normalizer := collector.New(trafficClassifier, vm, cfg.Privacy)
	aggregator := collector.NewAggregator(sinks...)
	go aggregator.Run(ctx, normalizer.Run(ctx, rawEvents))
	go func() {
		for err := range aggregator.Errors {
			log.Printf("collector: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		stop()
		<-aggregator.Done
		return fmt.Errorf("metrics server: %w", err)
	}
	stop()
	 
	<-aggregator.Done
	exporter.AgentUp.WithLabelValues(vm.VMID, vm.Hostname).Set(0)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
