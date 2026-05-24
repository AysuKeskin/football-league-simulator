// Package dbtest spins up a throwaway Postgres for repository tests.
//
// New(t) starts a Postgres container on first call within the test
// binary, applies the canonical schema, and returns a clean pool ready
// for use. Subsequent calls within the same binary reuse the container
// and just truncate data tables for isolation.
//
// If Docker is not available, New skips the test instead of failing,
// so unit-only test runs on a Docker-less machine still pass.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	dbUser     = "fls"
	dbPassword = "fls"
	dbName     = "fls"

	// readyTimeout bounds how long we wait for the container to accept
	// connections before declaring the test infrastructure broken.
	readyTimeout = 60 * time.Second
)

var (
	initOnce sync.Once
	sharedPool *pgxpool.Pool
	initErr  error
)

// New returns a pool against a clean schema. The pool is shared across
// the test binary; each call truncates data tables so tests do not see
// each other's writes.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	initOnce.Do(setup)
	if initErr != nil {
		t.Skipf("dbtest: skipping (no docker?): %v", initErr)
	}
	truncate(t, sharedPool)
	return sharedPool
}

func setup() {
	pool, err := dockertest.NewPool("")
	if err != nil {
		initErr = fmt.Errorf("connect to docker: %w", err)
		return
	}
	if err := pool.Client.Ping(); err != nil {
		initErr = fmt.Errorf("docker ping: %w", err)
		return
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_USER=" + dbUser,
			"POSTGRES_PASSWORD=" + dbPassword,
			"POSTGRES_DB=" + dbName,
		},
	}, func(c *docker.HostConfig) {
		c.AutoRemove = true
		c.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		initErr = fmt.Errorf("run postgres container: %w", err)
		return
	}
	// Container is killed when the test binary exits; AutoRemove
	// cleans up the image layer too.

	dsn := fmt.Sprintf(
		"postgres://%s:%s@localhost:%s/%s?sslmode=disable",
		dbUser, dbPassword, resource.GetPort("5432/tcp"), dbName,
	)

	ctx, cancel := context.WithTimeout(context.Background(), readyTimeout)
	defer cancel()

	// Postgres may take a few seconds to be ready; retry until success.
	if err := pool.Retry(func() error {
		p, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return err
		}
		if err := p.Ping(ctx); err != nil {
			p.Close()
			return err
		}
		sharedPool = p
		return nil
	}); err != nil {
		initErr = fmt.Errorf("postgres never became ready: %w", err)
		return
	}

	if err := applySchema(ctx, sharedPool); err != nil {
		initErr = fmt.Errorf("apply schema: %w", err)
	}
}

// applySchema runs database/schema.sql against the pool. We use the
// canonical schema file rather than the migrations because tests don't
// care about migration history — they just need the final shape.
func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
	schemaPath, err := findRepoFile("database/schema.sql")
	if err != nil {
		return err
	}
	sql, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema.sql: %w", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	return nil
}

// findRepoFile walks up from the calling source file to locate a file
// in the repo root. Lets tests reference database/schema.sql without
// hardcoding a relative path that breaks if the package moves.
func findRepoFile(relative string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("could not locate %s walking up from dbtest", relative)
}

// truncate empties data tables in dependency-safe order. RESTART
// IDENTITY resets sequences so primary keys start at 1 in each test.
func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const stmt = `
		TRUNCATE TABLE
			match_audit_logs,
			standings_snapshot_rows,
			standings_snapshots,
			external_team_profiles,
			matches,
			league_teams,
			leagues,
			teams
		RESTART IDENTITY CASCADE
	`
	if _, err := pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("dbtest: truncate: %v", err)
	}
}
