// Package httpapi wires the HTTP transport layer: the Gin engine,
// route registration, and the request/response DTOs that cross the wire.
//
// Handlers in subpackages stay thin; the router is the only place that
// knows how routes map to handler functions.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// readyPingTimeout bounds the DB ping issued from the /ready handler.
// Kept short so the probe fails fast when the DB is unreachable.
const readyPingTimeout = 2 * time.Second

// Pinger is the minimal contract /ready needs from the database layer.
// Declaring it here (instead of importing pgxpool) keeps the httpapi
// package free of database dependencies and trivially testable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the application's HTTP router with every route registered.
//
// The pinger is consulted by /ready; pass nil only in tests that do not
// exercise /ready.
func NewRouter(pinger Pinger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", health)
	r.GET("/ready", readyHandler(pinger))

	return r
}

// health is a liveness probe. It reports only that the process is up;
// dependency checks live on /ready.
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyHandler returns a handler that pings the database and responds
// 200 when reachable, 503 otherwise. The underlying error is logged at
// Warn but never returned in the response body — health probes are
// unauthenticated and the raw pgx error contains DSN-ish internals.
func readyHandler(pinger Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyPingTimeout)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			log.Warn().Err(err).Msg("readiness check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
