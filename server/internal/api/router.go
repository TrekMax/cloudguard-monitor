package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trekmax/cloudguard-monitor/internal/collector"
	"github.com/trekmax/cloudguard-monitor/internal/store"
	"github.com/trekmax/cloudguard-monitor/internal/ws"
)

// Server holds the dependencies for the API server.
type Server struct {
	logger    *slog.Logger
	store     *store.Store
	scheduler *collector.Scheduler
	sysInfo   *collector.SystemInfo
	token     string
	hub       *ws.Hub
}

// NewServer creates a new API server.
func NewServer(logger *slog.Logger, st *store.Store, sched *collector.Scheduler, sysInfo *collector.SystemInfo, token string, hub *ws.Hub) *Server {
	return &Server{
		logger:    logger,
		store:     st,
		scheduler: sched,
		sysInfo:   sysInfo,
		token:     token,
		hub:       hub,
	}
}

// Hub returns the WebSocket hub.
func (s *Server) Hub() *ws.Hub {
	return s.hub
}

// SetupRouter creates and configures the Gin router.
func (s *Server) SetupRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	r.Use(RateLimitMiddleware(30, 60)) // 30 req/s per IP, burst 60
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

		// Alert rules
		v1.GET("/alerts/rules", s.handleListAlertRules)
		v1.POST("/alerts/rules", s.handleCreateAlertRule)
		v1.PUT("/alerts/rules/:id", s.handleUpdateAlertRule)
		v1.DELETE("/alerts/rules/:id", s.handleDeleteAlertRule)

		// Alert events
		v1.GET("/alerts", s.handleListAlerts)
		v1.POST("/alerts/:id/ack", s.handleAckAlert)
	}

	// WebSocket (auth via query param)
	r.GET("/ws/v1/realtime", s.handleWebSocket)

	return r
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func (s *Server) handleWebSocket(c *gin.Context) {
	// Authenticate via query parameter
	if s.token == "" {
		errorResponse(c, http.StatusServiceUnavailable, "authentication not configured")
		return
	}
	token := c.Query("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		errorResponse(c, http.StatusUnauthorized, "invalid token")
		return
	}
	s.hub.HandleConnect(c.Writer, c.Request)
}
