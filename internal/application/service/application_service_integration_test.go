package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

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
// Cleanups are registered in LIFO order so that the user row (which cascades
// to applications, matches, and resumes) is deleted before the job row.
func seedFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) fixtures {
	t.Helper()

	f := fixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		matchID:  uuid.New(),
		resumeID: uuid.New(),
	}

	// ── job (no FK deps) ──────────────────────────────────────────────────────
	_, err := pool.Exec(ctx, `
		INSERT INTO jobs
			(id, external_id, source_board, title, company, is_active, discovered_at, created_at, updated_at)
		VALUES ($1, $2, 'test', 'Go Engineer', 'Acme Corp', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedFixtures: insert job: %v", err)
	}
	// job cleanup — runs SECOND (registered first, LIFO)
	t.Cleanup(func() {
		if _, delErr := pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID); delErr != nil {
			t.Logf("seedFixtures cleanup: delete job %s: %v", f.jobID, delErr)
		}
	})

	// ── user ─────────────────────────────────────────────────────────────────
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
	// user cleanup — runs FIRST (registered second, LIFO)
	// Cascades to: resumes, matches, applications, application_events.
	t.Cleanup(func() {
		if _, delErr := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID); delErr != nil {
			t.Logf("seedFixtures cleanup: delete user %s: %v", f.userID, delErr)
		}
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
// Pass a non-nil asynq.Client only for Submit tests; all other service methods
// work with nil (the underlying repository paths don't touch the asynq client).
func newTestService(pool *pgxpool.Pool, asynqClient *asynq.Client) *service.Service {
	repo := repository.New(pool, testhelper.NopLogger())
	// browser.Client and notification.Client are passed as nil.
	// notification.Client is nil-safe; browser.Client is only called by
	// EmergencyStop, which is not exercised in these tests.
	return service.New(repo, asynqClient, nil, nil, testhelper.NopLogger())
}

// newTestAsynqClient creates an asynq.Client connected to the test Redis
// instance. The test is skipped if Redis is not reachable.
func newTestAsynqClient(t *testing.T) *asynq.Client {
	t.Helper()

	addr := fmt.Sprintf("%s:%s",
		testhelper.EnvOrDefault("REDIS_HOST", "localhost"),
		testhelper.EnvOrDefault("REDIS_PORT", "6379"),
	)

	// Ping first via go-redis so we get a clean skip instead of a hang.
	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("REDIS not reachable at %s (%v); skipping Submit integration test", addr, err)
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: addr})
	t.Cleanup(func() { client.Close() })
	return client
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestService_GetByID(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedFixtures(t, ctx, pool)

	svc := newTestService(pool, nil)

	// Pre-insert an application directly so we don't need a real asynq client.
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

	// Insert a second job + match so we can create a second application
	// (UNIQUE(user_id, job_id) prevents reusing the same job).
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

	// Insert two applications with different statuses.
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
		apps, total, err := svc.ListForUser(ctx, f.userID, "", 20, 0)
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
		apps, total, err := svc.ListForUser(ctx, f.userID, models.StatusQueued, 20, 0)
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
		apps, total, err := svc.ListForUser(ctx, uuid.New(), "", 20, 0)
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

	// Pre-insert an application.
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

	// Use OutcomeViewed — does NOT trigger sendOutcomeNotification,
	// so the nil notification client is never dereferenced.
	if err := svc.RecordOutcome(ctx, appID, f.userID, models.OutcomeViewed); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	// Confirm outcome persisted.
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

	err := svc.RecordOutcome(ctx, uuid.New(), uuid.New(), models.OutcomeRejected)
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

	// Insert one queued application.
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

	asynqClient := newTestAsynqClient(t) // skips if Redis unreachable
	svc := newTestService(pool, asynqClient)

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

	asynqClient := newTestAsynqClient(t) // skips if Redis unreachable

	// Create a user WITHOUT a primary resume.
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

	svc := newTestService(pool, asynqClient)

	_, err = svc.Submit(ctx, userID, jobID, matchID)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("expected ErrNotFound (no primary resume); got %v", err)
	}
}
