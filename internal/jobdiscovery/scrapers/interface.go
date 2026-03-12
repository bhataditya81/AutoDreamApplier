package scrapers

import (
	"context"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// SearchParams defines the query parameters for a job search.
type SearchParams struct {
	Keywords []string // e.g. ["software engineer", "golang"]
	Location string   // e.g. "Remote" or "San Francisco, CA"
	Remote   bool     // only remote jobs
	MaxPages int      // max pages to scrape (default 5)
	SalaryMin int     // optional salary filter
}

// Scraper is the interface that all job board scrapers must implement.
type Scraper interface {
	// Source returns the job board identifier.
	Source() models.JobSource

	// Search scrapes job listings matching the given parameters.
	// Returns a channel that emits scraped jobs as they are found.
	Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error)

	// Name returns a human-readable name for the scraper.
	Name() string
}
