// Package repository_test contains integration tests for ApplicationRepository.
//
// All tests require a real PostgreSQL connection. They are skipped automatically
// when TEST_DATABASE_URL is not set, so local development without a database
// still compiles and exits cleanly.
package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newRepo(t *testing.T) (*repository.ApplicationRepository, *pgxpool.Pool) {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	return repository.New(pool, testhelper.NopLogger()), pool
}

// repoFixtures holds the IDs of rows seeded before each test.
type repoFixtures struct {
	userID   uuid.UUID
	jobID    uuid.UUID
	matchID  uuid.UUID
	resumeID uuid.UUID
}

// seedRepoFixtures inserts the minimum rows required for an application row:
// a job, a user, a primary resume, and a match. Cleanup is registered via
// t.Cleanup in LIFO order so that the user (which cascades to everything else)
// is deleted first, then the job.
func seedRepoFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) repoFixtures {
	t.Helper()

	f := repoFixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		matchID:  uuid.New(),
		resumeID: uuid.New(),
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Repo Test Job', 'Repo Co', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("repo-ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedRepoFixtures: insert job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Repo User', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		f.userID,
		fmt.Sprintf("cognito-repo-%s", f.userID),
		fmt.Sprintf("repo-%s@example.com", f.userID),
	)
	if err != nil {
		t.Fatalf("seedRepoFixtures: insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO resumes
			(id, user_id, file_name, s3_key, is_primary, interview_count, created_at)
		VALUES ($1, $2, 'resume.pdf', $3, true, 0, NOW())`,
		f.resumeID, f.userID,
		fmt.Sprintf("resumes/%s/resume.pdf", f.userID),
	)
	if err != nil {
		t.Fatalf("seedRepoFixtures: insert resume: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO matches
			(id, user_id, job_id, match_score, status, created_at, updated_at)
		VALUES ($1, $2, $3, 0.9, 'pending', NOW(), NOW())`,
		f.matchID, f.userID, f.jobID,
	)
	if err != nil {
		t.Fatalf("seedRepoFixtures: insert match: %v", err)
	}

	return f
}

// insertApplication is a convenience helper that directly inserts an application
// row with the given status into the DB and returns its ID.
func insertApplication(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	f repoFixtures,
	status models.ApplicationStatus,
) uuid.UUID {
	t.Helper()
	appID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO applications
			(id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
		appID, f.userID, f.jobID, f.matchID, f.resumeID, string(status),
	)
	if err != nil {
		t.Fatalf("insertApplication: %v", err)
	}
	return appID
}

// ── Create + GetByID ──────────────────────────────────────────────────────────

// TestRepo_Create_GetByID_HappyPath creates an application via the repo and
// reads it back, verifying all persisted fields match.
func TestRepo_Create_GetByID_HappyPath(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	app, err := repo.Create(ctx, f.userID, f.jobID, f.matchID, f.resumeID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if app == nil {
		t.Fatal("Create returned nil")
	}
	if app.ID == uuid.Nil {
		t.Error("Create: returned application has nil ID")
	}
	if app.Status != models.StatusQueued {
		t.Errorf("Create: status = %s; want %s", app.Status, models.StatusQueued)
	}

	got, err := repo.GetByID(ctx, app.ID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != app.ID {
		t.Errorf("GetByID ID: got %s; want %s", got.ID, app.ID)
	}
	if got.UserID != f.userID {
		t.Errorf("GetByID UserID: got %s; want %s", got.UserID, f.userID)
	}
	if got.JobID != f.jobID {
		t.Errorf("GetByID JobID: got %s; want %s", got.JobID, f.jobID)
	}
	if got.MatchID != f.matchID {
		t.Errorf("GetByID MatchID: got %s; want %s", got.MatchID, f.matchID)
	}
	if got.ResumeID != f.resumeID {
		t.Errorf("GetByID ResumeID: got %s; want %s", got.ResumeID, f.resumeID)
	}
	if got.Status != models.StatusQueued {
		t.Errorf("GetByID Status: got %s; want %s", got.Status, models.StatusQueued)
	}
	if got.Outcome != nil {
		t.Errorf("GetByID Outcome: got %v; want nil", got.Outcome)
	}
}

// TestRepo_GetByID_WrongUserID verifies that fetching a valid application with
// the wrong userID returns ErrNotFound (ownership enforcement).
func TestRepo_GetByID_WrongUserID(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	appID := insertApplication(t, ctx, pool, f, models.StatusQueued)

	_, err := repo.GetByID(ctx, appID, uuid.New() /* wrong user */)
	if err == nil {
		t.Fatal("GetByID with wrong userID: expected error; got nil")
	}
	if err != repository.ErrNotFound {
		t.Errorf("GetByID with wrong userID: got %v; want ErrNotFound", err)
	}
}

// TestRepo_GetByID_NonExistentID verifies that fetching a completely unknown
// application ID returns ErrNotFound.
func TestRepo_GetByID_NonExistentID(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	_, err := repo.GetByID(ctx, uuid.New() /* does not exist */, f.userID)
	if err == nil {
		t.Fatal("GetByID with non-existent ID: expected error; got nil")
	}
	if err != repository.ErrNotFound {
		t.Errorf("GetByID with non-existent ID: got %v; want ErrNotFound", err)
	}
}

// ── UpdateStatus ──────────────────────────────────────────────────────────────

// TestRepo_UpdateStatus_ValidTransition creates an application in "queued" state
// and transitions it to "ai_preparing", verifying the new status is stored.
func TestRepo_UpdateStatus_ValidTransition(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	appID := insertApplication(t, ctx, pool, f, models.StatusQueued)

	if err := repo.UpdateStatus(ctx, appID, models.StatusAIPreparing); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// Verify via a direct DB query (not GetByID, which requires userID).
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM applications WHERE id = $1`, appID,
	).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if models.ApplicationStatus(status) != models.StatusAIPreparing {
		t.Errorf("status after UpdateStatus: got %q; want %q", status, models.StatusAIPreparing)
	}
}

// TestRepo_UpdateStatus_InvalidAppID verifies that updating a non-existent app
// returns an error (ErrNotFound from RowsAffected == 0).
func TestRepo_UpdateStatus_InvalidAppID(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	// No seeding needed — we just call with a UUID that has no row.
	_ = pool

	err := repo.UpdateStatus(ctx, uuid.New(), models.StatusApplied)
	if err == nil {
		t.Fatal("UpdateStatus with non-existent ID: expected error; got nil")
	}
	if err != repository.ErrNotFound {
		t.Errorf("UpdateStatus with non-existent ID: got %v; want ErrNotFound", err)
	}
}

// ── ListForUser ───────────────────────────────────────────────────────────────

// TestRepo_ListForUser_ReturnsOwnApps verifies that List returns only the
// applications belonging to the queried user and excludes other users' rows.
func TestRepo_ListForUser_ReturnsOwnApps(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()

	// Two independent users, each with their own fixture set.
	f1 := seedRepoFixtures(t, ctx, pool)

	// Second user — needs a separate job (UNIQUE user+job in matches).
	user2ID := uuid.New()
	job2ID := uuid.New()
	match2ID := uuid.New()
	resume2ID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'User2 Job', 'Co2', true, NOW(), NOW(), NOW())`,
		job2ID, fmt.Sprintf("u2-ext-%s", job2ID),
	)
	if err != nil {
		t.Fatalf("insert job2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, job2ID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'User2', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		user2ID,
		fmt.Sprintf("cognito-u2-%s", user2ID),
		fmt.Sprintf("u2-%s@example.com", user2ID),
	)
	if err != nil {
		t.Fatalf("insert user2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user2ID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO resumes
			(id, user_id, file_name, s3_key, is_primary, interview_count, created_at)
		VALUES ($1, $2, 'r2.pdf', $3, true, 0, NOW())`,
		resume2ID, user2ID, fmt.Sprintf("resumes/%s/r2.pdf", user2ID),
	)
	if err != nil {
		t.Fatalf("insert resume2: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO matches
			(id, user_id, job_id, match_score, status, created_at, updated_at)
		VALUES ($1, $2, $3, 0.85, 'pending', NOW(), NOW())`,
		match2ID, user2ID, job2ID,
	)
	if err != nil {
		t.Fatalf("insert match2: %v", err)
	}

	// One application for user1, one for user2.
	f2 := repoFixtures{
		userID: user2ID, jobID: job2ID, matchID: match2ID, resumeID: resume2ID,
	}
	insertApplication(t, ctx, pool, f1, models.StatusQueued)
	insertApplication(t, ctx, pool, f2, models.StatusQueued)

	apps1, total1, err := repo.ListForUser(ctx, f1.userID, "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser user1: %v", err)
	}
	for _, a := range apps1 {
		if a.UserID != f1.userID {
			t.Errorf("ListForUser: got application for wrong user %s", a.UserID)
		}
	}
	if total1 < 1 {
		t.Errorf("ListForUser user1: total = %d; want >= 1", total1)
	}

	apps2, total2, err := repo.ListForUser(ctx, f2.userID, "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser user2: %v", err)
	}
	for _, a := range apps2 {
		if a.UserID != f2.userID {
			t.Errorf("ListForUser: got application for wrong user %s", a.UserID)
		}
	}
	if total2 < 1 {
		t.Errorf("ListForUser user2: total = %d; want >= 1", total2)
	}

	// The two lists must be disjoint.
	ids1 := make(map[uuid.UUID]bool, len(apps1))
	for _, a := range apps1 {
		ids1[a.ID] = true
	}
	for _, a := range apps2 {
		if ids1[a.ID] {
			t.Errorf("ListForUser: application %s appears in both users' lists", a.ID)
		}
	}
}

// TestRepo_ListForUser_StatusFilter verifies that the status filter only
// returns applications with the requested status.
func TestRepo_ListForUser_StatusFilter(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	// We need two distinct jobs to insert two applications for the same user.
	job2ID := uuid.New()
	match2ID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Filter Job 2', 'Co', true, NOW(), NOW(), NOW())`,
		job2ID, fmt.Sprintf("filter-ext-%s", job2ID),
	)
	if err != nil {
		t.Fatalf("insert job2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, job2ID) //nolint:errcheck
	})
	_, err = pool.Exec(ctx, `
		INSERT INTO matches
			(id, user_id, job_id, match_score, status, created_at, updated_at)
		VALUES ($1, $2, $3, 0.8, 'pending', NOW(), NOW())`,
		match2ID, f.userID, job2ID,
	)
	if err != nil {
		t.Fatalf("insert match2: %v", err)
	}
	f2 := repoFixtures{
		userID: f.userID, jobID: job2ID, matchID: match2ID, resumeID: f.resumeID,
	}

	insertApplication(t, ctx, pool, f, models.StatusQueued)
	insertApplication(t, ctx, pool, f2, models.StatusApplied)

	// Filter: only queued.
	apps, total, err := repo.ListForUser(ctx, f.userID, models.StatusQueued, "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser with status filter: %v", err)
	}
	if total < 1 {
		t.Errorf("total filtered: got %d; want >= 1", total)
	}
	for _, a := range apps {
		if a.Status != models.StatusQueued {
			t.Errorf("status filter: got %s; want %s", a.Status, models.StatusQueued)
		}
	}
}

// TestRepo_ListForUser_Pagination verifies that limit and offset work correctly
// by inserting 3 applications and fetching them page-by-page.
func TestRepo_ListForUser_Pagination(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	// Insert 3 applications across 3 distinct jobs.
	const n = 3
	for i := 0; i < n; i++ {
		jobID := uuid.New()
		matchID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO jobs
				(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
			VALUES ($1, $2, 'test', $3, 'PagCo', true, NOW(), NOW(), NOW())`,
			jobID, fmt.Sprintf("pag-ext-%d-%s", i, jobID), fmt.Sprintf("Pag Job %d", i),
		)
		if err != nil {
			t.Fatalf("insert job %d: %v", i, err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID) //nolint:errcheck
		})
		_, err = pool.Exec(ctx, `
			INSERT INTO matches
				(id, user_id, job_id, match_score, status, created_at, updated_at)
			VALUES ($1, $2, $3, 0.8, 'pending', NOW(), NOW())`,
			matchID, f.userID, jobID,
		)
		if err != nil {
			t.Fatalf("insert match %d: %v", i, err)
		}
		fi := repoFixtures{userID: f.userID, jobID: jobID, matchID: matchID, resumeID: f.resumeID}
		insertApplication(t, ctx, pool, fi, models.StatusQueued)
	}

	// Page 1: first 2 rows.
	page1, total, err := repo.ListForUser(ctx, f.userID, "", "", 2, 0)
	if err != nil {
		t.Fatalf("ListForUser page1: %v", err)
	}
	if total < n {
		t.Errorf("total: got %d; want >= %d", total, n)
	}
	if len(page1) != 2 {
		t.Errorf("page1 len: got %d; want 2", len(page1))
	}

	// Page 2: remaining rows (at least 1).
	page2, _, err := repo.ListForUser(ctx, f.userID, "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListForUser page2: %v", err)
	}
	if len(page2) < 1 {
		t.Errorf("page2 len: got %d; want >= 1", len(page2))
	}

	// No overlap between pages.
	seen := make(map[uuid.UUID]bool, len(page1))
	for _, a := range page1 {
		seen[a.ID] = true
	}
	for _, a := range page2 {
		if seen[a.ID] {
			t.Errorf("pagination: application %s appears on both pages", a.ID)
		}
	}
}

// ── CountByStatus ─────────────────────────────────────────────────────────────

// TestRepo_CountByStatus_CorrectCounts verifies the grouped counts are accurate.
func TestRepo_CountByStatus_CorrectCounts(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	// Insert: 2 queued, 1 applied, 1 failed — all for the same user.
	type entry struct {
		status models.ApplicationStatus
		count  int
	}
	entries := []entry{
		{models.StatusQueued, 2},
		{models.StatusApplied, 1},
		{models.StatusFailed, 1},
	}

	jobIndex := 0
	for _, e := range entries {
		for j := 0; j < e.count; j++ {
			jobID := uuid.New()
			matchID := uuid.New()
			_, err := pool.Exec(ctx, `
				INSERT INTO jobs
					(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
				VALUES ($1, $2, 'test', $3, 'CntCo', true, NOW(), NOW(), NOW())`,
				jobID, fmt.Sprintf("cnt-ext-%d-%s", jobIndex, jobID), fmt.Sprintf("Cnt Job %d", jobIndex),
			)
			if err != nil {
				t.Fatalf("insert job: %v", err)
			}
			jobIndex++
			t.Cleanup(func() {
				pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID) //nolint:errcheck
			})
			_, err = pool.Exec(ctx, `
				INSERT INTO matches
					(id, user_id, job_id, match_score, status, created_at, updated_at)
				VALUES ($1, $2, $3, 0.8, 'pending', NOW(), NOW())`,
				matchID, f.userID, jobID,
			)
			if err != nil {
				t.Fatalf("insert match: %v", err)
			}
			fi := repoFixtures{userID: f.userID, jobID: jobID, matchID: matchID, resumeID: f.resumeID}
			insertApplication(t, ctx, pool, fi, e.status)
		}
	}

	counts, err := repo.CountByStatus(ctx, f.userID)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts == nil {
		t.Fatal("CountByStatus: returned nil map")
	}

	for _, e := range entries {
		if counts[e.status] < e.count {
			t.Errorf("CountByStatus[%s]: got %d; want >= %d",
				e.status, counts[e.status], e.count)
		}
	}
}

// TestRepo_CountByStatus_ZeroCounts verifies that when a user has no
// applications the returned map is non-nil and contains no entries.
func TestRepo_CountByStatus_ZeroCounts(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	_ = pool // pool only used to satisfy newRepo signature

	counts, err := repo.CountByStatus(ctx, uuid.New())
	if err != nil {
		t.Fatalf("CountByStatus empty: %v", err)
	}
	if counts == nil {
		t.Fatal("CountByStatus empty: returned nil map; want non-nil empty map")
	}
	if len(counts) != 0 {
		t.Errorf("CountByStatus empty: map has %d entries; want 0", len(counts))
	}
}

// ── UpdateStatus (additional transitions) ────────────────────────────────────

// TestRepo_UpdateStatus_MultipleTransitions walks an application through several
// status transitions and verifies the final persisted state after each step.
func TestRepo_UpdateStatus_MultipleTransitions(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	appID := insertApplication(t, ctx, pool, f, models.StatusQueued)

	transitions := []models.ApplicationStatus{
		models.StatusAIPreparing,
		models.StatusAIReady,
		models.StatusApplying,
		models.StatusApplied,
	}

	for _, next := range transitions {
		if err := repo.UpdateStatus(ctx, appID, next); err != nil {
			t.Fatalf("UpdateStatus to %s: %v", next, err)
		}
		var got string
		if err := pool.QueryRow(ctx,
			`SELECT status FROM applications WHERE id = $1`, appID,
		).Scan(&got); err != nil {
			t.Fatalf("query status after transition to %s: %v", next, err)
		}
		if models.ApplicationStatus(got) != next {
			t.Errorf("after transition to %s: got %q", next, got)
		}
	}
}

// ── GetByID ownership enforcement (additional) ────────────────────────────────

// TestRepo_Create_GetByID_OwnershipEnforcement verifies that the owner of a
// freshly created application can retrieve it, but a different user cannot.
func TestRepo_Create_GetByID_OwnershipEnforcement(t *testing.T) {
	repo, pool := newRepo(t)
	ctx := context.Background()
	f := seedRepoFixtures(t, ctx, pool)

	app, err := repo.Create(ctx, f.userID, f.jobID, f.matchID, f.resumeID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Correct user — must succeed.
	if _, err := repo.GetByID(ctx, app.ID, f.userID); err != nil {
		t.Errorf("GetByID owner: unexpected error: %v", err)
	}

	// Wrong user — must return ErrNotFound.
	_, err = repo.GetByID(ctx, app.ID, uuid.New())
	if err != repository.ErrNotFound {
		t.Errorf("GetByID wrong user: got %v; want ErrNotFound", err)
	}
}
