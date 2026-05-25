// Package database embeds the SQL migrations so they ship inside the
// binary and can be applied on startup without the migration files (or
// the migrate CLI) being present on the host — which matters for the
// distroless image, where only the binary is copied.
package database

import "embed"

//go:embed migrations/*.sql
var MigrationsFS embed.FS
