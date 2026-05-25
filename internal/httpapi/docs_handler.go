package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/AysuKeskin/football-league-simulator/api"
)

// swaggerHTML renders Swagger UI from a CDN and points it at the spec
// served by this server. Kept inline (no vendored assets); the deployed
// demo has internet access.
const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Football League Simulator — API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`

// openapiSpec serves the embedded OpenAPI document.
func openapiSpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", api.OpenAPISpec)
}

// swaggerUI serves the Swagger UI page that consumes /openapi.yaml.
func swaggerUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}
