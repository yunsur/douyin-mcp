package main

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// authMiddleware enforces a static Bearer token; empty token disables auth.
func authMiddleware(token string) gin.HandlerFunc {
	expected := []byte(token)
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		scheme, credentials, found := strings.Cut(c.GetHeader("Authorization"), " ")
		credentials = strings.TrimLeft(credentials, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") ||
			subtle.ConstantTimeCompare([]byte(credentials), expected) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "未授权", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// corsMiddleware enables permissive CORS for local integrations.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// errorHandlingMiddleware converts panics into 500 responses.
func errorHandlingMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logrus.Errorf("服务器内部错误: %v, path: %s", recovered, c.Request.URL.Path)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误", recovered)
	})
}
