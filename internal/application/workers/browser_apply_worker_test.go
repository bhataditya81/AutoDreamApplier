package workers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/application/workers"
	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Browser pool stubs ────────────────────────────────────────────────────────

// newBrowserStub creates an httptest server returning a configured apply response.
func newBrowserStub(t *testing.T, success bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errMsg := ""
		if !success {
			errMsg = "captcha blocked"
		}
		json.NewEncoder(w).Encode(browser.ApplyResponse{ //nolint:errcheck
			Success:        success,
			ScreenshotKey:  "screenshots/test.png",
			StepsCompleted: []string{"form_filled", "submitted"},
			ErrorMessage:   errMsg,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedBrowserWorkerFixtures seeds a user, job, match, resume, and an ai_ready application.
func seedBrowserWorkerFixtures(t *testing.T, ctx context.Context) workerFixtures {
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
			`INSERT INTO jobs (id, external_id, source_board, url, title, company, ats_type, apply_url, is_active, discovered_at)
			 VALUES ($1, $2, 'test', 'https://example.com/job', 'Go Engineer', 'AcmeCorp', 'greenhouse', 'https://example.com/apply', true, NOW())`,
			[]any{f.jobID, fmt.Sprintf("ext-bw-%s", f.jobID)},
		},
		{
			`INSERT INTO users (id, cognito_sub, email, full_name, is_active)
			 VALUES ($1, $2, $3, 'Browser Worker User', true)`,
			[]any{f.userID, fmt.Sprintf("sub-bw-%s", f.userID), fmt.Sprintf("bw-%s@example.com", f.userID)},
		},
		{
			`INSERT INTO resumes (id, user_id, file_name, s3_key, is_primary, raw_text)
			 VALUES ($1, $2, 'bw-cv.pdf', $3, true, 'Go developer 5yr')`,
			[]any{f.resumeID, f.userID, fmt.Sprintf("resumes/%s/bw-cv.pdf", f.userID)},
		},
		{
			`INSERT INTO matches (id, user_id, job_id, match_score)
			 VALUES ($1, $2, $3, 0.95)`,
			[]any{f.matchID, f.userID, f.jobID},
		},
		{
			// ai_ready with S3 keys already set so the worker's S3 GetText calls hit the stub.
			`INSERT INTO applications
			   (id, user_id, job_id, match_id, resume_id, status, tailored_resume_s3, cover_letter_s3)
			 VALUES ($1, $2, $3, $4, $5, 'ai_ready', 'tailored-resumes/stub.txt', 'cover-letters/stub.txt')`,
			[]any{f.appID, f.userID, f.jobID, f.matchID, f.resumeID},
		},
	}

	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seedBrowserWorkerFixtures: %v", err)
		}
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)   //nolint:errcheck
	})

	return f
}

// makeBrowserApplyJob creates a River job struct for unit tests.
func makeBrowserApplyJob(args tasks.BrowserApplyArgs) *river.Job[tasks.BrowserApplyArgs] {
	return &river.Job[tasks.BrowserApplyArgs]{Args: args}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestBrowserApplyWorker_HappyPath(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	browserSrv := newBrowserStub(t, true)
	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New(browserSrv.URL, testhelper.NopLogger()),
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil, // notifier is nil-safe
		testhelper.NopLogger(),
	)

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID,
		UserID:        f.userID,
		JobID:         f.jobID,
	})

	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != string(models.StatusApplied) {
		t.Errorf("status = %q; want %q", status, models.StatusApplied)
	}
}

func TestBrowserApplyWorker_BrowserFailure_SetsFailedStatus(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	browserSrv := newBrowserStub(t, false) // success=false
	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New(browserSrv.URL, testhelper.NopLogger()),
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil,
		testhelper.NopLogger(),
	)

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID,
	})

	if err := w.Work(ctx, job); err == nil {
		t.Error("expected error when browser reports failure; got nil")
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status) //nolint:errcheck
	if status != string(models.StatusFailed) {
		t.Errorf("status = %q; want %q after browser failure", status, models.StatusFailed)
	}
}

// TestBrowserApplyWorker_BrowserPoolError_SetsFailedStatus verifies that an
// HTTP-level error from the browser pool marks the application as failed.
func TestBrowserApplyWorker_BrowserPoolError_SetsFailedStatus(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New("http://127.0.0.1:19997", testhelper.NopLogger()), // nothing here
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil,
		testhelper.NopLogger(),
	)

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID,
	})

	if err := w.Work(ctx, job); err == nil {
		t.Error("expected error when browser pool is unreachable; got nil")
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status) //nolint:errcheck
	if status != string(models.StatusFailed) {
		t.Errorf("status = %q; want %q when browser pool is unreachable", status, models.StatusFailed)
	}
}

// TestBrowserApplyWorker_WebhookFiredOnSuccess verifies that attaching a
// WebhookService fires an EventApplicationSubmitted webhook.
func TestBrowserApplyWorker_WebhookFiredOnSuccess(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	webhookCalled := make(chan string, 2)
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		txt, _ := body["text"].(string)
		webhookCalled <- txt
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(webhookSrv.Close)

	_, err := pool.Exec(ctx,
		`INSERT INTO user_preferences
		   (user_id, target_titles, locations, remote_pref, salary_currency, exclusions,
		    slack_webhook_url, webhook_events)
		 VALUES ($1, ARRAY['Go Engineer'], ARRAY['Remote'], 'any', 'USD', '{}',
		         $2, ARRAY['application_submitted'])
		 ON CONFLICT (user_id) DO UPDATE
		   SET slack_webhook_url = EXCLUDED.slack_webhook_url,
		       webhook_events    = EXCLUDED.webhook_events`,
		f.userID, webhookSrv.URL,
	)
	if err != nil {
		t.Fatalf("insert user_preferences: %v", err)
	}

	browserSrv := newBrowserStub(t, true)
	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())
	webhookSvc := notification.NewWebhookService(testhelper.NopLogger())

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New(browserSrv.URL, testhelper.NopLogger()),
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil,
		testhelper.NopLogger(),
	).WithWebhookService(webhookSvc, "https://app.example.com")

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID,
	})

	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work: %v", err)
	}

	select {
	case msg := <-webhookCalled:
		if !strings.Contains(msg, "Applied") {
			t.Errorf("webhook text %q does not contain 'Applied'", msg)
		}
	case <-timeoutCh(3):
		t.Error("success webhook was not fired within 3 seconds")
	}
}

// TestBrowserApplyWorker_WebhookFiredOnFailure verifies that a failure webhook
// is fired with the error message when the browser pool reports failure.
func TestBrowserApplyWorker_WebhookFiredOnFailure(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	webhookCalled := make(chan string, 2)
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		txt, _ := body["text"].(string)
		webhookCalled <- txt
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

	browserSrv := newBrowserStub(t, false)
	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())
	webhookSvc := notification.NewWebhookService(testhelper.NopLogger())

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New(browserSrv.URL, testhelper.NopLogger()),
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil,
		testhelper.NopLogger(),
	).WithWebhookService(webhookSvc, "https://app.example.com")

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID,
	})

	if err := w.Work(ctx, job); err == nil {
		t.Error("expected error from browser failure; got nil")
	}

	select {
	case msg := <-webhookCalled:
		if !strings.Contains(msg, "captcha blocked") {
			t.Errorf("webhook text %q does not contain the error message 'captcha blocked'", msg)
		}
	case <-timeoutCh(3):
		t.Error("failure webhook was not fired within 3 seconds")
	}
}

// TestBrowserApplyWorker_ContextDeadlineExceeded verifies that a cancelled
// context causes the worker to return an error.
func TestBrowserApplyWorker_ContextDeadlineExceeded(t *testing.T) {
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()
	f := seedBrowserWorkerFixtures(t, ctx)

	s3Srv := newS3Stub(t)
	reg := ats.NewRegistry(testhelper.NopLogger())

	browserSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(browser.ApplyResponse{Success: true}) //nolint:errcheck
	}))
	t.Cleanup(browserSrv.Close)

	w := workers.NewBrowserApplyWorker(
		repository.New(pool, testhelper.NopLogger()),
		browser.New(browserSrv.URL, testhelper.NopLogger()),
		newS3Client(t, s3Srv.URL),
		workers.S3Buckets{Resumes: "test-resumes", Screenshots: "test-screenshots"},
		reg,
		nil,
		testhelper.NopLogger(),
	)

	job := makeBrowserApplyJob(tasks.BrowserApplyArgs{
		ApplicationID: f.appID, UserID: f.userID, JobID: f.jobID,
	})

	deadCtx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	if err := w.Work(deadCtx, job); err == nil {
		t.Error("expected error with cancelled context; got nil")
	}

	var status string
	pool.QueryRow(ctx, `SELECT status FROM applications WHERE id = $1`, f.appID).Scan(&status) //nolint:errcheck
	if status == string(models.StatusApplied) {
		t.Errorf("status = %q; application should not be applied with cancelled context", status)
	}
}
