package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newJobRepo(t *testing.T) *repository.JobRepository {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	return repository.NewJobRepository(pool, zerolog.New(nil))
}

func makeScrapedJob(extID string) *models.ScrapedJob {
	return &models.ScrapedJob{
		ExternalID: extID,
		Source:     models.SourceIndeed,
		URL:        fmt.Sprintf("https://indeed.com/job/%s", extID),
		Title:      "Go Engineer",
		Company:    "TestCo",
		Location:   "New York",
		IsRemote:   false,
		ATSType:    models.ATSGreenhouse,
		ApplyURL:   "https://boards.greenhouse.io/testco/jobs/123",
	}
}

// cleanupJob deletes a job row by external_id + source_board after a test.
func cleanupJob(t *testing.T, extID string, source models.JobSource) {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	t.Cleanup(func() {
		pool.Exec(context.Background(),
			`DELETE FROM jobs WHERE external_id = $1 AND source_board = $2`,
			extID, string(source),
		) //nolint:errcheck
	})
}

// ── Upsert deduplication ──────────────────────────────────────────────────────

func TestJobRepo_Upsert_NewJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	extID := fmt.Sprintf("new-%s", uuid.New())
	job := makeScrapedJob(extID)
	cleanupJob(t, extID, models.SourceIndeed)

	id, isNew, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !isNew {
		t.Error("Upsert (new): isNew = false; want true")
	}
	if id == uuid.Nil {
		t.Error("Upsert (new): id is nil")
	}
}

func TestJobRepo_Upsert_Deduplication(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	extID := fmt.Sprintf("dupe-%s", uuid.New())
	job := makeScrapedJob(extID)
	cleanupJob(t, extID, models.SourceIndeed)

	id1, isNew1, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("Upsert (first): %v", err)
	}
	if !isNew1 {
		t.Error("Upsert (first): isNew should be true")
	}

	// Update title to verify update path
	job.Title = "Updated Go Engineer"
	id2, isNew2, err := repo.Upsert(ctx, job)
	if err != nil {
		t.Fatalf("Upsert (second): %v", err)
	}
	if isNew2 {
		t.Error("Upsert (second): isNew should be false (conflict/update)")
	}
	if id1 != id2 {
		t.Errorf("Upsert (second): id changed from %s to %s; should remain same", id1, id2)
	}
}

// ── BulkUpsert ────────────────────────────────────────────────────────────────

func TestJobRepo_BulkUpsert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	ext1 := fmt.Sprintf("bulk1-%s", uuid.New())
	ext2 := fmt.Sprintf("bulk2-%s", uuid.New())
	cleanupJob(t, ext1, models.SourceIndeed)
	cleanupJob(t, ext2, models.SourceIndeed)

	jobs := []*models.ScrapedJob{
		makeScrapedJob(ext1),
		makeScrapedJob(ext2),
	}

	newCount, dupeCount, err := repo.BulkUpsert(ctx, jobs)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}
	if newCount != 2 {
		t.Errorf("BulkUpsert: newCount = %d; want 2", newCount)
	}
	if dupeCount != 0 {
		t.Errorf("BulkUpsert: dupeCount = %d; want 0", dupeCount)
	}

	// Re-run same jobs — all dupes
	newCount2, dupeCount2, err := repo.BulkUpsert(ctx, jobs)
	if err != nil {
		t.Fatalf("BulkUpsert (dupe): %v", err)
	}
	if newCount2 != 0 {
		t.Errorf("BulkUpsert (dupe): newCount = %d; want 0", newCount2)
	}
	if dupeCount2 != 2 {
		t.Errorf("BulkUpsert (dupe): dupeCount = %d; want 2", dupeCount2)
	}
}

// ── GetActiveJobs with pagination ─────────────────────────────────────────────

func TestJobRepo_GetActiveJobs_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	// Insert 3 active jobs
	exts := []string{
		fmt.Sprintf("pag1-%s", uuid.New()),
		fmt.Sprintf("pag2-%s", uuid.New()),
		fmt.Sprintf("pag3-%s", uuid.New()),
	}
	for _, ext := range exts {
		cleanupJob(t, ext, models.SourceIndeed)
		if _, _, err := repo.Upsert(ctx, makeScrapedJob(ext)); err != nil {
			t.Fatalf("Upsert (%s): %v", ext, err)
		}
	}

	// Limit=2 → 2 rows
	page1, err := repo.GetActiveJobs(ctx, 2, 0)
	if err != nil {
		t.Fatalf("GetActiveJobs (limit=2): %v", err)
	}
	if len(page1) > 2 {
		t.Errorf("GetActiveJobs (limit=2): got %d; want <= 2", len(page1))
	}
}

// ── CountBySource ─────────────────────────────────────────────────────────────

func TestJobRepo_CountBySource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	ext := fmt.Sprintf("count-%s", uuid.New())
	cleanupJob(t, ext, models.SourceIndeed)

	if _, _, err := repo.Upsert(ctx, makeScrapedJob(ext)); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	counts, err := repo.CountBySource(ctx)
	if err != nil {
		t.Fatalf("CountBySource: %v", err)
	}
	indeedStats, ok := counts["indeed"]
	if !ok {
		t.Fatal("CountBySource: no entry for 'indeed'")
	}
	if indeedStats.Total < 1 {
		t.Errorf("CountBySource: indeed total = %d; want >= 1", indeedStats.Total)
	}
}

// ── MarkInactiveExcept ────────────────────────────────────────────────────────

func TestJobRepo_MarkInactiveExcept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newJobRepo(t)

	ext1 := fmt.Sprintf("inactive1-%s", uuid.New())
	ext2 := fmt.Sprintf("inactive2-%s", uuid.New())
	cleanupJob(t, ext1, models.SourceIndeed)
	cleanupJob(t, ext2, models.SourceIndeed)

	// Both active
	pool := testhelper.NewTestPool(t)
	now := time.Now()
	for _, ext := range []string{ext1, ext2} {
		_, err := pool.Exec(ctx,
			`INSERT INTO jobs (external_id, source_board, url, title, company, is_active, discovered_at)
			 VALUES ($1, 'indeed', $2, 'Job', 'Co', true, $3)`,
			ext, fmt.Sprintf("https://indeed.com/%s", ext), now,
		)
		if err != nil {
			t.Fatalf("insert job %s: %v", ext, err)
		}
	}

	// Mark ext2 active, everything else inactive
	if err := repo.MarkInactiveExcept(ctx, models.SourceIndeed, []string{ext2}); err != nil {
		t.Fatalf("MarkInactiveExcept: %v", err)
	}

	// ext1 should now be inactive
	var isActive bool
	if err := pool.QueryRow(ctx,
		`SELECT is_active FROM jobs WHERE external_id = $1 AND source_board = 'indeed'`, ext1,
	).Scan(&isActive); err != nil {
		t.Fatalf("scan ext1 is_active: %v", err)
	}
	if isActive {
		t.Errorf("MarkInactiveExcept: ext1 should be inactive")
	}

	// ext2 should remain active
	if err := pool.QueryRow(ctx,
		`SELECT is_active FROM jobs WHERE external_id = $1 AND source_board = 'indeed'`, ext2,
	).Scan(&isActive); err != nil {
		t.Fatalf("scan ext2 is_active: %v", err)
	}
	if !isActive {
		t.Errorf("MarkInactiveExcept: ext2 should be active")
	}
}
