package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/scrapers"
)

// DiscoveryService orchestrates job discovery across all enabled scrapers.
type DiscoveryService struct {
	repo     *repository.JobRepository
	scrapers []scrapers.Scraper
	log      zerolog.Logger
}

// NewDiscoveryService creates a new discovery service with all registered scrapers.
func NewDiscoveryService(repo *repository.JobRepository, log zerolog.Logger) *DiscoveryService {
	return &DiscoveryService{
		repo: repo,
		scrapers: []scrapers.Scraper{
			scrapers.NewIndeedScraper(),
			scrapers.NewGlassdoorScraper(),
			scrapers.NewZipRecruiterScraper(),
		},
		log: log.With().Str("service", "discovery").Logger(),
	}
}

// RunResult holds the outcome of a discovery run.
type RunResult struct {
	Source    models.JobSource
	JobsFound int
	JobsNew   int
	JobsDupe  int
	Duration  time.Duration
	Err       error
}

// DiscoverParams defines what to search for across all boards.
type DiscoverParams struct {
	Keywords  []string
	Location  string
	Remote    bool
	MaxPages  int
	SalaryMin int
}

// RunAll triggers discovery across all scrapers concurrently.
// Returns one RunResult per scraper.
func (s *DiscoveryService) RunAll(ctx context.Context, params DiscoverParams) []RunResult {
	var wg sync.WaitGroup
	results := make([]RunResult, len(s.scrapers))

	for i, scraper := range s.scrapers {
		wg.Add(1)
		go func(idx int, sc scrapers.Scraper) {
			defer wg.Done()
			results[idx] = s.runScraper(ctx, sc, params)
		}(i, scraper)
	}

	wg.Wait()
	return results
}

// RunSingle runs a single scraper by source name.
func (s *DiscoveryService) RunSingle(ctx context.Context, source models.JobSource, params DiscoverParams) RunResult {
	for _, sc := range s.scrapers {
		if sc.Source() == source {
			return s.runScraper(ctx, sc, params)
		}
	}
	return RunResult{
		Source: source,
		Err:    fmt.Errorf("no scraper registered for source %q", source),
	}
}

// runScraper executes a single scraper and persists the results.
func (s *DiscoveryService) runScraper(ctx context.Context, sc scrapers.Scraper, params DiscoverParams) RunResult {
	start := time.Now()
	result := RunResult{Source: sc.Source()}

	log := s.log.With().
		Str("scraper", sc.Name()).
		Str("source", string(sc.Source())).
		Logger()

	log.Info().
		Strs("keywords", params.Keywords).
		Str("location", params.Location).
		Msg("Starting scrape")

	searchParams := scrapers.SearchParams{
		Keywords:  params.Keywords,
		Location:  params.Location,
		Remote:    params.Remote,
		MaxPages:  params.MaxPages,
		SalaryMin: params.SalaryMin,
	}

	jobsCh, errCh := sc.Search(ctx, searchParams)

	// Batch jobs for bulk upsert
	const batchSize = 20
	batch := make([]*models.ScrapedJob, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		newCount, dupeCount, err := s.repo.BulkUpsert(ctx, batch)
		if err != nil {
			log.Error().Err(err).Int("batch_size", len(batch)).Msg("Bulk upsert failed")
		} else {
			result.JobsNew += newCount
			result.JobsDupe += dupeCount
			log.Debug().Int("new", newCount).Int("dupe", dupeCount).Msg("Batch upserted")
		}
		batch = batch[:0]
	}

	// Drain jobs channel
	for {
		select {
		case <-ctx.Done():
			flush()
			result.Err = ctx.Err()
			result.Duration = time.Since(start)
			return result

		case job, ok := <-jobsCh:
			if !ok {
				// Channel closed — flush remaining batch
				flush()
				result.JobsFound = result.JobsNew + result.JobsDupe
				result.Duration = time.Since(start)

				log.Info().
					Int("found", result.JobsFound).
					Int("new", result.JobsNew).
					Int("dupe", result.JobsDupe).
					Dur("duration", result.Duration).
					Msg("Scrape complete")
				return result
			}

			batch = append(batch, job)
			if len(batch) >= batchSize {
				flush()
			}

		case err, ok := <-errCh:
			if ok && err != nil {
				log.Warn().Err(err).Msg("Scraper error (non-fatal)")
				result.Err = err
			}
		}
	}
}

// SourceInfo describes a registered scraper for API responses.
type SourceInfo struct {
	Source models.JobSource
	Name   string
}

// Sources returns metadata about every registered scraper.
// Used by the HTTP handler to expose the /sources endpoint.
func (s *DiscoveryService) Sources() []SourceInfo {
	infos := make([]SourceInfo, 0, len(s.scrapers))
	for _, sc := range s.scrapers {
		infos = append(infos, SourceInfo{
			Source: sc.Source(),
			Name:   sc.Name(),
		})
	}
	return infos
}

// GetStats returns discovery statistics from the database.
// Response shape:
//
//	{
//	  "scrapers_registered": 2,
//	  "total_jobs":          1234,
//	  "active_jobs":         987,
//	  "by_source": {
//	    "indeed":    { "total": 700, "active": 600 },
//	    "glassdoor": { "total": 534, "active": 387 }
//	  }
//	}
func (s *DiscoveryService) GetStats(ctx context.Context) (map[string]any, error) {
	sourceCounts, err := s.repo.CountBySource(ctx)
	if err != nil {
		return nil, fmt.Errorf("count by source: %w", err)
	}

	bySource := make(map[string]any, len(sourceCounts))
	var totalJobs, activeJobs int
	for src, stats := range sourceCounts {
		bySource[src] = map[string]int{
			"total":  stats.Total,
			"active": stats.Active,
		}
		totalJobs += stats.Total
		activeJobs += stats.Active
	}

	return map[string]any{
		"scrapers_registered": len(s.scrapers),
		"total_jobs":          totalJobs,
		"active_jobs":         activeJobs,
		"by_source":           bySource,
	}, nil
}
