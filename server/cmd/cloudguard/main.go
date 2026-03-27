package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/trekmax/cloudguard-monitor/internal/api"
	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/config"
	"github.com/trekmax/cloudguard-monitor/internal/logging"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := logging.Setup(cfg.Log.Level, cfg.Log.Format)
	logger.Info("starting CloudGuard Monitor", "version", "0.1.0")

	// Ensure data directory exists
	dbDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		logger.Error("failed to create data directory", "path", dbDir, "error", err)
		os.Exit(1)
	}

	// Initialize store
	st, err := store.New(cfg.Database.Path, logger)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Collect and store system info
	sysInfo := collector.CollectSystemInfo()
	logger.Info("system info collected",
		"hostname", sysInfo.Hostname,
		"os", sysInfo.OS,
		"arch", sysInfo.Arch,
		"cores", sysInfo.CPUCores,
	)
	persistSystemInfo(st, sysInfo)

	// Initialize and start scheduler
	sched := collector.NewScheduler(logger)
	sched.Register(collector.NewCPUCollector())
	sched.Register(collector.NewMemoryCollector())
	sched.Register(collector.NewDiskCollector())
	sched.Register(collector.NewNetworkCollector())
	sched.Register(collector.NewProcessCollector())
	sched.Start(ctx)
	logger.Info("collectors started", "count", 5)

	// Start metrics persistence goroutine
	go persistMetrics(ctx, logger, sched, st)

	// Start cleanup goroutine
	go runCleanup(ctx, logger, st, cfg.Database.RetentionDays)

	// Setup and start API server
	srv := api.NewServer(logger, st, sched, sysInfo, cfg.Auth.Token)
	router := srv.SetupRouter()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		logger.Info("API server starting", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)

	sched.Stop()
	logger.Info("goodbye")
}

// persistMetrics periodically saves collected metrics to the database.
func persistMetrics(ctx context.Context, logger *slog.Logger, sched *collector.Scheduler, st *store.Store) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := sched.Latest()
			var records []store.MetricRecord
			ts := snap.Timestamp.Unix()

			for _, metrics := range snap.Metrics {
				for _, m := range metrics {
					labelsJSON := ""
					if len(m.Labels) > 0 {
						b, _ := json.Marshal(m.Labels)
						labelsJSON = string(b)
					}
					for name, value := range m.Values {
						records = append(records, store.MetricRecord{
							Timestamp: ts,
							Category:  m.Category,
							Name:      name,
							Value:     value,
							Labels:    labelsJSON,
						})
					}
				}
			}

			if len(records) > 0 {
				if err := st.InsertMetrics(records); err != nil {
					logger.Error("failed to persist metrics", "error", err)
				}
			}
		}
	}
}

// runCleanup periodically removes old data.
func runCleanup(ctx context.Context, logger *slog.Logger, st *store.Store, retentionDays int) {
	if retentionDays <= 0 {
		retentionDays = 30
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := st.Cleanup(time.Duration(retentionDays) * 24 * time.Hour)
			if err != nil {
				logger.Error("cleanup failed", "error", err)
			} else if deleted > 0 {
				logger.Info("cleanup completed", "deleted", deleted)
			}
		}
	}
}

func persistSystemInfo(st *store.Store, info *collector.SystemInfo) {
	st.SetSystemInfo("hostname", info.Hostname)
	st.SetSystemInfo("os", info.OS)
	st.SetSystemInfo("platform", info.Platform)
	st.SetSystemInfo("kernel", info.Kernel)
	st.SetSystemInfo("arch", info.Arch)
	st.SetSystemInfo("cpu_cores", fmt.Sprintf("%d", info.CPUCores))
	st.SetSystemInfo("go_version", info.GoVersion)
	st.SetSystemInfo("agent_version", info.AgentVersion)
}
