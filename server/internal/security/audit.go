package security

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditMiddleware logs sensitive operations (POST, PUT, DELETE).
func AuditMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		// Only audit mutating operations
		if method != "POST" && method != "PUT" && method != "DELETE" {
			c.Next()
			return
		}

		c.Next()

		logger.Info("audit",
			"action", method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ip", c.ClientIP(),
			"time", time.Now().Format(time.RFC3339),
		)
	}
}

// IPWhitelistMiddleware restricts access to allowed IPs.
// If the whitelist is empty, all IPs are allowed.
func IPWhitelistMiddleware(allowed []string) gin.HandlerFunc {
	if len(allowed) == 0 {
		return func(c *gin.Context) { c.Next() }
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, ip := range allowed {
		allowedSet[ip] = true
	}

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if !allowedSet[clientIP] {
			errorResponse(c, 403, "access denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":    code,
		"message": message,
		"data":    nil,
	})
}
