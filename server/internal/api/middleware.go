package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Bearer token.
func AuthMiddleware(token string) gin.HandlerFunc {
	tokenBytes := []byte(token)

	return func(c *gin.Context) {
		if len(tokenBytes) == 0 {
			errorResponse(c, http.StatusServiceUnavailable, "authentication not configured")
			c.Abort()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			errorResponse(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			errorResponse(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(parts[1]), tokenBytes) != 1 {
			errorResponse(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware limits requests per IP using a token bucket.
func RateLimitMiddleware(rps int, burst int) gin.HandlerFunc {
	type bucket struct {
		tokens    float64
		lastCheck time.Time
	}

	var mu sync.Mutex
	clients := make(map[string]*bucket)
	rate := float64(rps)
	maxTokens := float64(burst)

	// Periodic cleanup of stale entries
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, b := range clients {
				if now.Sub(b.lastCheck) > 5*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		b, exists := clients[ip]
		now := time.Now()
		if !exists {
			b = &bucket{tokens: maxTokens, lastCheck: now}
			clients[ip] = b
		}

		elapsed := now.Sub(b.lastCheck).Seconds()
		b.tokens += elapsed * rate
		if b.tokens > maxTokens {
			b.tokens = maxTokens
		}
		b.lastCheck = now

		if b.tokens < 1 {
			mu.Unlock()
			c.Header("Retry-After", "1")
			errorResponse(c, http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}
		b.tokens--
		mu.Unlock()

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
