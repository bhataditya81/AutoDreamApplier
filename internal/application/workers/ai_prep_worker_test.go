package workers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	aimodels "github.com/bhata/AutoDreamApplier/internal/ai"
	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/application/workers"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
	pkgconfig "github.com/bhata/AutoDreamApplier/pkg/config"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

// timeoutCh returns a channel that receives after the given number of seconds.
func timeoutCh(seconds int) <-chan time.Time {
	return time.After(time.Duration(seconds) * time.Second)
}

// ── Stubs ─────────────────────────────────────────────────────────────────────

// newAIStub creates an httptest server that returns canned AI responses.
func newAIStub(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/resume/tailor", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(aimodels.ResumeTailorResponse{TailoredText: "tailored"}) //nolint:errcheck
	})
	mux.HandleFunc("/api/v1/cover-letter/generate", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(aimodels.CoverLetterResponse{CoverLetter: "cover letter"}) //nolint:errcheck
	})
	mux.HandleFunc("/api/v1/form-qa/answer", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(aimodels.FormQAResponse{Answer: "yes"}) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newS3Stub creates a minimal S3-compatible stub server.
func newS3Stub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "stub-content")
			return
		}
		w.Header().Set("ETag", `"stub-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newS3Client creates a pkg/s3.Client pointing at the stub server.
func newS3Client(t *testing.T, stubURL string) *pkgs3.Client {
	t.Helper()
	client, err := pkgs3.New(context.Background(), pkgconfig.S3Config{
		Endpoint:          stubURL,
		BucketResumes:     "test-resumes",
		BucketScreenshots: "test-screenshots",
	}, pkgconfig.AWSConfig{
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}, testhelper.NopLogger())
	if err != nil {
		t.Fatalf("newS3Client: %v", err)
	}
	return client
}

// ── Seed helpers ──────────────────────────────────────────────────────────────

type workerFixtures struct {
	userID   uuid.UUID
	jobID    uuid.UUID
	resumeID uuid.UUID
	matchID  uuid.UUID
	appID    uuid.UUID
}

func seedWorkerFixtures(t *testing.T, ctx context.Context) workerFixtures {
	t.Helper()
	pool := testhelper.NewTestPool(t)

	f := workerFixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		resumeID: uuid.New(),
		matchID:  uuid.New(),
		appID:    uuid.New(),
	}

	stmts := []struct {
		sql  string
		args []any
	}{
		{
			`INSERT INTO jobs (id, external_id, source_board, title, company, is_active, discovered_at)
			 VALUES ($1, $2, 'test', 'Go Engineer', 'AcmeCorp', true, NOW())`,
			[]any{f.jobID, fmt.Sprintf("ext-%s", f.jobID)},
		},
		{
			`INSERT INTO users (id, cognito_sub, email, full_name, is_active)
			 VALUES ($1, $2, $3, 'Worker User', true)`,
			[]any{f.userID, fmt.Sprintf("sub-%s", f.userID), fmt.Sprintf("worker-%s@example.com", f.userID)},
		},
		{
			`INSERT INTO resumes (id, user_id, file_name, s3_key, is_primary, raw_text)
			 VALUES ($1, $2, 'cv.pdf', $3, true, 'Go developer')`,
			[]any{f.resumeID, f.userID, fmt.Sprintf("resumes/%s/cv.pdf", f.userID)},
		},
		{
			`INSERT INTO matches (id, user_id, job_id, match_score)
			 VALUES ($1, $2, $3, 0.9)`,
			[]any{f.matchID, f.userID, f.jobID},
		},
		{
			`INSERT INTO applications (id, user_id, job_id, match_id, resume_id, status)
			 VALUES ($1, $2, $3, $4, $5, 'queued')`,
			[]any{f.appID, f.userID, f.jobID, f.matchID, f.resumeID},
		},
	}

	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seedWorkerFixtures exec: %v", err)
		}
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)   //nolint:errcheck
	})

	return f
}

// makeAIPrepJob creates a River job struct for unit tests (no actual queue involved).
func makeAIPrepJob(args tasks.AIPrepArgs) *river.Job[tasks.AIPrepArgs] {
	return &river.Job[tasks.AIPrepArgs]{Args: args}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAIPrepWorker_HappyPath(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)

	aiSrv := newAIStub(t)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		nil, // riverClient nil — Stage 2 enqueue skipped, testing Stage 1 only
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	job := makeAIPrepJob(tasks.AIPrepArgs{
		ApplicationID: f.appID,
		UserID:        f.userID,
		JobID:         f.jobID,
		ResumeID:      f.resumeID,
	})

	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work: %v", err)
	}

	// Application must transition to ai_ready after successful Stage 1.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != string(models.StatusAIReady) {
		t.Errorf("status = %q; want %q", status, models.StatusAIReady)
	}
}

func TestAIPrepWorker_AIServiceError_SetsFailedStatus(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)

	// AI server always 500s.
	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "AI down", http.StatusInternalServerError)
	}))
	t.Cleanup(aiSrv.Close)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		nil,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	job := makeAIPrepJob(tasks.AIPrepArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	if err := w.Work(ctx, job); err == nil {
		t.Error("expected error from AI 500; got nil")
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status) //nolint:errcheck
	if status != string(models.StatusFailed) {
		t.Errorf("status = %q; want %q after AI error", status, models.StatusFailed)
	}
}

// TestAIPrepWorker_ContextCancelled verifies that a cancelled context causes
// Work to return an error.
func TestAIPrepWorker_ContextCancelled(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)

	aiSrv := newAIStub(t)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		nil,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	job := makeAIPrepJob(tasks.AIPrepArgs{
		ApplicationID: f.appID,
		UserID:        f.userID,
		JobID:         f.jobID,
		ResumeID:      f.resumeID,
	})

	// Cancel before calling Work.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := w.Work(cancelCtx, job); err == nil {
		t.Error("expected error when context is already cancelled; got nil")
	}
}

// TestAIPrepWorker_NilWebhookService_NoPanic verifies the worker does not panic
// when WithWebhookService was never called and the AI call fails.
func TestAIPrepWorker_NilWebhookService_NoPanic(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)

	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "AI down", http.StatusInternalServerError)
	}))
	t.Cleanup(aiSrv.Close)
	s3Srv := newS3Stub(t)

	// No WithWebhookService call — webhookSvc stays nil.
	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		nil,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	job := makeAIPrepJob(tasks.AIPrepArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	// Must not panic.
	err := w.Work(ctx, job)
	if err == nil {
		t.Error("expected error from AI 500; got nil")
	}
}

// TestAIPrepWorker_WithWebhookService_FiredOnFailure verifies that attaching a
// WebhookService fires a webhook when the AI call fails.
func TestAIPrepWorker_WithWebhookService_FiredOnFailure(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)

	slackCalled := make(chan struct{}, 1)
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slackCalled <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(webhookSrv.Close)

	_, err := pool.Exec(ctx,
		`INSERT INTO user_preferences
		   (user_id, target_titles, locations, remote_pref, salary_currency, exclusions,
		    slack_webhook_url, webhook_events)
		 VALUES ($1, ARRAY['Go Engineer'], ARRAY['Remote'], 'any', 'USD', '{}',
		         $2, ARRAY['application_failed'])
		 ON CONFLICT (user_id) DO UPDATE
		   SET slack_webhook_url = EXCLUDED.slack_webhook_url,
		       webhook_events    = EXCLUDED.webhook_events`,
		f.userID, webhookSrv.URL,
	)
	if err != nil {
		t.Fatalf("insert user_preferences: %v", err)
	}

	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "AI down", http.StatusInternalServerError)
	}))
	t.Cleanup(aiSrv.Close)
	s3Srv := newS3Stub(t)

	webhookSvc := notification.NewWebhookService(testhelper.NopLogger())
	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		nil,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	).WithWebhookService(webhookSvc)

	job := makeAIPrepJob(tasks.AIPrepArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	if err := w.Work(ctx, job); err == nil {
		t.Error("expected error from AI 500; got nil")
	}

	select {
	case <-slackCalled:
		// webhook fired as expected
	case <-timeoutCh(3):
		t.Error("webhook was not fired within 3 seconds after AI failure")
	}
}
