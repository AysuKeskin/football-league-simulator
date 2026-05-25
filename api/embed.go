// Package api exposes the OpenAPI specification as an embedded asset so
// it ships inside the binary and can be served at runtime without a
// filesystem dependency (the distroless image copies only the binary).
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
