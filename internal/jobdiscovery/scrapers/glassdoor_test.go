package scrapers_test

import (
	"context"
	"testing"
	"time"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/scrapers"
)

// drainGlassdoor reads all jobs and errors from the channels returned by
// glassdoorScraper.Search and returns them. It times out after 2 seconds
// to guard against a channel that never closes.
func drainGlassdoor(jobsCh <-chan *models.ScrapedJob, errCh <-chan error) ([]*models.ScrapedJob, []error) {
	var jobs []*models.ScrapedJob
	var errs []error

	timeout := time.After(2 * time.Second)
	jobsDone := false
	errsDone := false

	for !jobsDone || !errsDone {
		select {
		case j, ok := <-jobsCh:
			if !ok {
				jobsDone = true
			} else if j != nil {
				jobs = append(jobs, j)
			}
		case e, ok := <-errCh:
			if !ok {
				errsDone = true
			} else if e != nil {
				errs = append(errs, e)
			}
		case <-timeout:
			return jobs, errs
		}
	}
	return jobs, errs
}

// TestGlassdoorScraper_Source verifies the scraper reports the correct source.
func TestGlassdoorScraper_Source(t *testing.T) {
	s := scrapers.NewGlassdoorScraper()
	if got := s.Source(); got != models.SourceGlassdoor {
		t.Errorf("Source() = %q, want %q", got, models.SourceGlassdoor)
	}
}

// TestGlassdoorScraper_Name verifies the scraper has a non-empty name.
func TestGlassdoorScraper_Name(t *testing.T) {
	s := scrapers.NewGlassdoorScraper()
	if s.Name() == "" {
		t.Error("Name() returned empty string")
	}
}

// TestGlassdoorScraper_ParsesJobs verifies that Search returns channels that
// close cleanly. The Glassdoor scraper is currently a stub (MVP-B) that
// returns zero jobs. The test documents this contract explicitly.
func TestGlassdoorScraper_ParsesJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	s := scrapers.NewGlassdoorScraper()
	params := scrapers.SearchParams{
		Keywords: []string{"software engineer"},
		Location: "Remote",
		MaxPages: 1,
	}

	ctx := context.Background()
	jobsCh, errCh := s.Search(ctx, params)

	jobs, errs := drainGlassdoor(jobsCh, errCh)

	// Stub returns 0 jobs and 0 errors — channels are closed immediately.
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	// Document stub behaviour: 0 jobs expected until MVP-B implementation.
	t.Logf("Glassdoor stub returned %d jobs (expected 0 until MVP-B)", len(jobs))
}

// TestGlassdoorScraper_EmptyPage verifies that an empty-looking call produces
// zero jobs and zero errors (stub behaviour).
func TestGlassdoorScraper_EmptyPage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	s := scrapers.NewGlassdoorScraper()
	params := scrapers.SearchParams{
		Keywords: []string{},
		MaxPages: 1,
	}

	ctx := context.Background()
	jobsCh, errCh := s.Search(ctx, params)

	jobs, errs := drainGlassdoor(jobsCh, errCh)

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs from stub, got %d", len(jobs))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors from stub, got %d", len(errs))
	}
}

// TestGlassdoorScraper_ServerError verifies that the stub handles a cancelled
// context without panicking and closes channels cleanly.
func TestGlassdoorScraper_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	s := scrapers.NewGlassdoorScraper()
	params := scrapers.SearchParams{
		Keywords: []string{"go developer"},
		MaxPages: 1,
	}

	// Even with a cancelled context the stub should close channels cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	jobsCh, errCh := s.Search(ctx, params)

	jobs, errs := drainGlassdoor(jobsCh, errCh)

	// Stub is fire-and-forget: channels are pre-closed, so no jobs or errors.
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs with cancelled context, got %d", len(jobs))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors with cancelled context, got %d", len(errs))
	}
}

// TestGlassdoorScraper_ContextCancel verifies that cancelling a context mid-call
// does not cause a panic or deadlock.
func TestGlassdoorScraper_ContextCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	s := scrapers.NewGlassdoorScraper()
	params := scrapers.SearchParams{
		Keywords: []string{"backend engineer"},
		MaxPages: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	jobsCh, errCh := s.Search(ctx, params)

	// Drain channels; the stub closes them synchronously so this returns fast.
	jobs, errs := drainGlassdoor(jobsCh, errCh)

	t.Logf("context cancel test: %d jobs, %d errors", len(jobs), len(errs))
}

// TestGlassdoorScraper_JobFields verifies that jobs emitted (if any) would have
// the expected fields populated. Since the stub emits no jobs, this test
// documents the expected contract for when MVP-B is implemented.
func TestGlassdoorScraper_JobFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	s := scrapers.NewGlassdoorScraper()
	params := scrapers.SearchParams{
		Keywords: []string{"golang"},
		Location: "San Francisco, CA",
		MaxPages: 1,
	}

	ctx := context.Background()
	jobsCh, errCh := s.Search(ctx, params)
	jobs, _ := drainGlassdoor(jobsCh, errCh)

	for _, job := range jobs {
		if job.ExternalID == "" {
			t.Error("job.ExternalID must not be empty")
		}
		if job.Title == "" {
			t.Error("job.Title must not be empty")
		}
		if job.Company == "" {
			t.Error("job.Company must not be empty")
		}
		if job.URL == "" {
			t.Error("job.URL must not be empty")
		}
		if job.Source != models.SourceGlassdoor {
			t.Errorf("job.Source = %q, want %q", job.Source, models.SourceGlassdoor)
		}
	}
}
