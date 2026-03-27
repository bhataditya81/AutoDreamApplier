package salary

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

const minSamples = 5

// Repository handles salary benchmark persistence.
type Repository struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool, log zerolog.Logger) *Repository {
	return &Repository{pool: pool, log: log}
}

// GetBenchmark returns a pre-computed benchmark row.
// Returns nil, nil (not found) when no row exists or sample_size < minSamples.
func (r *Repository) GetBenchmark(ctx context.Context, titleKey, locationKey string) (*Benchmark, error) {
	const q = `
		SELECT title_key, location_key, currency,
		       min_salary, p25_salary, median_salary, p75_salary, max_salary,
		       sample_size, updated_at
		FROM salary_benchmarks
		WHERE title_key = $1 AND location_key = $2
		LIMIT 1`

	var b Benchmark
	err := r.pool.QueryRow(ctx, q, titleKey, locationKey).Scan(
		&b.TitleKey, &b.LocationKey, &b.Currency,
		&b.Min, &b.P25, &b.Median, &b.P75, &b.Max,
		&b.SampleSize, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.log.Error().Err(err).
			Str("title_key", titleKey).
			Str("location_key", locationKey).
			Msg("GetBenchmark: query failed")
		return nil, err
	}

	if b.SampleSize < minSamples {
		return nil, nil
	}
	return &b, nil
}

// RebuildBenchmarks re-aggregates salary data from the jobs table into
// salary_benchmarks. Designed to be called nightly.
// Only inserts rows where COUNT(*) >= minSamples (5).
// Returns count of rows upserted.
func (r *Repository) RebuildBenchmarks(ctx context.Context) (int, error) {
	// Step 1: find all distinct (title_key, location_key) pairs with >= 5 samples.
	const selectPairs = `
		SELECT
		    lower(regexp_replace(title, '(?i)^(senior|sr\.|jr\.|junior|staff|principal|lead|associate)\s+', '')) AS title_key,
		    lower(regexp_replace(location, '[^a-z0-9]+', '-', 'g')) AS location_key
		FROM jobs
		WHERE salary_min IS NOT NULL
		  AND salary_max IS NOT NULL
		  AND salary_min > 0
		  AND salary_max >= salary_min
		  AND is_active = true
		  AND is_scam   = false
		GROUP BY title_key, location_key
		HAVING COUNT(*) >= 5`

	rows, err := r.pool.Query(ctx, selectPairs)
	if err != nil {
		r.log.Error().Err(err).Msg("RebuildBenchmarks: failed to select pairs")
		return 0, err
	}
	defer rows.Close()

	type pair struct{ titleKey, locationKey string }
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.titleKey, &p.locationKey); err != nil {
			return 0, err
		}
		pairs = append(pairs, p)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Step 2: upsert each pair.
	const upsert = `
		INSERT INTO salary_benchmarks
		    (title_key, location_key, currency,
		     min_salary, p25_salary, median_salary, p75_salary, max_salary,
		     sample_size, updated_at)
		SELECT
		    $1,
		    $2,
		    COALESCE(salary_currency, 'USD'),
		    MIN(mid_salary)::INT,
		    PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY mid_salary)::INT,
		    PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY mid_salary)::INT,
		    PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY mid_salary)::INT,
		    MAX(mid_salary)::INT,
		    COUNT(*)::INT,
		    NOW()
		FROM (
		    SELECT
		        (salary_min + salary_max) / 2.0 AS mid_salary,
		        salary_currency,
		        lower(regexp_replace(title, '(?i)^(senior|sr\.|jr\.|junior|staff|principal|lead|associate)\s+', '')) AS title_key,
		        lower(regexp_replace(location, '[^a-z0-9]+', '-', 'g')) AS location_key
		    FROM jobs
		    WHERE salary_min IS NOT NULL
		      AND salary_max IS NOT NULL
		      AND salary_min > 0
		      AND salary_max >= salary_min
		      AND is_active = true
		      AND is_scam   = false
		) sub
		WHERE title_key = $1 AND location_key = $2
		HAVING COUNT(*) >= 5
		ON CONFLICT (title_key, location_key, currency) DO UPDATE SET
		    min_salary    = EXCLUDED.min_salary,
		    p25_salary    = EXCLUDED.p25_salary,
		    median_salary = EXCLUDED.median_salary,
		    p75_salary    = EXCLUDED.p75_salary,
		    max_salary    = EXCLUDED.max_salary,
		    sample_size   = EXCLUDED.sample_size,
		    updated_at    = NOW()`

	count := 0
	for _, p := range pairs {
		tag, err := r.pool.Exec(ctx, upsert, p.titleKey, p.locationKey)
		if err != nil {
			r.log.Error().Err(err).
				Str("title_key", p.titleKey).
				Str("location_key", p.locationKey).
				Msg("RebuildBenchmarks: upsert failed")
			return count, err
		}
		count += int(tag.RowsAffected())
	}

	r.log.Info().
		Int("upserted", count).
		Time("at", time.Now()).
		Msg("RebuildBenchmarks: complete")
	return count, nil
}
