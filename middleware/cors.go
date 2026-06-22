package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	corsDefaultAllowHeaders = "Origin,Content-Length,Content-Type,Accept,Authorization,Cache-Control,X-Requested-With"
	corsDefaultAllowMethods = "GET,POST,PUT,PATCH,DELETE,OPTIONS,HEAD"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" || isSameOrigin(c, origin) {
			c.Next()
			return
		}

		if !isAllowedCORSOrigin(origin) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", origin)
		header.Set("Access-Control-Allow-Credentials", "true")
		header.Add("Vary", "Origin")

		if c.Request.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", corsDefaultAllowMethods)
			header.Set("Access-Control-Allow-Headers", requestedCORSHeaders(c))
			header.Add("Vary", "Access-Control-Request-Method")
			header.Add("Vary", "Access-Control-Request-Headers")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedCORSOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func isSameOrigin(c *gin.Context, origin string) bool {
	host := c.Request.Host
	return origin == "http://"+host || origin == "https://"+host
}

func requestedCORSHeaders(c *gin.Context) string {
	headers := strings.TrimSpace(c.Request.Header.Get("Access-Control-Request-Headers"))
	if headers == "" {
		return corsDefaultAllowHeaders
	}
	return headers
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
