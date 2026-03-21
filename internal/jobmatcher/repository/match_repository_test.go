package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Seed helpers ──────────────────────────────────────────────────────────────

type matchRepoFixtures struct {
	userID uuid.UUID
	jobID  uuid.UUID
}

func seedMatchRepoFixtures(t *testing.T, ctx context.Context) matchRepoFixtures {
	t.Helper()
	pool := testhelper.NewTestPool(t)

	f := matchRepoFixtures{
		userID: uuid.New(),
		jobID:  uuid.New(),
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO jobs (id, external_id, source_board, title, company, url, is_active, discovered_at, created_at, updated_at)
		 VALUES ($1, $2, 'testboard', 'Test Job', 'TestCo', 'https://example.com/job', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedMatchRepoFixtures: insert job: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, cognito_sub, email, full_name, is_active)
		 VALUES ($1, $2, $3, 'Match Repo User', true)`,
		f.userID,
		fmt.Sprintf("sub-%s", f.userID),
		fmt.Sprintf("matchrepo-%s@example.com", f.userID),
	)
	if err != nil {
		t.Fatalf("seedMatchRepoFixtures: insert user: %v", err)
	}

	// LIFO: user first (cascades matches), then job
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)   //nolint:errcheck
	})
	return f
}

func newMatchRepo(t *testing.T) *repository.MatchRepository {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	return repository.New(pool, testhelper.NopLogger())
}

// ── BulkInsert deduplication ──────────────────────────────────────────────────

func TestMatchRepo_BulkInsert_Deduplication(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	m := repository.MatchInsert{
		UserID: f.userID,
		JobID:  f.jobID,
		Score:  0.85,
		Status: models.MatchStatusPending,
	}

	// First insert: expect 1
	n, err := repo.BulkInsert(ctx, []repository.MatchInsert{m})
	if err != nil {
		t.Fatalf("BulkInsert (first): %v", err)
	}
	if n != 1 {
		t.Errorf("BulkInsert (first): inserted = %d; want 1", n)
	}

	// Second insert: same user+job → expect 0 (ON CONFLICT DO NOTHING)
	n, err = repo.BulkInsert(ctx, []repository.MatchInsert{m})
	if err != nil {
		t.Fatalf("BulkInsert (dupe): %v", err)
	}
	if n != 0 {
		t.Errorf("BulkInsert (dupe): inserted = %d; want 0", n)
	}
}

func TestMatchRepo_BulkInsert_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newMatchRepo(t)

	n, err := repo.BulkInsert(ctx, nil)
	if err != nil {
		t.Fatalf("BulkInsert (empty): %v", err)
	}
	if n != 0 {
		t.Errorf("BulkInsert (empty): inserted = %d; want 0", n)
	}
}

// ── ListForUser with status filter + pagination ────────────────────────────────

func TestMatchRepo_ListForUser_AllStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	// Insert a match
	_, err := repo.BulkInsert(ctx, []repository.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.9, Status: models.MatchStatusPending,
	}})
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	matches, total, err := repo.ListForUser(ctx, f.userID, "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if total < 1 {
		t.Errorf("ListForUser: total = %d; want >= 1", total)
	}
	if len(matches) < 1 {
		t.Errorf("ListForUser: len = %d; want >= 1", len(matches))
	}
}

func TestMatchRepo_ListForUser_StatusFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	_, err := repo.BulkInsert(ctx, []repository.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.75, Status: models.MatchStatusPending,
	}})
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	// Filter by approved — should return 0 rows since status is pending
	matches, total, err := repo.ListForUser(ctx, f.userID, models.MatchStatusApproved, 20, 0)
	if err != nil {
		t.Fatalf("ListForUser (approved filter): %v", err)
	}
	if total != 0 {
		t.Errorf("ListForUser (approved): total = %d; want 0", total)
	}
	if len(matches) != 0 {
		t.Errorf("ListForUser (approved): len = %d; want 0", len(matches))
	}

	// Filter by pending — should find it
	matches, total, err = repo.ListForUser(ctx, f.userID, models.MatchStatusPending, 20, 0)
	if err != nil {
		t.Fatalf("ListForUser (pending filter): %v", err)
	}
	if total < 1 {
		t.Errorf("ListForUser (pending): total = %d; want >= 1", total)
	}
	_ = matches
}

func TestMatchRepo_ListForUser_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	_, err := repo.BulkInsert(ctx, []repository.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.9, Status: models.MatchStatusPending,
	}})
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	// offset=100 → empty result but correct total
	matches, total, err := repo.ListForUser(ctx, f.userID, "", 20, 100)
	if err != nil {
		t.Fatalf("ListForUser (offset=100): %v", err)
	}
	if total < 1 {
		t.Errorf("ListForUser (offset=100): total should reflect all rows, got %d", total)
	}
	if len(matches) != 0 {
		t.Errorf("ListForUser (offset=100): expected empty page; got %d", len(matches))
	}
}

// ── UpdateStatus (ApproveMatch / RejectMatch) ─────────────────────────────────

func TestMatchRepo_UpdateStatus_Approve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	_, err := repo.BulkInsert(ctx, []repository.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.8, Status: models.MatchStatusPending,
	}})
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	// Retrieve match ID
	matches, _, _ := repo.ListForUser(ctx, f.userID, models.MatchStatusPending, 1, 0)
	if len(matches) == 0 {
		t.Fatal("no pending match found")
	}
	matchID := matches[0].ID

	if err := repo.UpdateStatus(ctx, matchID, f.userID, models.MatchStatusApproved); err != nil {
		t.Fatalf("UpdateStatus (approve): %v", err)
	}

	// Verify
	updated, err := repo.GetByID(ctx, matchID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != models.MatchStatusApproved {
		t.Errorf("UpdateStatus: status = %q; want %q", updated.Status, models.MatchStatusApproved)
	}
}

func TestMatchRepo_UpdateStatus_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	err := repo.UpdateStatus(ctx, uuid.New(), f.userID, models.MatchStatusApproved)
	if err == nil {
		t.Error("UpdateStatus (not found): expected error; got nil")
	}
}

// ── SetFeedback ───────────────────────────────────────────────────────────────

func TestMatchRepo_SetFeedback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	_, err := repo.BulkInsert(ctx, []repository.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.7, Status: models.MatchStatusPending,
	}})
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	matches, _, _ := repo.ListForUser(ctx, f.userID, "", 1, 0)
	if len(matches) == 0 {
		t.Fatal("no match found")
	}
	matchID := matches[0].ID

	if err := repo.SetFeedback(ctx, matchID, f.userID, "thumbs_up"); err != nil {
		t.Fatalf("SetFeedback: %v", err)
	}

	updated, err := repo.GetByID(ctx, matchID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.UserFeedback != "thumbs_up" {
		t.Errorf("SetFeedback: feedback = %q; want thumbs_up", updated.UserFeedback)
	}
}

func TestMatchRepo_SetFeedback_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	err := repo.SetFeedback(ctx, uuid.New(), f.userID, "thumbs_down")
	if err == nil {
		t.Error("SetFeedback (not found): expected error; got nil")
	}
}

// ── GetUserIDBySub ────────────────────────────────────────────────────────────

func TestMatchRepo_GetUserIDBySub_Found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchRepoFixtures(t, ctx)
	repo := newMatchRepo(t)

	sub := fmt.Sprintf("sub-%s", f.userID)
	id, err := repo.GetUserIDBySub(ctx, sub)
	if err != nil {
		t.Fatalf("GetUserIDBySub: %v", err)
	}
	if id != f.userID {
		t.Errorf("GetUserIDBySub: id = %s; want %s", id, f.userID)
	}
}

func TestMatchRepo_GetUserIDBySub_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newMatchRepo(t)

	_, err := repo.GetUserIDBySub(ctx, "nonexistent-sub")
	if err == nil {
		t.Error("GetUserIDBySub (not found): expected error; got nil")
	}
}
