package scrapers

import (
	"context"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// glassdoorScraper scrapes job listings from Glassdoor.
//
// Note: Glassdoor is more aggressively bot-protected than Indeed.
// MVP-B Strategy options:
//   1. ScraperAPI (recommended — handles JS rendering + CAPTCHAs)
//   2. Apify's Glassdoor actor
//   3. Glassdoor's unofficial API (may require auth)
type glassdoorScraper struct{}

// NewGlassdoorScraper creates a new Glassdoor scraper.
// Currently returns a stub — full implementation in MVP-B (Week 13+).
func NewGlassdoorScraper() Scraper {
	return &glassdoorScraper{}
}

func (s *glassdoorScraper) Source() models.JobSource { return models.SourceGlassdoor }
func (s *glassdoorScraper) Name() string              { return "Glassdoor Scraper (stub)" }

func (s *glassdoorScraper) Search(_ context.Context, _ SearchParams) (<-chan *models.ScrapedJob, <-chan error) {
	jobsCh := make(chan *models.ScrapedJob)
	errCh := make(chan error, 1)
	close(jobsCh)
	close(errCh)
	return jobsCh, errCh
}
