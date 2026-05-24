// Package httpapi wires the HTTP transport layer: the Gin engine,
// route registration, and the request/response DTOs that cross the wire.
//
// Handlers in subpackages stay thin; the router is the only place that
// knows how routes map to handler functions.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter builds the application's HTTP router with every route registered.
//
// The router is returned ready-to-serve; the caller owns the lifecycle
// (binding, graceful shutdown). Keeping construction pure makes the
// router trivially testable with httptest.
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", health)

	return r
}

// health is a liveness probe. It reports only that the process is up;
// dependency checks (DB, external APIs) belong on /ready, added in step 2.
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
