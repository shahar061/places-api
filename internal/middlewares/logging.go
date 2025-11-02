package middlewares

import (
	"places_api/internal/logger"
	"time"

	"github.com/gin-gonic/gin"
)

// Logging logs HTTP requests
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		userAgent := c.Request.UserAgent()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Global.LogHTTPRequest(method, path, userAgent, clientIP, statusCode, latency)
	}
}
