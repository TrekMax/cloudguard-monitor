package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

// Server holds the dependencies for the API server.
type Server struct {
	logger    *slog.Logger
	store     *store.Store
	scheduler *collector.Scheduler
	sysInfo   *collector.SystemInfo
	token     string
}

// NewServer creates a new API server.
func NewServer(logger *slog.Logger, st *store.Store, sched *collector.Scheduler, sysInfo *collector.SystemInfo, token string) *Server {
	return &Server{
		logger:    logger,
		store:     st,
		scheduler: sched,
		sysInfo:   sysInfo,
		token:     token,
	}
}

// SetupRouter creates and configures the Gin router.
func (s *Server) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(LoggingMiddleware(s.logger))

	// Health check (no auth)
	r.GET("/health", s.handleHealth)

	// API v1 group with auth
	v1 := r.Group("/api/v1")
	v1.Use(AuthMiddleware(s.token))
	{
		v1.GET("/status", s.handleStatus)
		v1.GET("/metrics", s.handleMetrics)
		v1.GET("/system", s.handleSystem)
		v1.GET("/processes", s.handleProcesses)
	}

	return r
}
