package salary

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const cacheTTL = 24 * time.Hour

// Benchmark is the salary range for a given title+location.
type Benchmark struct {
	TitleKey    string    `json:"title_key"`
	LocationKey string    `json:"location_key"`
	Currency    string    `json:"currency"`
	Min         int       `json:"min"`
	P25         int       `json:"p25"`
	Median      int       `json:"median"`
	P75         int       `json:"p75"`
	Max         int       `json:"max"`
	SampleSize  int       `json:"sample_size"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MarketPosition describes how a specific salary compares to the benchmark.
type MarketPosition string

const (
	PositionAbove   MarketPosition = "above"
	PositionAt      MarketPosition = "at"
	PositionBelow   MarketPosition = "below"
	PositionUnknown MarketPosition = "unknown"
)

// BenchmarkResponse is the full API response.
type BenchmarkResponse struct {
	Benchmark      *Benchmark     `json:"benchmark"`
	JobSalaryMin   *int           `json:"job_salary_min,omitempty"`
	JobSalaryMax   *int           `json:"job_salary_max,omitempty"`
	MarketPosition MarketPosition `json:"market_position"`
}

// BenchmarkGetter is the interface used by Service to fetch benchmarks.
// It is satisfied by *Repository and allows mock injection in tests.
type BenchmarkGetter interface {
	GetBenchmark(ctx context.Context, titleKey, locationKey string) (*Benchmark, error)
}

// Service handles business logic for salary benchmarks.
type Service struct {
	repo  BenchmarkGetter
	redis *redis.Client // may be nil
	log   zerolog.Logger
}

// NewService creates a new Service.
func NewService(repo *Repository, redisClient *redis.Client, log zerolog.Logger) *Service {
	return &Service{
		repo:  repo,
		redis: redisClient,
		log:   log,
	}
}

// cacheKey builds the Redis key for a benchmark.
func cacheKey(titleKey, locationKey string) string {
	return fmt.Sprintf("salary:benchmark:%s:%s", titleKey, locationKey)
}

// GetBenchmark returns the benchmark for a title+location, with 24h Redis caching.
// If redis is nil or cache miss, falls back to DB.
// Returns nil benchmark (not an error) when sample_size < 5 or no data.
func (s *Service) GetBenchmark(ctx context.Context, title, location string) (*BenchmarkResponse, error) {
	titleKey := NormalizeTitle(title)
	locationKey := NormalizeLocation(location)

	// Try Redis cache first.
	if s.redis != nil {
		key := cacheKey(titleKey, locationKey)
		val, err := s.redis.Get(ctx, key).Result()
		if err == nil {
			var b Benchmark
			if jsonErr := json.Unmarshal([]byte(val), &b); jsonErr == nil {
				resp := &BenchmarkResponse{
					Benchmark:      &b,
					MarketPosition: PositionUnknown,
				}
				return resp, nil
			}
		}
		// err == redis.Nil is a normal cache miss; other errors are transient — fall through.
	}

	// DB lookup.
	b, err := s.repo.GetBenchmark(ctx, titleKey, locationKey)
	if err != nil {
		return nil, err
	}

	// Cache the result in Redis (even nil → skip caching).
	if b != nil && s.redis != nil {
		if data, jsonErr := json.Marshal(b); jsonErr == nil {
			key := cacheKey(titleKey, locationKey)
			if setErr := s.redis.Set(ctx, key, data, cacheTTL).Err(); setErr != nil {
				s.log.Warn().Err(setErr).Msg("GetBenchmark: failed to cache result")
			}
		}
	}

	resp := &BenchmarkResponse{
		Benchmark:      b,
		MarketPosition: PositionUnknown,
	}
	return resp, nil
}

// CompareToMarket returns above/at/below based on job mid salary vs benchmark median.
// ±10% of median is considered "at market".
func CompareToMarket(jobMin, jobMax *int, benchmark *Benchmark) MarketPosition {
	if jobMin == nil || jobMax == nil {
		return PositionUnknown
	}
	if benchmark == nil {
		return PositionUnknown
	}

	jobMid := (*jobMin + *jobMax) / 2
	median := benchmark.Median
	threshold := int(float64(median) * 0.10)

	switch {
	case jobMid > median+threshold:
		return PositionAbove
	case jobMid < median-threshold:
		return PositionBelow
	default:
		return PositionAt
	}
}
