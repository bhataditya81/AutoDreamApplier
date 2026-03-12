package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bhata/AutoDreamApplier/internal/application/handler"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Fixtures ──────────────────────────────────────────────────────────────────

type handlerFixtures struct {
	userID   uuid.UUID
	jobID    uuid.UUID
	matchID  uuid.UUID
	resumeID uuid.UUID
}

// seedHandlerFixtures inserts the minimum rows required for handler tests
// (user, primary resume, job, match) and registers t.Cleanup callbacks.
//
// LIFO cleanup order:
//
//	Register job first  → runs last  (after user cascade clears applications/matches/resumes)
//	Register user second → runs first → DELETE user cascades resumes, matches, applications
func seedHandlerFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) handlerFixtures {
	t.Helper()
	f := handlerFixtures{
		userID:   uuid.New(),
		jobID:    uuid.New(),
		matchID:  uuid.New(),
		resumeID: uuid.New(),
	}

	// Register job cleanup first (LIFO → fires last).
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID)
	})

	// Insert user.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, cognito_sub, email, full_name)
		VALUES ($1, $2, $3, 'Handler Test User')
	`, f.userID, fmt.Sprintf("sub-%s", f.userID), fmt.Sprintf("handler-%s@example.com", f.userID))
	if err != nil {
		t.Fatalf("seedHandlerFixtures: insert user: %v", err)
	}
	// Register user cleanup second (LIFO → fires first, cascades resumes/matches/applications).
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID)
	})

	// Insert primary resume.
	_, err = pool.Exec(ctx, `
		INSERT INTO resumes (id, user_id, file_name, s3_key, is_primary)
		VALUES ($1, $2, 'handler-test.pdf', $3, true)
	`, f.resumeID, f.userID, fmt.Sprintf("resumes/%s/handler-test.pdf", f.userID))
	if err != nil {
		t.Fatalf("seedHandlerFixtures: insert resume: %v", err)
	}

	// Insert job.
	_, err = pool.Exec(ctx, `
		INSERT INTO jobs (id, external_id, source_board, url, title, company)
		VALUES ($1, $2, 'testboard', 'https://example.com/job', 'Handler Test Job', 'TestCo')
	`, f.jobID, fmt.Sprintf("ext-%s", f.jobID))
	if err != nil {
		t.Fatalf("seedHandlerFixtures: insert job: %v", err)
	}

	// Insert match.
	_, err = pool.Exec(ctx, `
		INSERT INTO matches (id, user_id, job_id, match_score)
		VALUES ($1, $2, $3, 0.9)
	`, f.matchID, f.userID, f.jobID)
	if err != nil {
		t.Fatalf("seedHandlerFixtures: insert match: %v", err)
	}

	return f
}

// insertApplication inserts an application row directly into the DB, bypassing
// the service layer (which requires an Asynq client). Cleanup is handled
// implicitly: deleting the parent user cascades to applications.
func insertApplication(t *testing.T, ctx context.Context, pool *pgxpool.Pool, f handlerFixtures) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO applications (id, user_id, job_id, match_id, resume_id, status)
		VALUES ($1, $2, $3, $4, $5, 'queued')
	`, id, f.userID, f.jobID, f.matchID, f.resumeID)
	if err != nil {
		t.Fatalf("insertApplication: %v", err)
	}
	return id
}

// ── Test helpers ──────────────────────────────────────────────────────────────

// newTestRouter builds a service backed by pool and returns the Chi handler.
// asynqClient, browserClient, and notifier are all nil — only DB-backed code
// paths are exercised here; Submit tests are limited to validation-only paths.
func newTestRouter(pool *pgxpool.Pool) http.Handler {
	repo := repository.New(pool, testhelper.NopLogger())
	svc := service.New(repo, nil, nil, nil, testhelper.NopLogger())
	return handler.Router(svc, testhelper.NopLogger())
}

// do fires a single HTTP request against router and returns the recorder.
// body is JSON-marshalled when non-nil.
func do(t *testing.T, router http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("do: marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func TestHandler_Stats_OK(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	f := seedHandlerFixtures(t, ctx, pool)
	insertApplication(t, ctx, pool, f)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/stats?user_id=%s", f.userID), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /stats: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode /stats response: %v", err)
	}
	if _, ok := resp["counts"]; !ok {
		t.Error("GET /stats: response missing 'counts' key")
	}
}

func TestHandler_Stats_InvalidUserID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, "/stats?user_id=not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /stats?user_id=bad: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestHandler_List_OK(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	f := seedHandlerFixtures(t, ctx, pool)
	insertApplication(t, ctx, pool, f)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/?user_id=%s", f.userID), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode GET / response: %v", err)
	}

	total, ok := resp["total"].(float64) // JSON numbers unmarshal as float64
	if !ok {
		t.Fatal("GET /: response missing 'total' field")
	}
	if int(total) < 1 {
		t.Errorf("GET /: total = %d; want >= 1", int(total))
	}

	apps, ok := resp["applications"]
	if !ok {
		t.Fatal("GET /: response missing 'applications' field")
	}
	if apps == nil {
		t.Error("GET /: 'applications' is nil")
	}
}

func TestHandler_List_InvalidUserID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, "/?user_id=bad", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /?user_id=bad: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestHandler_GetByID_OK(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	f := seedHandlerFixtures(t, ctx, pool)
	appID := insertApplication(t, ctx, pool, f)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/%s?user_id=%s", appID, f.userID), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /{appID}: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode GET /{appID} response: %v", err)
	}
	if resp["id"] != appID.String() {
		t.Errorf("GET /{appID}: id = %v; want %s", resp["id"], appID)
	}
	if resp["user_id"] != f.userID.String() {
		t.Errorf("GET /{appID}: user_id = %v; want %s", resp["user_id"], f.userID)
	}
}

func TestHandler_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/%s?user_id=%s", uuid.New(), uuid.New()), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /{unknown}: got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandler_GetByID_WrongUser(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	f := seedHandlerFixtures(t, ctx, pool)
	appID := insertApplication(t, ctx, pool, f)

	router := newTestRouter(pool)
	// Valid appID but wrong userID — repository enforces ownership.
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/%s?user_id=%s", appID, uuid.New()), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /{appID} wrong user: got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandler_GetByID_InvalidAppID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/not-a-uuid?user_id=%s", uuid.New()), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /not-a-uuid: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_GetByID_InvalidUserID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	rr := do(t, router, http.MethodGet, fmt.Sprintf("/%s?user_id=bad", uuid.New()), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /{appID}?user_id=bad: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── RecordOutcome ─────────────────────────────────────────────────────────────

func TestHandler_RecordOutcome_OK(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)
	ctx := context.Background()

	f := seedHandlerFixtures(t, ctx, pool)
	appID := insertApplication(t, ctx, pool, f)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id": f.userID.String(),
		"outcome": "viewed", // safe: does not trigger sendOutcomeNotification (nil notifier)
	}
	rr := do(t, router, http.MethodPut, fmt.Sprintf("/%s/outcome", appID), body)

	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /{appID}/outcome: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode PUT outcome response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("PUT /{appID}/outcome: status = %q; want %q", resp["status"], "ok")
	}
}

func TestHandler_RecordOutcome_InvalidOutcome(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id": uuid.New().String(),
		"outcome": "winning", // not a valid outcome
	}
	rr := do(t, router, http.MethodPut, fmt.Sprintf("/%s/outcome", uuid.New()), body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("PUT /{appID}/outcome bad outcome: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_RecordOutcome_NotFound(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id": uuid.New().String(),
		"outcome": "viewed",
	}
	rr := do(t, router, http.MethodPut, fmt.Sprintf("/%s/outcome", uuid.New()), body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("PUT /{unknown}/outcome: got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandler_RecordOutcome_InvalidAppID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id": uuid.New().String(),
		"outcome": "viewed",
	}
	rr := do(t, router, http.MethodPut, "/not-a-uuid/outcome", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("PUT /not-a-uuid/outcome: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_RecordOutcome_InvalidUserID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id": "not-a-uuid",
		"outcome": "viewed",
	}
	rr := do(t, router, http.MethodPut, fmt.Sprintf("/%s/outcome", uuid.New()), body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("PUT /{appID}/outcome bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── Submit ────────────────────────────────────────────────────────────────────
//
// Full submit (202) requires a live Asynq/Redis connection. The tests below
// cover only the validation and early-return paths that do not touch asynqClient.

func TestHandler_Submit_InvalidBody(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /submit bad JSON: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_Submit_InvalidUserID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id":  "not-a-uuid",
		"job_id":   uuid.New().String(),
		"match_id": uuid.New().String(),
	}
	rr := do(t, router, http.MethodPost, "/submit", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /submit bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_Submit_InvalidJobID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id":  uuid.New().String(),
		"job_id":   "bad",
		"match_id": uuid.New().String(),
	}
	rr := do(t, router, http.MethodPost, "/submit", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /submit bad job_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandler_Submit_InvalidMatchID(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	body := map[string]string{
		"user_id":  uuid.New().String(),
		"job_id":   uuid.New().String(),
		"match_id": "bad",
	}
	rr := do(t, router, http.MethodPost, "/submit", body)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /submit bad match_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestHandler_Submit_NoPrimaryResume verifies that a valid request body for a
// user with no primary resume in the DB yields 404. This path exits the service
// before any Asynq call, so no Redis is required.
func TestHandler_Submit_NoPrimaryResume(t *testing.T) {
	t.Parallel()
	pool := testhelper.NewTestPool(t)

	router := newTestRouter(pool)
	// All three UUIDs are random — no resume row will match → ErrNotFound.
	body := map[string]string{
		"user_id":  uuid.New().String(),
		"job_id":   uuid.New().String(),
		"match_id": uuid.New().String(),
	}
	rr := do(t, router, http.MethodPost, "/submit", body)

	if rr.Code != http.StatusNotFound {
		t.Errorf("POST /submit no resume: got %d; want %d — body: %s",
			rr.Code, http.StatusNotFound, rr.Body)
	}
}
