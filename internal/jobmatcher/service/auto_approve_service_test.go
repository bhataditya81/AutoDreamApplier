//go:build auto_approve
// +build auto_approve

// This file requires auto_approve_service.go to be present in this package.
// Run with: go test -tags auto_approve ./internal/jobmatcher/service/...
package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	matchsvc "github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
	userrepo "github.com/bhata/AutoDreamApplier/internal/user/repository"
)

// ── Seed helpers ──────────────────────────────────────────────────────────────

type autoApproveFixtures struct {
	userID  uuid.UUID
	jobID   uuid.UUID
	matchID uuid.UUID
}

// seedAutoApproveFixtures creates an active user with auto_apply_enabled=true,
// a job, and a pending match at the given score.
func seedAutoApproveFixtures(t *testing.T, ctx context.Context, score float64) autoApproveFixtures {
	t.Helper()
	pool := testhelper.NewTestPool(t)

	f := autoApproveFixtures{
		userID:  uuid.New(),
		jobID:   uuid.New(),
		matchID: uuid.New(),
	}

	// Job
	_, err := pool.Exec(ctx,
		`INSERT INTO jobs (id, external_id, source_board, title, company, url, is_active, discovered_at, created_at, updated_at)
		 VALUES ($1, $2, 'testboard', 'AA Test Job', 'TestCo', 'https://example.com/job', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("aa-ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedAutoApproveFixtures: job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID) //nolint:errcheck
	})

	// User with auto_threshold = 0.75
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, cognito_sub, email, full_name, auto_threshold, is_active)
		 VALUES ($1, $2, $3, 'AA Test User', 0.75, true)`,
		f.userID,
		fmt.Sprintf("aa-sub-%s", f.userID),
		fmt.Sprintf("auto-approve-%s@example.com", f.userID),
	)
	if err != nil {
		t.Fatalf("seedAutoApproveFixtures: user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
	})

	// User preferences with auto_apply_enabled = true
	_, err = pool.Exec(ctx,
		`INSERT INTO user_preferences (user_id, target_titles, remote_pref, auto_apply_enabled)
		 VALUES ($1, ARRAY['Engineer'], 'remote', true)`,
		f.userID,
	)
	if err != nil {
		t.Fatalf("seedAutoApproveFixtures: preferences: %v", err)
	}

	// Pending match at given score
	_, err = pool.Exec(ctx,
		`INSERT INTO matches (id, user_id, job_id, match_score, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'pending', NOW(), NOW())`,
		f.matchID, f.userID, f.jobID, score,
	)
	if err != nil {
		t.Fatalf("seedAutoApproveFixtures: match: %v", err)
	}

	return f
}

func newAutoApproveService(t *testing.T) *matchsvc.AutoApproveService {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	mr := matchrepo.New(pool, testhelper.NopLogger())
	ur := userrepo.NewUserRepository(pool, testhelper.NopLogger())
	return matchsvc.NewAutoApproveService(mr, ur, testhelper.NopLogger())
}

// ── ProcessPendingMatches ─────────────────────────────────────────────────────

func TestAutoApproveService_ProcessPending_AboveThreshold_Approved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testhelper.NewTestPool(t)

	// score 0.80 >= threshold 0.75 → should be approved
	f := seedAutoApproveFixtures(t, ctx, 0.80)
	svc := newAutoApproveService(t)

	if err := svc.ProcessPendingMatches(ctx, f.userID, 0.75); err != nil {
		t.Fatalf("ProcessPendingMatches: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM matches WHERE id = $1`, f.matchID,
	).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "approved" {
		t.Errorf("ProcessPendingMatches (above threshold): status = %q; want approved", status)
	}
}

func TestAutoApproveService_ProcessPending_BelowThreshold_StaysPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testhelper.NewTestPool(t)

	// score 0.60 < threshold 0.75 → should stay pending
	f := seedAutoApproveFixtures(t, ctx, 0.60)
	svc := newAutoApproveService(t)

	if err := svc.ProcessPendingMatches(ctx, f.userID, 0.75); err != nil {
		t.Fatalf("ProcessPendingMatches: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM matches WHERE id = $1`, f.matchID,
	).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "pending" {
		t.Errorf("ProcessPendingMatches (below threshold): status = %q; want pending", status)
	}
}

func TestAutoApproveService_ProcessPending_ExactThreshold_Approved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testhelper.NewTestPool(t)

	// score == threshold → should be approved (>= check)
	f := seedAutoApproveFixtures(t, ctx, 0.75)
	svc := newAutoApproveService(t)

	if err := svc.ProcessPendingMatches(ctx, f.userID, 0.75); err != nil {
		t.Fatalf("ProcessPendingMatches: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM matches WHERE id = $1`, f.matchID,
	).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "approved" {
		t.Errorf("ProcessPendingMatches (exact threshold): status = %q; want approved", status)
	}
}

func TestAutoApproveService_ProcessPending_NoMatches_NoError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := newAutoApproveService(t)
	// User with no matches → should succeed silently
	if err := svc.ProcessPendingMatches(ctx, uuid.New(), 0.75); err != nil {
		t.Errorf("ProcessPendingMatches (no matches): unexpected error: %v", err)
	}
}

// ── RunForAllUsers ────────────────────────────────────────────────────────────

func TestAutoApproveService_RunForAllUsers_ProcessesAutoApplyUsers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testhelper.NewTestPool(t)

	// Seed two users: both auto-apply enabled, one with a high-score pending match
	f1 := seedAutoApproveFixtures(t, ctx, 0.90) // above threshold → approved
	f2 := seedAutoApproveFixtures(t, ctx, 0.50) // below threshold → stays pending

	svc := newAutoApproveService(t)

	if err := svc.RunForAllUsers(ctx); err != nil {
		t.Fatalf("RunForAllUsers: %v", err)
	}

	var status1, status2 string
	pool.QueryRow(ctx, `SELECT status FROM matches WHERE id = $1`, f1.matchID).Scan(&status1) //nolint:errcheck
	pool.QueryRow(ctx, `SELECT status FROM matches WHERE id = $1`, f2.matchID).Scan(&status2) //nolint:errcheck

	if status1 != "approved" {
		t.Errorf("RunForAllUsers: f1 match status = %q; want approved (score 0.90 >= threshold 0.75)", status1)
	}
	if status2 != "pending" {
		t.Errorf("RunForAllUsers: f2 match status = %q; want pending (score 0.50 < threshold 0.75)", status2)
	}
}

func TestAutoApproveService_RunForAllUsers_NoAutoApplyUsers_NoError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc := newAutoApproveService(t)
	// With a clean DB slice (or users without auto_apply_enabled), should not error
	if err := svc.RunForAllUsers(ctx); err != nil {
		t.Errorf("RunForAllUsers (empty): unexpected error: %v", err)
	}
}
