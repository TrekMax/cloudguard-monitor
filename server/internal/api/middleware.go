package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Bearer token.
func AuthMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			errorResponse(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" || parts[1] != token {
			errorResponse(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Next()
	}
}

// LoggingMiddleware logs each request.
func LoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		logger.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"ip", c.ClientIP(),
		)
	}
}
