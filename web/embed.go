// Package web embeds the single-page browser UI so it ships inside the
// binary and is served at runtime without a filesystem dependency (the
// distroless image copies only the binary), and with no frontend build
// step — the page loads Vue from a CDN.
package web

import _ "embed"

//go:embed index.html
var IndexHTML []byte
