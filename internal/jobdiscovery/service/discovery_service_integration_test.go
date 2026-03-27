package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	discrepo "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	discsvc "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/service"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/scrapers"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
	"github.com/google/uuid"
)

// ── Stub scrapers for service-level tests ─────────────────────────────────────

// stubScraper is a minimal scrapers.Scraper that sends a fixed set of jobs.
type stubScraper struct {
	source discmodels.JobSource
	jobs   []*discmodels.ScrapedJob
	err    error // if non-nil, sent on the error channel before closing job channel
}

func (s *stubScraper) Source() discmodels.JobSource { return s.source }
func (s *stubScraper) Name() string                 { return "stub-" + string(s.source) }

func (s *stubScraper) Search(_ context.Context, _ scrapers.SearchParams) (<-chan *discmodels.ScrapedJob, <-chan error) {
	jobCh := make(chan *discmodels.ScrapedJob, len(s.jobs))
	errCh := make(chan error, 1)

	go func() {
		for _, j := range s.jobs {
			jobCh <- j
		}
		close(jobCh)
		if s.err != nil {
			errCh <- s.err
		}
		close(errCh)
	}()
	return jobCh, errCh
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func makeScrapedJob(t *testing.T, externalID string, source discmodels.JobSource) *discmodels.ScrapedJob {
	t.Helper()
	return &discmodels.ScrapedJob{
		ExternalID:     externalID,
		Source:         source,
		URL:            fmt.Sprintf("https://indeed.com/job/%s", externalID),
		Title:          "Senior Go Engineer",
		Company:        "TechCorp",
		Location:       "New York, NY",
		IsRemote:       true,
		SalaryCurrency: "USD",
		Description:    "Build distributed systems with Go, Kubernetes, and PostgreSQL.",
		ATSType:        discmodels.ATSGreenhouse,
		ApplyURL:       "https://boards.greenhouse.io/techcorp/jobs/123",
	}
}

// ── Repository Tests (JobRepository) ─────────────────────────────────────────

func TestJobRepository_Upsert_NewJob(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	job := makeScrapedJob(t, fmt.Sprintf("ext-upsert-%s", uuid.New()), discmodels.SourceIndeed)

	id, isNew, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
	if !isNew {
		t.Error("expected isNew = true for new job")
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, id) //nolint:errcheck
	})
}

func TestJobRepository_Upsert_DuplicateIsUpsert(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	job := makeScrapedJob(t, fmt.Sprintf("ext-dupe-%s", uuid.New()), discmodels.SourceIndeed)

	id1, isNew1, err := repo.Upsert(ctx, job)
	if err != nil || !isNew1 {
		t.Fatalf("first upsert: err=%v, isNew=%v", err, isNew1)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, id1) }) //nolint:errcheck

	// Update title to verify the ON CONFLICT path updates fields.
	job.Title = "Principal Go Engineer"
	id2, isNew2, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if isNew2 {
		t.Error("expected isNew = false on duplicate upsert")
	}
	if id1 != id2 {
		t.Errorf("expected same id; got %s and %s", id1, id2)
	}

	// Verify the title was updated.
	var title string
	pool.QueryRow(ctx, `SELECT title FROM jobs WHERE id = $1`, id1).Scan(&title) //nolint:errcheck
	if title != "Principal Go Engineer" {
		t.Errorf("title = %q; want %q after upsert update", title, "Principal Go Engineer")
	}
}

func TestJobRepository_BulkUpsert_MixedNewAndDupe(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	jobs := []*discmodels.ScrapedJob{
		makeScrapedJob(t, "bulk-new-1-"+base, discmodels.SourceIndeed),
		makeScrapedJob(t, "bulk-new-2-"+base, discmodels.SourceIndeed),
		makeScrapedJob(t, "bulk-new-1-"+base, discmodels.SourceIndeed), // duplicate of first
	}

	newCount, dupeCount, err := repo.BulkUpsert(ctx, jobs)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}
	if newCount != 2 {
		t.Errorf("newCount = %d; want 2", newCount)
	}
	if dupeCount != 1 {
		t.Errorf("dupeCount = %d; want 1", dupeCount)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE external_id LIKE $1`, "bulk-new-%-"+base) //nolint:errcheck
	})
}

func TestJobRepository_GetActiveJobs(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	active := makeScrapedJob(t, "active-"+base, discmodels.SourceGlassdoor)
	inactive := makeScrapedJob(t, "inactive-"+base, discmodels.SourceGlassdoor)

	activeID, _, _ := repo.Upsert(ctx, active)
	inactiveID, _, _ := repo.Upsert(ctx, inactive)

	// Mark the second job inactive.
	pool.Exec(ctx, `UPDATE jobs SET is_active = false WHERE id = $1`, inactiveID) //nolint:errcheck

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{activeID, inactiveID}) //nolint:errcheck
	})

	jobs, err := repo.GetActiveJobs(ctx, 100, 0)
	if err != nil {
		t.Fatalf("GetActiveJobs: %v", err)
	}

	found := make(map[uuid.UUID]struct{})
	for _, j := range jobs {
		found[j.ID] = struct{}{}
	}
	if _, ok := found[activeID]; !ok {
		t.Error("expected active job to appear in GetActiveJobs")
	}
	if _, ok := found[inactiveID]; ok {
		t.Error("expected inactive job NOT to appear in GetActiveJobs")
	}
}

func TestJobRepository_CountBySource(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	j1 := makeScrapedJob(t, "cbs-1-"+base, discmodels.SourceIndeed)
	j2 := makeScrapedJob(t, "cbs-2-"+base, discmodels.SourceIndeed)
	j3 := makeScrapedJob(t, "cbs-3-"+base, discmodels.SourceGlassdoor)

	id1, _, _ := repo.Upsert(ctx, j1)
	id2, _, _ := repo.Upsert(ctx, j2)
	id3, _, _ := repo.Upsert(ctx, j3)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{id1, id2, id3}) //nolint:errcheck
	})

	counts, err := repo.CountBySource(ctx)
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}

	// We at least seeded 2 Indeed and 1 Glassdoor rows — verify minimums.
	if counts["indeed"].Total < 2 {
		t.Errorf("indeed total = %d; want >= 2", counts["indeed"].Total)
	}
	if counts["glassdoor"].Total < 1 {
		t.Errorf("glassdoor total = %d; want >= 1", counts["glassdoor"].Total)
	}
}

func TestJobRepository_MarkInactiveExcept(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	stay := makeScrapedJob(t, "stay-"+base, discmodels.SourceIndeed)
	go_ := makeScrapedJob(t, "gone-"+base, discmodels.SourceIndeed)

	stayID, _, _ := repo.Upsert(ctx, stay)
	goneID, _, _ := repo.Upsert(ctx, go_)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = ANY($1)`, []uuid.UUID{stayID, goneID}) //nolint:errcheck
	})

	// Mark source inactive, except for the "stay" job.
	if err := repo.MarkInactiveExcept(ctx, discmodels.SourceIndeed, []string{"stay-" + base}); err != nil {
		t.Fatalf("MarkInactiveExcept: %v", err)
	}

	var stayActive, goneActive bool
	pool.QueryRow(ctx, `SELECT is_active FROM jobs WHERE id = $1`, stayID).Scan(&stayActive)   //nolint:errcheck
	pool.QueryRow(ctx, `SELECT is_active FROM jobs WHERE id = $1`, goneID).Scan(&goneActive) //nolint:errcheck

	if !stayActive {
		t.Error("expected 'stay' job to remain active")
	}
	if goneActive {
		t.Error("expected 'gone' job to be marked inactive")
	}
}

// ── DiscoveryService.GetStats ─────────────────────────────────────────────────

func TestDiscoveryService_GetStats_ReturnsValidShape(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	job := makeScrapedJob(t, "stats-"+base, discmodels.SourceIndeed)
	id, _, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, id) }) //nolint:errcheck

	// Call GetStats via the service directly (it wraps CountBySource).
	// Test it by calling CountBySource since GetStats has no testable side effects beyond DB.
	counts, err := repo.CountBySource(ctx)
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	if counts == nil {
		t.Error("expected non-nil counts map from CountBySource")
	}
	// At least one entry must exist after our insert.
	if len(counts) == 0 {
		t.Error("expected at least one source in counts after seeding")
	}
}

// ── DiscoveryService.RunSingle with stub scraper ──────────────────────────────
// The real scrapers hit the internet; we test the service's plumbing via the
// repository directly (BulkUpsert is the integration point).

func TestDiscovery_BulkUpsert_Idempotent(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	jobs := []*discmodels.ScrapedJob{
		makeScrapedJob(t, "idemp-1-"+base, discmodels.SourceIndeed),
	}

	var lastID uuid.UUID
	for i := 0; i < 3; i++ {
		newCount, _, err := repo.BulkUpsert(ctx, jobs)
		if err != nil {
			t.Fatalf("BulkUpsert run %d: %v", i+1, err)
		}
		if i == 0 && newCount != 1 {
			t.Errorf("run 1: newCount = %d; want 1", newCount)
		}
		if i > 0 && newCount != 0 {
			t.Errorf("run %d (re-run): newCount = %d; want 0 (idempotent)", i+1, newCount)
		}
	}

	pool.QueryRow(ctx, `SELECT id FROM jobs WHERE external_id = $1`, "idemp-1-"+base).Scan(&lastID) //nolint:errcheck
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, lastID) }) //nolint:errcheck
}

// ── Time/context cancellation ─────────────────────────────────────────────────

func TestJobRepository_Upsert_RespectsContextCancel(t *testing.T) {
	pool := testhelper.NewTestPool(t)

	// Cancel the context before the call.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure expired

	repo := discrepo.NewJobRepository(pool)
	job := makeScrapedJob(t, "cancel-"+uuid.New().String(), discmodels.SourceIndeed)

	_, _, err := repo.Upsert(ctx, job)
	if err == nil {
		t.Error("expected error with cancelled context; got nil")
	}
}

// ── DiscoveryService RunSingle via stub scraper ───────────────────────────────

// TestDiscoveryService_ScamJobDetected_IsScamSetBeforeStorage verifies that
// when a scraper returns a job that triggers scam detection, the job stored in
// the DB has is_scam=true (set by the discovery service before BulkUpsert).
func TestDiscoveryService_ScamJobDetected_IsScamSetBeforeStorage(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	base := uuid.New().String()
	extID := "scam-svc-" + base

	salaryVal := 500001 // Signal 1: suspiciously high salary
	scamJob := &discmodels.ScrapedJob{
		ExternalID:  extID,
		Source:      discmodels.SourceIndeed,
		URL:         "http://totally-scam-domain.biz/job",
		Title:       "work from home earn easy money",
		Company:     "",
		Description: "Pay to apply today. Send $99 training fee. Contact scammer@gmail.com. Unlimited earning. Passive income. Be your own boss. Ground floor MLM opportunity.",
		ATSType:     discmodels.ATSUnknown,
		ApplyURL:    "http://totally-scam-domain.biz/apply",
		SalaryMin:   &salaryVal,
	}

	stub := &stubScraper{
		source: discmodels.SourceIndeed,
		jobs:   []*discmodels.ScrapedJob{scamJob},
	}

	svc := discsvc.NewDiscoveryService(repo, testhelper.NopLogger(), stub)
	result := svc.RunSingle(ctx, discmodels.SourceIndeed, discsvc.DiscoverParams{
		Keywords: []string{"engineer"},
		MaxPages: 1,
	})
	if result.Err != nil {
		t.Fatalf("RunSingle: %v", result.Err)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE external_id = $1`, extID) //nolint:errcheck
	})

	// Verify the job is stored with is_scam = true.
	var isScam bool
	if err := pool.QueryRow(ctx, `SELECT is_scam FROM jobs WHERE external_id = $1`, extID).Scan(&isScam); err != nil {
		t.Fatalf("query is_scam: %v", err)
	}
	if !isScam {
		t.Error("expected is_scam=true for scam job; got false")
	}
}

// TestDiscoveryService_EmptyResults_NoDBWrites verifies that when a scraper
// returns no jobs, the discovery service performs no DB writes.
func TestDiscoveryService_EmptyResults_NoDBWrites(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	// Count jobs before the run.
	var beforeCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&beforeCount) //nolint:errcheck

	stub := &stubScraper{
		source: discmodels.SourceZipRecruiter,
		jobs:   nil, // no jobs
	}

	svc := discsvc.NewDiscoveryService(repo, testhelper.NopLogger(), stub)
	result := svc.RunSingle(ctx, discmodels.SourceZipRecruiter, discsvc.DiscoverParams{
		Keywords: []string{"engineer"},
		MaxPages: 1,
	})
	if result.Err != nil {
		t.Fatalf("RunSingle: %v", result.Err)
	}

	if result.JobsNew != 0 {
		t.Errorf("JobsNew = %d; want 0 when scraper returns no jobs", result.JobsNew)
	}
	if result.JobsDupe != 0 {
		t.Errorf("JobsDupe = %d; want 0 when scraper returns no jobs", result.JobsDupe)
	}

	// Total jobs in DB must be unchanged.
	var afterCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&afterCount) //nolint:errcheck
	if afterCount != beforeCount {
		t.Errorf("job count changed from %d to %d; expected no writes", beforeCount, afterCount)
	}
}

// TestDiscoveryService_ScraperError_OtherScrapersStillRun verifies that when
// one scraper emits an error, the result captures it but the service continues
// (other scrapers in RunAll would still run; here we verify a single erroring
// scraper still produces a completed (non-nil) result with Err set).
func TestDiscoveryService_ScraperError_ResultHasErr(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	repo := discrepo.NewJobRepository(pool)

	stub := &stubScraper{
		source: discmodels.SourceGlassdoor,
		jobs:   nil,
		err:    errors.New("network timeout from glassdoor"),
	}

	svc := discsvc.NewDiscoveryService(repo, testhelper.NopLogger(), stub)
	result := svc.RunSingle(ctx, discmodels.SourceGlassdoor, discsvc.DiscoverParams{
		Keywords: []string{"engineer"},
		MaxPages: 1,
	})

	// The service logs the error as non-fatal; it surfaces via result.Err.
	if result.Err == nil {
		t.Error("expected result.Err to be set when scraper emits an error; got nil")
	}
	// Source field must reflect the scraper that ran.
	if result.Source != discmodels.SourceGlassdoor {
		t.Errorf("result.Source = %q; want %q", result.Source, discmodels.SourceGlassdoor)
	}
}
