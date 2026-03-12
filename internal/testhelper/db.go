// Package testhelper provides shared utilities for integration tests.
// Tests that require a live PostgreSQL database call SkipIfNoTestDB (or
// NewTestPool) so they are skipped gracefully when no test DB is configured.
//
// CI sets TEST_DATABASE_URL so all integration tests run automatically.
// Local development without a database gets a clean skip instead of a failure.
package testhelper

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// SkipIfNoTestDB skips t immediately if TEST_DATABASE_URL is not set.
// Call this at the top of every integration test or in TestMain.
func SkipIfNoTestDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
}

// NewTestPool creates a *pgxpool.Pool connected to the test database, ensures
// the full schema is present, and registers t.Cleanup(pool.Close).
//
// It calls SkipIfNoTestDB internally, so callers need not call it separately.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	SkipIfNoTestDB(t)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, TestDSN())
	if err != nil {
		t.Fatalf("testhelper.NewTestPool: connect: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("testhelper.NewTestPool: ping: %v", err)
	}

	if err := ApplyTestSchema(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("testhelper.NewTestPool: apply schema: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// NopLogger returns a zerolog.Logger that discards all output.
func NopLogger() zerolog.Logger {
	return zerolog.Nop()
}

// TestDSN returns the DSN for the test database.
// TEST_DATABASE_URL takes precedence; otherwise it is built from DB_* env vars.
func TestDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		EnvOrDefault("DB_USER", "postgres"),
		EnvOrDefault("DB_PASSWORD", "postgres"),
		EnvOrDefault("DB_HOST", "localhost"),
		EnvOrDefault("DB_PORT", "5432"),
		EnvOrDefault("DB_NAME", "autodreamapplier_test"),
		EnvOrDefault("DB_SSLMODE", "disable"),
	)
}

// EnvOrDefault returns the value of the environment variable key, or def if
// the variable is unset or empty.
func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
