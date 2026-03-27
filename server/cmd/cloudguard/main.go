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

	alertpkg "github.com/trekmax/cloudguard-monitor/internal/alert"
	"github.com/trekmax/cloudguard-monitor/internal/api"
	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/config"
	"github.com/trekmax/cloudguard-monitor/internal/logging"
	"github.com/trekmax/cloudguard-monitor/internal/security"
	"github.com/trekmax/cloudguard-monitor/internal/store"
	"github.com/trekmax/cloudguard-monitor/internal/ws"
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

	// Auto-generate token if empty
	if cfg.Auth.Token == "" {
		token, err := security.GenerateToken(32)
		if err != nil {
			logger.Error("failed to generate token", "error", err)
			os.Exit(1)
		}
		cfg.Auth.Token = token
		logger.Info("auto-generated API token", "token", token)
	}

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

	// Initialize WebSocket hub
	hub := ws.NewHub(logger)

	// Initialize alert engine
	alertEngine := alertpkg.NewEngine(logger, st, sched)
	alertEngine.OnAlert(func(event *store.AlertEvent, rule *store.AlertRule) {
		// Broadcast alert via WebSocket
		hub.Broadcast(&ws.Message{
			Type:      "alert",
			Timestamp: time.Now().Unix(),
			Data: map[string]interface{}{
				"id":        event.ID,
				"rule_name": rule.Name,
				"status":    event.Status,
				"value":     event.Value,
				"threshold": rule.Threshold,
				"message":   event.Message,
			},
		})
	})
	go alertEngine.Run(ctx)
	logger.Info("alert engine started")

	// Start metrics persistence + WebSocket broadcast goroutine
	go persistAndBroadcast(ctx, logger, sched, st, hub)

	// Start cleanup goroutine
	go runCleanup(ctx, logger, st, cfg.Database.RetentionDays)

	// Setup and start API server
	srv := api.NewServer(logger, st, sched, sysInfo, cfg.Auth.Token, hub)
	router := srv.SetupRouter()

	// Add security middleware
	router.Use(security.AuditMiddleware(logger))
	if len(cfg.Security.IPWhitelist) > 0 {
		router.Use(security.IPWhitelistMiddleware(cfg.Security.IPWhitelist))
		logger.Info("IP whitelist enabled", "allowed", cfg.Security.IPWhitelist)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		if cfg.TLS.Enabled {
			certFile, keyFile, err := security.EnsureCert(security.TLSConfig{
				Enabled:  cfg.TLS.Enabled,
				CertFile: cfg.TLS.CertFile,
				KeyFile:  cfg.TLS.KeyFile,
				AutoCert: cfg.TLS.AutoCert,
			}, filepath.Dir(cfg.Database.Path))
			if err != nil {
				logger.Error("TLS setup failed", "error", err)
				cancel()
				return
			}
			logger.Info("API server starting (HTTPS)", "addr", addr)
			if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				logger.Error("API server error", "error", err)
				cancel()
			}
		} else {
			logger.Info("API server starting (HTTP)", "addr", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("API server error", "error", err)
				cancel()
			}
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

// persistAndBroadcast saves metrics to DB and pushes via WebSocket.
func persistAndBroadcast(ctx context.Context, logger *slog.Logger, sched *collector.Scheduler, st *store.Store, hub *ws.Hub) {
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

			// Build WS broadcast data
			wsData := make(map[string]interface{})

			for collectorName, metrics := range snap.Metrics {
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

					// For WS: use first (aggregate) metric per collector
					if _, exists := wsData[collectorName]; !exists && len(m.Labels) == 0 {
						wsData[collectorName] = m.Values
					}
				}
			}

			if len(records) > 0 {
				if err := st.InsertMetrics(records); err != nil {
					logger.Error("failed to persist metrics", "error", err)
				}
			}

			// Broadcast via WebSocket if clients connected
			if hub.ClientCount() > 0 {
				hub.Broadcast(&ws.Message{
					Type:      "metrics",
					Timestamp: ts,
					Data:      wsData,
				})
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
