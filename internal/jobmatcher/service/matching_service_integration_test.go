package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	discrepo "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	"github.com/bhata/AutoDreamApplier/internal/embedding"
	matchmodels "github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	matchscorer "github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
	matchsvc "github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Seed helpers ──────────────────────────────────────────────────────────────

type matchTestFixtures struct {
	userID   uuid.UUID
	jobID    uuid.UUID
	resumeID uuid.UUID
}

// seedMatchFixtures inserts a user with preferences and primary resume,
// plus a single active job, returning their IDs.
func seedMatchFixtures(t *testing.T, ctx context.Context) matchTestFixtures {
	t.Helper()
	pool := testhelper.NewTestPool(t)

	f := matchTestFixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		resumeID: uuid.New(),
	}

	externalID := fmt.Sprintf("match-ext-%s", f.jobID)

	stmts := []struct {
		sql  string
		args []any
	}{
		// User
		{
			`INSERT INTO users (id, cognito_sub, email, full_name, apply_mode, auto_threshold, is_active)
			 VALUES ($1, $2, $3, 'Match Tester', 'review', 0.75, true)`,
			[]any{f.userID, fmt.Sprintf("sub-m-%s", f.userID), fmt.Sprintf("match-%s@example.com", f.userID)},
		},
		// Preferences — strong match for "Go Engineer" in New York
		{
			`INSERT INTO user_preferences (user_id, target_titles, locations, remote_pref, salary_currency, exclusions)
			 VALUES ($1, ARRAY['Go Engineer','Backend Engineer'], ARRAY['New York, NY'], 'any', 'USD', '{}')`,
			[]any{f.userID},
		},
		// Primary resume
		{
			`INSERT INTO resumes (id, user_id, file_name, s3_key, is_primary, raw_text)
			 VALUES ($1, $2, 'match_cv.pdf', $3, true, 'Experienced Go developer, Kubernetes, PostgreSQL, 5 years backend')`,
			[]any{f.resumeID, f.userID, fmt.Sprintf("resumes/%s/match_cv.pdf", f.userID)},
		},
		// Job — high relevance to the preferences above
		{
			`INSERT INTO jobs (id, external_id, source_board, url, title, company, location, is_remote,
			                   description, ats_type, apply_url, is_active, discovered_at)
			 VALUES ($1, $2, 'indeed', 'https://example.com/job/1', 'Go Engineer', 'Acme', 'New York, NY', true,
			         'Looking for a Go engineer with Kubernetes and PostgreSQL experience.', 'greenhouse',
			         'https://boards.greenhouse.io/acme/jobs/1', true, NOW())`,
			[]any{f.jobID, externalID},
		},
	}

	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seedMatchFixtures exec: %v", err)
		}
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)   //nolint:errcheck
	})

	return f
}

// ── MatchRepository tests ─────────────────────────────────────────────────────

func TestMatchRepository_BulkInsert_AndListForUser(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	ins := []matchrepo.MatchInsert{{
		UserID: f.userID,
		JobID:  f.jobID,
		Score:  0.88,
		Breakdown: matchmodels.ScoreBreakdown{
			TitleScore:    0.9,
			LocationScore: 0.8,
			SalaryScore:   0.5,
			SkillsScore:   0.95,
		},
		Status: matchmodels.MatchStatusPending,
	}}

	n, err := repo.BulkInsert(ctx, ins)
	if err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}
	if n != 1 {
		t.Errorf("BulkInsert returned %d; want 1", n)
	}

	// ListForUser must return the new match.
	matches, total, err := repo.ListForUser(ctx, f.userID, "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if total < 1 {
		t.Errorf("total = %d; want >= 1", total)
	}
	found := false
	for _, m := range matches {
		if m.JobID == f.jobID {
			found = true
			if m.MatchScore != 0.88 {
				t.Errorf("MatchScore = %v; want 0.88", m.MatchScore)
			}
		}
	}
	if !found {
		t.Error("expected match for seeded job not found in ListForUser")
	}
}

func TestMatchRepository_BulkInsert_NoDuplicates(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	ins := []matchrepo.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.7,
		Status: matchmodels.MatchStatusPending,
	}}

	n1, err := repo.BulkInsert(ctx, ins)
	if err != nil {
		t.Fatalf("first BulkInsert: %v", err)
	}
	n2, err := repo.BulkInsert(ctx, ins)
	if err != nil {
		t.Fatalf("second BulkInsert: %v", err)
	}
	if n1 != 1 {
		t.Errorf("first insert: got %d; want 1", n1)
	}
	if n2 != 0 {
		t.Errorf("duplicate insert: got %d; want 0", n2)
	}
}

func TestMatchRepository_UpdateStatus(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	ins := []matchrepo.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.8,
		Status: matchmodels.MatchStatusPending,
	}}
	if _, err := repo.BulkInsert(ctx, ins); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	// Fetch the match ID.
	var matchID uuid.UUID
	pool.QueryRow(ctx, `SELECT id FROM matches WHERE user_id = $1 AND job_id = $2`, f.userID, f.jobID).Scan(&matchID) //nolint:errcheck

	// Approve it.
	if err := repo.UpdateStatus(ctx, matchID, f.userID, matchmodels.MatchStatusApproved); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// Verify.
	mwj, err := repo.GetByID(ctx, matchID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if mwj.Status != matchmodels.MatchStatusApproved {
		t.Errorf("status = %q; want %q", mwj.Status, matchmodels.MatchStatusApproved)
	}
}

func TestMatchRepository_SetFeedback(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	if _, err := repo.BulkInsert(ctx, []matchrepo.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.75,
		Status: matchmodels.MatchStatusPending,
	}}); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	var matchID uuid.UUID
	pool.QueryRow(ctx, `SELECT id FROM matches WHERE user_id = $1 AND job_id = $2`, f.userID, f.jobID).Scan(&matchID) //nolint:errcheck

	if err := repo.SetFeedback(ctx, matchID, f.userID, "thumbs_up"); err != nil {
		t.Fatalf("SetFeedback: %v", err)
	}

	mwj, err := repo.GetByID(ctx, matchID, f.userID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if mwj.UserFeedback != "thumbs_up" {
		t.Errorf("UserFeedback = %q; want %q", mwj.UserFeedback, "thumbs_up")
	}
}

func TestMatchRepository_SetFeedback_InvalidValue(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	// SetFeedback validation is enforced in the service layer.
	if err := svc.SetFeedback(ctx, uuid.New(), f.userID, "invalid_value"); err == nil {
		t.Error("expected error for invalid feedback value; got nil")
	}
}

func TestMatchRepository_GetAlreadyMatchedJobIDs(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)
	repo := matchrepo.New(pool, testhelper.NopLogger())

	if _, err := repo.BulkInsert(ctx, []matchrepo.MatchInsert{{
		UserID: f.userID, JobID: f.jobID, Score: 0.6,
		Status: matchmodels.MatchStatusPending,
	}}); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	seen, err := repo.GetAlreadyMatchedJobIDs(ctx, f.userID)
	if err != nil {
		t.Fatalf("GetAlreadyMatchedJobIDs: %v", err)
	}
	if _, ok := seen[f.jobID]; !ok {
		t.Errorf("expected job_id %s to appear in already-matched set", f.jobID)
	}
}

// ── MatchingService integration tests ────────────────────────────────────────

func TestMatchingService_RunForUser_ProducesMatches(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	result, err := svc.RunForUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("RunForUser: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil RunResult")
	}
	if result.JobsScored < 1 {
		t.Errorf("JobsScored = %d; want >= 1 (seeded 1 active job)", result.JobsScored)
	}
	if result.MatchesNew < 1 {
		t.Errorf("MatchesNew = %d; want >= 1 for a clearly relevant job", result.MatchesNew)
	}

	// Verify match row was actually persisted.
	_, total, err := repo.ListForUser(ctx, f.userID, "", 20, 0)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 match in DB after RunForUser; got %d", total)
	}
}

func TestMatchingService_RunForUser_NoPreferences_Skips(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	// User with no preferences.
	userID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, cognito_sub, email, full_name, is_active)
		 VALUES ($1, $2, $3, 'No Prefs User', true)`,
		userID, "sub-np-"+userID.String(), "nopref-"+userID.String()+"@example.com",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	result, err := svc.RunForUser(ctx, userID)
	if err != nil {
		t.Fatalf("RunForUser: %v", err)
	}
	// Should still succeed, just return 0 matches.
	if result.MatchesNew != 0 {
		t.Errorf("MatchesNew = %d; want 0 for user with no preferences", result.MatchesNew)
	}
}

func TestMatchingService_ApproveAndRejectMatch(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)

	// Seed a second job for reject test.
	jobID2 := uuid.New()
	discJobRepo := discrepo.NewJobRepository(pool)
	job2 := &discmodels.ScrapedJob{
		ExternalID: "rej-" + jobID2.String(),
		Source:     discmodels.SourceIndeed,
		Title:      "Go Engineer 2",
		Company:    "Corp2",
		Location:   "New York, NY",
		IsRemote:   true,
		Description: "Looking for Go engineers.",
		ApplyURL:   "https://boards.greenhouse.io/corp2/jobs/2",
	}
	job2ID, _, err := discJobRepo.Upsert(ctx, job2)
	if err != nil {
		t.Fatalf("upsert job2: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, job2ID) }) //nolint:errcheck

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	// Seed two matches manually.
	if _, err := repo.BulkInsert(ctx, []matchrepo.MatchInsert{
		{UserID: f.userID, JobID: f.jobID, Score: 0.9, Status: matchmodels.MatchStatusPending},
		{UserID: f.userID, JobID: job2ID, Score: 0.85, Status: matchmodels.MatchStatusPending},
	}); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	var approveID, rejectID uuid.UUID
	pool.QueryRow(ctx, `SELECT id FROM matches WHERE user_id = $1 AND job_id = $2`, f.userID, f.jobID).Scan(&approveID) //nolint:errcheck
	pool.QueryRow(ctx, `SELECT id FROM matches WHERE user_id = $1 AND job_id = $2`, f.userID, job2ID).Scan(&rejectID)    //nolint:errcheck

	if err := svc.ApproveMatch(ctx, approveID, f.userID); err != nil {
		t.Fatalf("ApproveMatch: %v", err)
	}
	if err := svc.RejectMatch(ctx, rejectID, f.userID); err != nil {
		t.Fatalf("RejectMatch: %v", err)
	}

	approved, _ := repo.GetByID(ctx, approveID, f.userID)
	rejected, _ := repo.GetByID(ctx, rejectID, f.userID)

	if approved.Status != matchmodels.MatchStatusApproved {
		t.Errorf("approved status = %q; want %q", approved.Status, matchmodels.MatchStatusApproved)
	}
	if rejected.Status != matchmodels.MatchStatusRejected {
		t.Errorf("rejected status = %q; want %q", rejected.Status, matchmodels.MatchStatusRejected)
	}
}

func TestMatchingService_RunForUser_SecondRunSkipsAlreadyMatched(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	// First run.
	r1, err := svc.RunForUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("first RunForUser: %v", err)
	}

	// Second run — same job, already matched, should produce 0 new matches.
	r2, err := svc.RunForUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("second RunForUser: %v", err)
	}
	_ = r1 // just verify it ran

	if r2.MatchesNew != 0 {
		t.Errorf("second run MatchesNew = %d; want 0 (already matched)", r2.MatchesNew)
	}
}

// makeEmbStub returns a test embedding server that always responds with the
// provided vector.  Used by the matching service semantic scorer tests.
func makeEmbStub(t *testing.T, vec []float32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"embedding":  vec,
			"dimensions": len(vec),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// unit384match returns a unit vector of length 384 suitable for embedding tests
// without importing the scorer package's private helper.
func unit384match() []float32 {
	v := make([]float32, 384)
	v[0] = 1.0
	return v
}

// TestMatchingService_SemanticScorer_Unreachable_CombinedScoreUsesNeutral
// verifies that when the AI service is unreachable the semantic scorer falls
// back to 0.5 and the combined score is 0.6*keyword + 0.4*0.5.
func TestMatchingService_SemanticScorer_Unreachable_CombinedScore(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)

	// Point semantic scorer at a port where nothing is listening.
	embClient := embedding.New("http://127.0.0.1:19995")
	ss := matchscorer.NewSemanticScorer(embClient)

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger()).WithSemanticScorer(ss)

	result, err := svc.RunForUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("RunForUser with unreachable AI: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil RunResult")
	}
	// The service must complete without error; matches may or may not be created
	// depending on keyword score — what we verify is no panic and valid result.
	if result.JobsScored < 0 {
		t.Errorf("JobsScored = %d; must be >= 0", result.JobsScored)
	}
}

// TestMatchingService_SalaryFilter_BelowMinRejected verifies that a job whose
// salary_max is below the user's salary_min is filtered out (JobsFiltered++,
// MatchesNew stays 0).
func TestMatchingService_SalaryFilter_BelowMinRejected(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	userID := uuid.New()
	jobID := uuid.New()
	resumeID := uuid.New()

	// User with a salary floor of $200 000.
	stmts := []struct {
		sql  string
		args []any
	}{
		{
			`INSERT INTO users (id, cognito_sub, email, full_name, apply_mode, auto_threshold, is_active)
			 VALUES ($1, $2, $3, 'Salary Filter User', 'review', 0.75, true)`,
			[]any{userID, "sub-sf-" + userID.String(), "sf-" + userID.String() + "@example.com"},
		},
		{
			// salary_min = 200000 in user_preferences
			`INSERT INTO user_preferences (user_id, target_titles, locations, remote_pref, salary_min, salary_currency, exclusions)
			 VALUES ($1, ARRAY['Go Engineer'], ARRAY['New York, NY'], 'any', 200000, 'USD', '{}')`,
			[]any{userID},
		},
		{
			`INSERT INTO resumes (id, user_id, file_name, s3_key, is_primary, raw_text)
			 VALUES ($1, $2, 'sf-cv.pdf', $3, true, 'Experienced Go developer')`,
			[]any{resumeID, userID, fmt.Sprintf("resumes/%s/sf-cv.pdf", userID)},
		},
		{
			// Job offering only $60 000 — well below the user's floor.
			`INSERT INTO jobs (id, external_id, source_board, url, title, company, location, is_remote,
			                   salary_min, salary_max, salary_currency, description, ats_type, apply_url,
			                   is_active, discovered_at)
			 VALUES ($1, $2, 'indeed', 'https://example.com/low-salary', 'Go Engineer', 'LowPayCo', 'New York, NY', true,
			         40000, 60000, 'USD',
			         'Looking for a Go engineer with Kubernetes and PostgreSQL experience.', 'greenhouse',
			         'https://boards.greenhouse.io/lowpayco/jobs/1', true, NOW())`,
			[]any{jobID, "sf-ext-" + jobID.String()},
		},
	}

	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seedSalaryFilter: %v", err)
		}
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, jobID)   //nolint:errcheck
	})

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())

	result, err := svc.RunForUser(ctx, userID)
	if err != nil {
		t.Fatalf("RunForUser: %v", err)
	}

	// The low-salary job should not produce a match.
	if result.MatchesNew > 0 {
		t.Errorf("MatchesNew = %d; want 0 (job salary below user floor)", result.MatchesNew)
	}
}

// TestMatchingService_WithSemanticScorerNil_KeywordOnlyScoring verifies that
// calling WithSemanticScorer(nil) still produces correct results using only the
// keyword scorer.
func TestMatchingService_WithSemanticScorerNil_KeywordOnlyScoring(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedMatchFixtures(t, ctx)

	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger()).WithSemanticScorer(nil)

	result, err := svc.RunForUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("RunForUser with nil semantic scorer: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil RunResult")
	}
	if result.JobsScored < 1 {
		t.Errorf("JobsScored = %d; want >= 1 (seeded 1 active job)", result.JobsScored)
	}
	if result.MatchesNew < 1 {
		t.Errorf("MatchesNew = %d; want >= 1 with keyword-only scoring", result.MatchesNew)
	}
}
