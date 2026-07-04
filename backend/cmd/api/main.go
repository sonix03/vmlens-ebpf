package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apihttp "github.com/vmlens/vmlens/backend/internal/api"
	"github.com/vmlens/vmlens/backend/internal/config"
	"github.com/vmlens/vmlens/backend/internal/db"
	"github.com/vmlens/vmlens/backend/internal/realtime"
	"github.com/vmlens/vmlens/backend/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	classifier, err := service.NewClassifier(cfg.InternalCIDRs)
	if err != nil {
		return err
	}
	hub := realtime.New()
	agents := service.NewAgentService(pool, hub)
	vms := service.NewVMService(pool)
	flows := service.NewFlowService(pool, classifier, hub)
	graph := service.NewGraphService(pool, vms, cfg.FlowActiveWindow)
	stats := service.NewStatsService(pool)
	handlers := &apihttp.Handlers{Pool: pool, Agents: agents, VMs: vms, Flows: flows, Graph: graph, Stats: stats}

	server := &http.Server{Addr: cfg.ListenAddr, Handler: apihttp.Routes(handlers, hub, cfg.AllowedOrigins), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("VMLens API listening on %s", cfg.ListenAddr)
		serverErrors <- server.ListenAndServe()
	}()
	go func() {
		ticker := time.NewTicker(cfg.StatusSweepPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := agents.UpdateStatuses(context.Background()); err != nil {
					log.Printf("status sweep: %v", err)
				}
				deleted, err := agents.DeleteExpired(context.Background(), cfg.VMDeleteAfter)
				if err != nil {
					log.Printf("expired VM cleanup: %v", err)
				} else if deleted > 0 {
					log.Printf("expired VM cleanup: deleted %d node(s)", deleted)
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
