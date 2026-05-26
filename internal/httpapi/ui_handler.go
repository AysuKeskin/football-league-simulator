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

// vueJS serves the vendored Vue runtime used by the embedded UI.
func vueJS(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", web.VueJS)
}
