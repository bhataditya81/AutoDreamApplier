package salary

// DB-dependent tests for Repository.GetBenchmark.
// All tests in this file are skipped unless TEST_DATABASE_URL is set.
//
// To run locally:
//   TEST_DATABASE_URL="postgres://autodream:autodream_dev@localhost:5432/autodreamapplier?sslmode=disable" \
//   go test ./internal/salary/... -run TestRepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// openTestPool opens a pgxpool using TEST_DATABASE_URL.
// It calls t.Skip if the env var is not set, and t.Fatal on connection error.
func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	if pingErr := pool.Ping(ctx); pingErr != nil {
		pool.Close()
		t.Fatalf("pool.Ping: %v", pingErr)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestRepository_GetBenchmark_NoRow verifies that GetBenchmark returns nil, nil
// (no error) when no matching row exists in salary_benchmarks.
func TestRepository_GetBenchmark_NoRow(t *testing.T) {
	pool := openTestPool(t)
	repo := NewRepository(pool, zerolog.Nop())

	// Use a title/location key that is extremely unlikely to exist.
	b, err := repo.GetBenchmark(context.Background(),
		"xyzzy-nonexistent-title-9999",
		"xyzzy-nonexistent-location-9999",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != nil {
		t.Errorf("expected nil benchmark for missing row, got %+v", b)
	}
}

// TestRepository_GetBenchmark_SmallSampleSize verifies that a row with
// sample_size < minSamples (5) is treated as if it does not exist and
// GetBenchmark returns nil, nil.
//
// This test inserts a row with sample_size = 2, asserts nil is returned,
// then cleans up.
func TestRepository_GetBenchmark_SmallSampleSize(t *testing.T) {
	pool := openTestPool(t)

	const (
		testTitleKey    = "test-small-sample-engineer"
		testLocationKey = "test-small-location"
	)

	ctx := context.Background()

	// Insert a row with sample_size = 2 (below minSamples = 5).
	_, err := pool.Exec(ctx, `
		INSERT INTO salary_benchmarks
		    (title_key, location_key, currency,
		     min_salary, p25_salary, median_salary, p75_salary, max_salary,
		     sample_size, updated_at)
		VALUES ($1, $2, 'USD', 50000, 60000, 70000, 80000, 90000, 2, $3)
		ON CONFLICT (title_key, location_key, currency) DO UPDATE
		    SET sample_size = EXCLUDED.sample_size,
		        updated_at  = EXCLUDED.updated_at`,
		testTitleKey, testLocationKey, time.Now(),
	)
	if err != nil {
		t.Fatalf("insert test row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM salary_benchmarks WHERE title_key = $1 AND location_key = $2`,
			testTitleKey, testLocationKey,
		)
	})

	repo := NewRepository(pool, zerolog.Nop())
	b, err := repo.GetBenchmark(ctx, testTitleKey, testLocationKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != nil {
		t.Errorf("expected nil benchmark for sample_size < %d, got %+v", minSamples, b)
	}
}

// TestRepository_GetBenchmark_AdequateSampleSize verifies that a row with
// sample_size >= minSamples (5) is returned correctly.
func TestRepository_GetBenchmark_AdequateSampleSize(t *testing.T) {
	pool := openTestPool(t)

	const (
		testTitleKey    = "test-adequate-sample-engineer"
		testLocationKey = "test-adequate-location"
	)

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO salary_benchmarks
		    (title_key, location_key, currency,
		     min_salary, p25_salary, median_salary, p75_salary, max_salary,
		     sample_size, updated_at)
		VALUES ($1, $2, 'USD', 80000, 100000, 120000, 140000, 160000, 10, $3)
		ON CONFLICT (title_key, location_key, currency) DO UPDATE
		    SET sample_size  = EXCLUDED.sample_size,
		        median_salary = EXCLUDED.median_salary,
		        updated_at   = EXCLUDED.updated_at`,
		testTitleKey, testLocationKey, time.Now(),
	)
	if err != nil {
		t.Fatalf("insert test row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM salary_benchmarks WHERE title_key = $1 AND location_key = $2`,
			testTitleKey, testLocationKey,
		)
	})

	repo := NewRepository(pool, zerolog.Nop())
	b, err := repo.GetBenchmark(ctx, testTitleKey, testLocationKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil benchmark for adequate sample size")
	}
	if b.Median != 120000 {
		t.Errorf("Median = %d; want 120000", b.Median)
	}
	if b.SampleSize != 10 {
		t.Errorf("SampleSize = %d; want 10", b.SampleSize)
	}
	if b.TitleKey != testTitleKey {
		t.Errorf("TitleKey = %q; want %q", b.TitleKey, testTitleKey)
	}
}
