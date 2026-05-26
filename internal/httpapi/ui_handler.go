package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/web"
)

// ui serves the embedded single-page browser UI at the site root.
func ui(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", web.IndexHTML)
}
