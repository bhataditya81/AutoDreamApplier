// Package service_test contains unit/integration tests for the application
// service layer.
//
// Because Service holds a concrete *repository.ApplicationRepository (not an
// interface), every test here uses a real PostgreSQL connection supplied by
// testhelper.NewTestPool.  The tests are skipped automatically when
// TEST_DATABASE_URL is not set so CI can run them and local development
// without a database still compiles and exits cleanly.
package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Submit ────────────────────────────────────────────────────────────────────

// TestService_Submit_HappyPath verifies that Submit creates an application row
// and returns it with the correct field values when a primary resume exists.
// A real Redis / asynq client is required; the test skips if Redis is not
// reachable.
func TestService_Submit_HappyPath(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	rc := newTestRiverClient(t, pool) // skips if River migration unavailable
	svc := newTestService(pool, rc)

	app, err := svc.Submit(ctx, f.userID, f.jobID, f.matchID)
	if err != nil {
		t.Fatalf("Submit happy path: %v", err)
	}
	if app == nil {
		t.Fatal("Submit returned nil application")
	}
	if app.UserID != f.userID {
		t.Errorf("UserID: got %s; want %s", app.UserID, f.userID)
	}
	if app.JobID != f.jobID {
		t.Errorf("JobID: got %s; want %s", app.JobID, f.jobID)
	}
	if app.ResumeID != f.resumeID {
		t.Errorf("ResumeID: got %s; want %s", app.ResumeID, f.resumeID)
	}
	if app.Status != models.StatusQueued {
		t.Errorf("Status: got %s; want %s", app.Status, models.StatusQueued)
	}
	if app.ID == uuid.Nil {
		t.Error("returned application has nil ID")
	}
}

// TestService_Submit_NoPrimaryResume verifies Submit returns a wrapped
// ErrNotFound when the user has no primary resume.
func TestService_Submit_NoPrimaryResume_ReturnsNotFound(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	rc := newTestRiverClient(t, pool) // skips if River migration unavailable

	// User with no resume at all.
	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'No Resume', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		userID,
		fmt.Sprintf("cognito-nr-%s", userID),
		fmt.Sprintf("nr-%s@example.com", userID),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	jobID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Dev', 'Co', true, NOW(), NOW(), NOW())`,
		jobID, fmt.Sprintf("ext-nr-%s", jobID),
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID) //nolint:errcheck
	})

	matchID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO matches
			(id, user_id, job_id, match_score, status, created_at, updated_at)
		VALUES ($1, $2, $3, 0.75, 'pending', NOW(), NOW())`,
		matchID, userID, jobID,
	)
	if err != nil {
		t.Fatalf("insert match: %v", err)
	}

	svc := newTestService(pool, rc)

	_, err = svc.Submit(ctx, userID, jobID, matchID)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Submit with no resume: expected ErrNotFound; got %v", err)
	}
}

// TestService_Submit_RepoCreateError triggers a DB-level error by passing a
// jobID that does not exist in the jobs table (FK violation), so that
// repo.Create returns an error which must be propagated.
func TestService_Submit_RepoCreateError(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	rc := newTestRiverClient(t, pool) // skips if River migration unavailable

	// Seed just a user + resume — but no job row, so the FK on applications.job_id
	// will fire and repo.Create will return an error.
	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Repo Error User', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		userID,
		fmt.Sprintf("cognito-rpe-%s", userID),
		fmt.Sprintf("rpe-%s@example.com", userID),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	resumeID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO resumes
			(id, user_id, file_name, s3_key, is_primary, interview_count, created_at)
		VALUES ($1, $2, 'r.pdf', $3, true, 0, NOW())`,
		resumeID, userID,
		fmt.Sprintf("resumes/%s/r.pdf", userID),
	)
	if err != nil {
		t.Fatalf("insert resume: %v", err)
	}

	// Use a random non-existent jobID and matchID — FK violation ensures error.
	nonExistentJobID := uuid.New()
	nonExistentMatchID := uuid.New()

	svc := newTestService(pool, rc)

	_, err = svc.Submit(ctx, userID, nonExistentJobID, nonExistentMatchID)
	if err == nil {
		t.Error("Submit with non-existent job FK: expected error; got nil")
	}
}

// ── ListForUser ───────────────────────────────────────────────────────────────

// TestService_ListForUser_PaginatedList verifies the returned slice and total
// count match the inserted rows.
func TestService_ListForUser_PaginatedList(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	// Insert 3 applications for this user (one per distinct job).
	statuses := []models.ApplicationStatus{
		models.StatusQueued,
		models.StatusApplied,
		models.StatusFailed,
	}

	for i, status := range statuses {
		jobID := uuid.New()
		matchID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO jobs
				(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
			VALUES ($1, $2, 'test', $3, 'ListCo', true, NOW(), NOW(), NOW())`,
			jobID, fmt.Sprintf("ext-list-%d-%s", i, jobID), fmt.Sprintf("List Job %d", i),
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
		_, err = pool.Exec(ctx, `
			INSERT INTO applications
				(id, user_id, job_id, match_id, resume_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			uuid.New(), f.userID, jobID, matchID, f.resumeID, string(status),
		)
		if err != nil {
			t.Fatalf("insert application %d: %v", i, err)
		}
	}

	apps, total, err := svc.ListForUser(ctx, f.userID, "", "", 10, 0)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if total < int64(len(statuses)) {
		t.Errorf("total: got %d; want >= %d", total, len(statuses))
	}
	if len(apps) < len(statuses) {
		t.Errorf("len(apps): got %d; want >= %d", len(apps), len(statuses))
	}

	// Verify pagination: limit=1 should return exactly 1 result but total unchanged.
	appsPage, totalPage, err := svc.ListForUser(ctx, f.userID, "", "", 1, 0)
	if err != nil {
		t.Fatalf("ListForUser page: %v", err)
	}
	if len(appsPage) != 1 {
		t.Errorf("page len: got %d; want 1", len(appsPage))
	}
	if totalPage < int64(len(statuses)) {
		t.Errorf("page total: got %d; want >= %d", totalPage, len(statuses))
	}
}

// TestService_ListForUser_StatusFilter verifies that passing a status returns
// only applications matching that status.
func TestService_ListForUser_StatusFilter(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	// Insert one queued and one applied application for this user.
	for i, status := range []models.ApplicationStatus{models.StatusQueued, models.StatusApplied} {
		jobID := uuid.New()
		matchID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO jobs
				(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
			VALUES ($1, $2, 'test', $3, 'FilterCo', true, NOW(), NOW(), NOW())`,
			jobID, fmt.Sprintf("ext-filter-%d-%s", i, jobID), fmt.Sprintf("Filter Job %d", i),
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
		_, err = pool.Exec(ctx, `
			INSERT INTO applications
				(id, user_id, job_id, match_id, resume_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			uuid.New(), f.userID, jobID, matchID, f.resumeID, string(status),
		)
		if err != nil {
			t.Fatalf("insert application %d: %v", i, err)
		}
	}

	apps, total, err := svc.ListForUser(ctx, f.userID, models.StatusQueued, "", 20, 0)
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

// TestService_ListForUser_EmptyResult verifies that a user with no applications
// gets an empty slice (not nil) and a zero total.
func TestService_ListForUser_EmptyResult(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	svc := newTestService(pool, nil)

	// Use a random UUID that has no applications.
	apps, total, err := svc.ListForUser(ctx, uuid.New(), "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser empty: %v", err)
	}
	if total != 0 {
		t.Errorf("total: got %d; want 0", total)
	}
	// The slice must not be nil — callers expect a safe range.
	if apps == nil {
		t.Error("ListForUser empty: returned nil slice; want empty (non-nil) slice")
	}
	if len(apps) != 0 {
		t.Errorf("len(apps): got %d; want 0", len(apps))
	}
}

// ── CountByStatus ─────────────────────────────────────────────────────────────

// TestService_CountByStatus_WithApps verifies the map contains correct counts
// when applications exist in multiple statuses.
func TestService_CountByStatus_WithApps(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	// Insert one queued and two applied applications.
	type entry struct {
		status models.ApplicationStatus
	}
	entries := []entry{
		{models.StatusQueued},
		{models.StatusApplied},
		{models.StatusApplied},
	}

	for i, e := range entries {
		jobID := uuid.New()
		matchID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO jobs
				(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
			VALUES ($1, $2, 'test', $3, 'StatsCo', true, NOW(), NOW(), NOW())`,
			jobID, fmt.Sprintf("ext-stats-%d-%s", i, jobID), fmt.Sprintf("Stats Job %d", i),
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
		_, err = pool.Exec(ctx, `
			INSERT INTO applications
				(id, user_id, job_id, match_id, resume_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			uuid.New(), f.userID, jobID, matchID, f.resumeID, string(e.status),
		)
		if err != nil {
			t.Fatalf("insert application %d: %v", i, err)
		}
	}

	counts, err := svc.CountByStatus(ctx, f.userID)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts == nil {
		t.Fatal("CountByStatus returned nil map")
	}
	if counts[models.StatusQueued] < 1 {
		t.Errorf("queued count: got %d; want >= 1", counts[models.StatusQueued])
	}
	if counts[models.StatusApplied] < 2 {
		t.Errorf("applied count: got %d; want >= 2", counts[models.StatusApplied])
	}
}

// TestService_CountByStatus_EmptyDB verifies that when a user has no
// applications, CountByStatus returns a non-nil (possibly empty) map with
// zero counts — never nil.
func TestService_CountByStatus_EmptyDB(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	svc := newTestService(pool, nil)

	counts, err := svc.CountByStatus(ctx, uuid.New())
	if err != nil {
		t.Fatalf("CountByStatus empty: %v", err)
	}
	if counts == nil {
		t.Fatal("CountByStatus empty: returned nil map; want non-nil empty map")
	}
	// There must be no entries for statuses that have zero apps.
	for status, count := range counts {
		if count != 0 {
			t.Errorf("unexpected non-zero count for status %s: %d", status, count)
		}
	}
}

// ── GetByID (additional edge cases beyond integration_test.go) ───────────────

// TestService_GetByID_CorrectDataRoundtrip verifies all fields survive the
// insert → GetByID roundtrip.
func TestService_GetByID_CorrectDataRoundtrip(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	appID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO applications
			(id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'queued', NOW())`,
		appID, f.userID, f.jobID, f.matchID, f.resumeID,
	)
	if err != nil {
		t.Fatalf("insert application: %v", err)
	}

	app, err := svc.GetByID(ctx, appID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if app.ID != appID {
		t.Errorf("ID mismatch: got %s; want %s", app.ID, appID)
	}
	if app.UserID != f.userID {
		t.Errorf("UserID mismatch: got %s; want %s", app.UserID, f.userID)
	}
	if app.JobID != f.jobID {
		t.Errorf("JobID mismatch: got %s; want %s", app.JobID, f.jobID)
	}
	if app.MatchID != f.matchID {
		t.Errorf("MatchID mismatch: got %s; want %s", app.MatchID, f.matchID)
	}
	if app.ResumeID != f.resumeID {
		t.Errorf("ResumeID mismatch: got %s; want %s", app.ResumeID, f.resumeID)
	}
	if app.Status != models.StatusQueued {
		t.Errorf("Status: got %s; want queued", app.Status)
	}
	if app.Outcome != nil {
		t.Errorf("Outcome: got %v; want nil for fresh application", app.Outcome)
	}
}

// ── RecordOutcome (additional cases) ─────────────────────────────────────────

// TestService_RecordOutcome_WithNotes verifies that outcome notes are persisted.
func TestService_RecordOutcome_WithNotes(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	appID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO applications
			(id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'applied', NOW())`,
		appID, f.userID, f.jobID, f.matchID, f.resumeID,
	)
	if err != nil {
		t.Fatalf("insert application: %v", err)
	}

	// OutcomeViewed does not trigger notification (nil notifier is safe).
	if err := svc.RecordOutcome(ctx, appID, f.userID, models.OutcomeViewed, "recruiter viewed profile"); err != nil {
		t.Fatalf("RecordOutcome with notes: %v", err)
	}

	// Confirm notes were persisted via a direct DB query.
	var notes string
	err = pool.QueryRow(ctx,
		`SELECT COALESCE(outcome_notes, '') FROM applications WHERE id = $1`, appID,
	).Scan(&notes)
	if err != nil {
		t.Fatalf("query outcome_notes: %v", err)
	}
	if notes != "recruiter viewed profile" {
		t.Errorf("outcome_notes: got %q; want %q", notes, "recruiter viewed profile")
	}
}

// TestService_RecordOutcome_WrongUser verifies that updating the outcome of an
// application that belongs to a different user returns ErrNotFound.
func TestService_RecordOutcome_WrongUser(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	appID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO applications
			(id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'applied', NOW())`,
		appID, f.userID, f.jobID, f.matchID, f.resumeID,
	)
	if err != nil {
		t.Fatalf("insert application: %v", err)
	}

	err = svc.RecordOutcome(ctx, appID, uuid.New() /* wrong user */, models.OutcomeRejected, "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("RecordOutcome wrong user: expected ErrNotFound; got %v", err)
	}
}
