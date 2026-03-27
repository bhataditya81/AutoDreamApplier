package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── fixtures ──────────────────────────────────────────────────────────────────

type fixtures struct {
	userID   uuid.UUID
	jobID    uuid.UUID
	matchID  uuid.UUID
	resumeID uuid.UUID
}

// seedFixtures inserts a user, primary resume, job, and match into the test DB.
func seedFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) fixtures {
	t.Helper()

	f := fixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		matchID:  uuid.New(),
		resumeID: uuid.New(),
	}

	// ── job ───────────────────────────────────────────────────────────────────
	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Go Engineer', 'Acme Corp', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedFixtures: insert job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID) //nolint:errcheck
	})

	// ── user ──────────────────────────────────────────────────────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Test User', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		f.userID,
		fmt.Sprintf("cognito-%s", f.userID),
		fmt.Sprintf("test-%s@example.com", f.userID),
	)
	if err != nil {
		t.Fatalf("seedFixtures: insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
	})

	// ── primary resume ────────────────────────────────────────────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO resumes
			(id, user_id, file_name, s3_key, is_primary, interview_count, created_at)
		VALUES ($1, $2, 'resume.pdf', $3, true, 0, NOW())`,
		f.resumeID, f.userID,
		fmt.Sprintf("resumes/%s/resume.pdf", f.userID),
	)
	if err != nil {
		t.Fatalf("seedFixtures: insert resume: %v", err)
	}

	// ── match ─────────────────────────────────────────────────────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO matches
			(id, user_id, job_id, match_score, status, created_at, updated_at)
		VALUES ($1, $2, $3, 0.9, 'pending', NOW(), NOW())`,
		f.matchID, f.userID, f.jobID,
	)
	if err != nil {
		t.Fatalf("seedFixtures: insert match: %v", err)
	}

	return f
}

// newTestService builds a *service.Service backed by a real repository.
// Pass a non-nil River client only for Submit tests; all other methods work
// with nil since they don't touch the job queue.
func newTestService(pool *pgxpool.Pool, rc *river.Client[pgx.Tx]) *service.Service {
	repo := repository.New(pool, testhelper.NopLogger())
	return service.New(repo, rc, nil, nil, testhelper.NopLogger())
}

// newTestRiverClient creates a River client backed by the test PostgreSQL pool.
// River migrations are applied idempotently so the job tables exist.
// The test is skipped if River table creation fails unexpectedly.
func newTestRiverClient(t *testing.T, pool *pgxpool.Pool) *river.Client[pgx.Tx] {
	t.Helper()
	ctx := context.Background()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Skipf("newTestRiverClient: create migrator: %v", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Skipf("newTestRiverClient: migrate: %v", err)
	}

	rc, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatalf("newTestRiverClient: %v", err)
	}
	return rc
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestService_GetByID(t *testing.T) {
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

	t.Run("found", func(t *testing.T) {
		app, err := svc.GetByID(ctx, appID, f.userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if app.ID != appID {
			t.Errorf("ID: got %s; want %s", app.ID, appID)
		}
		if app.UserID != f.userID {
			t.Errorf("UserID: got %s; want %s", app.UserID, f.userID)
		}
		if app.Status != models.StatusQueued {
			t.Errorf("Status: got %s; want %s", app.Status, models.StatusQueued)
		}
	})

	t.Run("wrong_app_id", func(t *testing.T) {
		_, err := svc.GetByID(ctx, uuid.New(), f.userID)
		if !errors.Is(err, service.ErrNotFound) {
			t.Errorf("expected ErrNotFound; got %v", err)
		}
	})

	t.Run("wrong_user_id", func(t *testing.T) {
		_, err := svc.GetByID(ctx, appID, uuid.New())
		if !errors.Is(err, service.ErrNotFound) {
			t.Errorf("expected ErrNotFound for wrong user; got %v", err)
		}
	})
}

// ── ListForUser ───────────────────────────────────────────────────────────────

func TestService_ListForUser(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	job2ID := uuid.New()
	match2ID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Senior Go Engineer', 'Acme Corp', true, NOW(), NOW(), NOW())`,
		job2ID, fmt.Sprintf("ext2-%s", job2ID),
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
		VALUES ($1, $2, $3, 0.85, 'pending', NOW(), NOW())`,
		match2ID, f.userID, job2ID,
	)
	if err != nil {
		t.Fatalf("insert match2: %v", err)
	}

	app1ID := uuid.New()
	app2ID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO applications (id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1,$2,$3,$4,$5,'queued',NOW()),
		       ($6,$2,$7,$8,$5,'applied',NOW())`,
		app1ID, f.userID, f.jobID, f.matchID, f.resumeID,
		app2ID, job2ID, match2ID,
	)
	if err != nil {
		t.Fatalf("insert applications: %v", err)
	}
	_ = app1ID
	_ = app2ID

	t.Run("all_statuses", func(t *testing.T) {
		apps, total, err := svc.ListForUser(ctx, f.userID, "", "", 20, 0)
		if err != nil {
			t.Fatalf("ListForUser: %v", err)
		}
		if total < 2 {
			t.Errorf("total: got %d; want >= 2", total)
		}
		if len(apps) < 2 {
			t.Errorf("len(apps): got %d; want >= 2", len(apps))
		}
	})

	t.Run("filter_queued", func(t *testing.T) {
		apps, total, err := svc.ListForUser(ctx, f.userID, models.StatusQueued, "", 20, 0)
		if err != nil {
			t.Fatalf("ListForUser filtered: %v", err)
		}
		if total < 1 {
			t.Errorf("total filtered: got %d; want >= 1", total)
		}
		for _, a := range apps {
			if a.Status != models.StatusQueued {
				t.Errorf("unexpected status %s in filtered list", a.Status)
			}
		}
	})

	t.Run("no_results_for_unknown_user", func(t *testing.T) {
		apps, total, err := svc.ListForUser(ctx, uuid.New(), "", "", 20, 0)
		if err != nil {
			t.Fatalf("ListForUser unknown user: %v", err)
		}
		if total != 0 {
			t.Errorf("total: got %d; want 0", total)
		}
		if len(apps) != 0 {
			t.Errorf("len(apps): got %d; want 0", len(apps))
		}
	})
}

// ── RecordOutcome ─────────────────────────────────────────────────────────────

func TestService_RecordOutcome(t *testing.T) {
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

	// OutcomeViewed — does NOT trigger sendOutcomeNotification.
	if err := svc.RecordOutcome(ctx, appID, f.userID, models.OutcomeViewed, ""); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	app, err := svc.GetByID(ctx, appID, f.userID)
	if err != nil {
		t.Fatalf("GetByID after RecordOutcome: %v", err)
	}
	if app.Outcome == nil || *app.Outcome != models.OutcomeViewed {
		t.Errorf("Outcome: got %v; want %s", app.Outcome, models.OutcomeViewed)
	}
}

func TestService_RecordOutcome_NotFound(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	svc := newTestService(pool, nil)

	err := svc.RecordOutcome(ctx, uuid.New(), uuid.New(), models.OutcomeRejected, "")
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("expected ErrNotFound; got %v", err)
	}
}

// ── CountByStatus ─────────────────────────────────────────────────────────────

func TestService_CountByStatus(t *testing.T) {
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

	counts, err := svc.CountByStatus(ctx, f.userID)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts[models.StatusQueued] < 1 {
		t.Errorf("queued count: got %d; want >= 1", counts[models.StatusQueued])
	}
}

// ── Submit ────────────────────────────────────────────────────────────────────

func TestService_Submit(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	rc := newTestRiverClient(t, pool) // skips if River migration unavailable
	svc := newTestService(pool, rc)

	app, err := svc.Submit(ctx, f.userID, f.jobID, f.matchID)
	if err != nil {
		t.Fatalf("Submit: %v", err)
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
}

func TestService_Submit_NoPrimaryResume(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	rc := newTestRiverClient(t, pool)

	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users
			(id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'No Resume User', 'free', 'review', 0.8, 5, true, NOW(), NOW())`,
		userID,
		fmt.Sprintf("cognito-noresume-%s", userID),
		fmt.Sprintf("noresume-%s@example.com", userID),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	jobID := uuid.New()
	matchID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Go Dev', 'No Corp', true, NOW(), NOW(), NOW())`,
		jobID, fmt.Sprintf("ext-noresume-%s", jobID),
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID) //nolint:errcheck
	})

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
		t.Errorf("expected ErrNotFound (no primary resume); got %v", err)
	}
}
