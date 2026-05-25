// Command server is the composition root for the football-league-simulator
// HTTP service. It loads configuration, configures logging, opens the
// Postgres pool, builds the router, and runs the server with graceful
// shutdown on SIGINT/SIGTERM.
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
	"github.com/AysuKeskin/football-league-simulator/internal/fixture"
	"github.com/AysuKeskin/football-league-simulator/internal/httpapi"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
	"github.com/AysuKeskin/football-league-simulator/internal/simulation"
	"github.com/AysuKeskin/football-league-simulator/internal/standings"
)

const (
	// shutdownTimeout bounds how long in-flight requests have to drain
	// after a termination signal before the process exits.
	shutdownTimeout = 10 * time.Second

	// dbStartupTimeout bounds the time to open and ping the pool at boot.
	// On failure the process exits before binding the listener.
	dbStartupTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
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

	// Open the DB pool before binding the listener so a misconfigured
	// DATABASE_URL fails the process startup rather than the first request.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), dbStartupTimeout)
	defer dbCancel()
	pool, err := postgres.NewPool(dbCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db pool: %w", err)
	}
	defer pool.Close()
	log.Info().Msg("postgres pool opened")

	// Compose the application: repositories + algorithms → services → router.
	repos := postgres.NewRepositories(pool)
	transactor := postgres.NewTransactor(pool)
	leagueService := service.NewLeagueService(
		repos, transactor, fixture.New(), simulation.New(), standings.New(),
	)
	matchService := service.NewMatchService(repos, transactor, standings.New())
	teamService := service.NewTeamService(repos)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewRouter(pool, leagueService, matchService, teamService),
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

	// Drain any error the listener goroutine may have raised concurrent
	// with the shutdown signal. Without this, a fatal ListenAndServe error
	// that arrived right as a signal landed would be silently dropped.
	if err, ok := <-serverErr; ok && err != nil {
		return fmt.Errorf("http server: %w", err)
	}

	log.Info().Msg("http server stopped cleanly")
	return nil
}

// configureLogger wires zerolog's global logger to the configured level.
// Falls back to info on an unrecognized level so logging never breaks
// startup even if validation is loosened in the future.
func configureLogger(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}
