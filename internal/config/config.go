// Package config loads runtime configuration from environment variables.
//
// All fields are populated from the process environment via envconfig.
// Load is the single entry point; callers receive a validated Config or
// an error describing which variable was rejected.
package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/kelseyhightower/envconfig"
)

// Config holds every runtime setting the server needs.
//
// Fields are intentionally narrow: a new field arrives only when a feature
// requires it. Each tagged field is validated at load time so the server
// fails fast on misconfiguration instead of crashing mid-request.
type Config struct {
	// Port is the TCP port the HTTP server binds to.
	Port int `envconfig:"PORT" default:"8080" validate:"min=1,max=65535"`

	// LogLevel controls zerolog verbosity. Accepted: debug, info, warn, error.
	LogLevel string `envconfig:"LOG_LEVEL" default:"info" validate:"oneof=debug info warn error"`

	// DatabaseURL is the Postgres connection string consumed by pgx.
	// Required; the server cannot serve /ready or any persistent endpoint
	// without it. Both URL form (postgres://user:pass@host:port/db?...)
	// and pgx keyword form (host=... user=... password=...) are accepted;
	// pgxpool.ParseConfig validates the syntax when the pool is opened.
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true" validate:"required"`
}

// Load reads the environment, applies defaults, and validates the result.
// The returned error wraps both envconfig parse failures and validator
// constraint violations so the caller has a single error to surface.
func Load() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse env: %w", err)
	}
	if err := validator.New().Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("config: validate: %w", err)
	}
	return cfg, nil
}
