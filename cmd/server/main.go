// Command server is the composition root for the football-league-simulator
// HTTP service. It loads configuration, configures logging, builds the
// router, and runs the server with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/AysuKeskin/football-league-simulator/internal/config"
	"github.com/AysuKeskin/football-league-simulator/internal/httpapi"
)

// shutdownTimeout bounds how long in-flight requests have to drain
// after a termination signal before the process exits.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		// Use Fatal here so the exit code is non-zero; the error is
		// already logged with context by the caller chain.
		log.Fatal().Err(err).Msg("server exited with error")
	}
}

// run is the real entry point. Returning an error from run instead of
// calling log.Fatal directly keeps main testable and lets defers fire.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	configureLogger(cfg.LogLevel)
	gin.SetMode(gin.ReleaseMode)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run ListenAndServe in a goroutine so we can wait on signals in main.
	serverErr := make(chan error, 1)
	go func() {
		log.Info().Int("port", cfg.Port).Msg("http server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Block until a termination signal or a fatal server error arrives.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err, ok := <-serverErr:
		if ok && err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info().Msg("http server stopped cleanly")
	return nil
}

// configureLogger wires zerolog's global logger to the configured level.
// Falls back to info on an unrecognized level rather than failing —
// validation in config.Load already rejects unknown values, so this is
// belt-and-braces for future callers.
func configureLogger(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}
