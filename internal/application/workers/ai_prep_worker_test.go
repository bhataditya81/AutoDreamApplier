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
	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

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
// Used to bound webhook-delivery assertions without importing testing timeouts.
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
// The AWS SDK will issue S3 API calls (PutObject, GetObject); we respond 200 for all.
func newS3Stub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "stub-content")
			return
		}
		// PutObject expects a 200 with an ETag header.
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

// skipIfNoRedis skips the test if Redis is not reachable.
func skipIfNoRedis(t *testing.T) *asynq.Client {
	t.Helper()
	addr := fmt.Sprintf("%s:%s",
		testhelper.EnvOrDefault("REDIS_HOST", "localhost"),
		testhelper.EnvOrDefault("REDIS_PORT", "6379"),
	)
	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis not reachable at %s (%v): skipping test", addr, err)
	}
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: addr})
	t.Cleanup(func() { client.Close() })
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

	// LIFO cleanup: user (cascades most rows) first, job last.
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)   //nolint:errcheck
	})

	return f
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAIPrepWorker_HappyPath(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)
	asynqClient := skipIfNoRedis(t)

	aiSrv := newAIStub(t)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		asynqClient,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	payload, err := tasks.NewAIPrep(tasks.AIPrepPayload{
		ApplicationID: f.appID,
		UserID:        f.userID,
		JobID:         f.jobID,
		ResumeID:      f.resumeID,
	})
	if err != nil {
		t.Fatalf("NewAIPrep: %v", err)
	}

	if err := w.ProcessTask(ctx, payload); err != nil {
		t.Fatalf("ProcessTask: %v", err)
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
	asynqClient := skipIfNoRedis(t)

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
		asynqClient,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	payload, _ := tasks.NewAIPrep(tasks.AIPrepPayload{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	if err := w.ProcessTask(ctx, payload); err == nil {
		t.Error("expected error from AI 500; got nil")
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status) //nolint:errcheck
	if status != string(models.StatusFailed) {
		t.Errorf("status = %q; want %q after AI error", status, models.StatusFailed)
	}
}

func TestAIPrepWorker_BadPayload_ReturnsError(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	asynqClient := skipIfNoRedis(t)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient("http://localhost"),
		newS3Client(t, s3Srv.URL),
		asynqClient,
		workers.S3Buckets{},
		testhelper.NopLogger(),
	)

	// Malformed JSON payload must return an error without panicking.
	badTask := asynq.NewTask(tasks.TypeAIPrep, []byte(`{bad json`))
	if err := w.ProcessTask(context.Background(), badTask); err == nil {
		t.Error("expected error for malformed payload; got nil")
	}
}

// TestAIPrepWorker_ContextCancelled verifies that a cancelled context causes
// ProcessTask to return an error (the DB call will fail with context.Canceled).
func TestAIPrepWorker_ContextCancelled(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)
	asynqClient := skipIfNoRedis(t)

	aiSrv := newAIStub(t)
	s3Srv := newS3Stub(t)

	w := workers.NewAIPrepWorker(
		repository.New(pool, testhelper.NopLogger()),
		aimodels.NewClient(aiSrv.URL),
		newS3Client(t, s3Srv.URL),
		asynqClient,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	payload, err := tasks.NewAIPrep(tasks.AIPrepPayload{
		ApplicationID: f.appID,
		UserID:        f.userID,
		JobID:         f.jobID,
		ResumeID:      f.resumeID,
	})
	if err != nil {
		t.Fatalf("NewAIPrep: %v", err)
	}

	// Cancel before calling ProcessTask.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := w.ProcessTask(cancelCtx, payload); err == nil {
		t.Error("expected error when context is already cancelled; got nil")
	}
}

// TestAIPrepWorker_NilWebhookService verifies the worker does not panic when
// WithWebhookService was never called (webhookSvc is nil) and the AI call fails.
func TestAIPrepWorker_NilWebhookService_NoPanic(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)
	asynqClient := skipIfNoRedis(t)

	// AI server returns 500 to trigger the failure webhook path.
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
		asynqClient,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	)

	payload, _ := tasks.NewAIPrep(tasks.AIPrepPayload{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	// Must not panic — nil webhook is handled inside sendFailureWebhook.
	err := w.ProcessTask(ctx, payload)
	if err == nil {
		t.Error("expected error from AI 500; got nil")
	}
}

// TestAIPrepWorker_WithWebhookService_FiredOnFailure verifies that attaching a
// WebhookService via WithWebhookService causes a webhook to be fired when the
// AI call fails. We use a counting test server to confirm at least one POST.
func TestAIPrepWorker_WithWebhookService_FiredOnFailure(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedWorkerFixtures(t, ctx)
	asynqClient := skipIfNoRedis(t)

	// Insert webhook preferences for the user so sendFailureWebhook can load them.
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

	// AI server returns 500 to trigger the failure path.
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
		asynqClient,
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		testhelper.NopLogger(),
	).WithWebhookService(webhookSvc)

	payload, _ := tasks.NewAIPrep(tasks.AIPrepPayload{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID, ResumeID: f.resumeID,
	})

	if err := w.ProcessTask(ctx, payload); err == nil {
		t.Error("expected error from AI 500; got nil")
	}

	// Give the goroutine launched by WebhookService.Send time to deliver.
	select {
	case <-slackCalled:
		// webhook fired as expected
	case <-timeoutCh(3):
		t.Error("webhook was not fired within 3 seconds after AI failure")
	}
}
