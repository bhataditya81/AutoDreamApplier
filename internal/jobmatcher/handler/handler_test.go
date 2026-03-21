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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/handler"
	matchmodels "github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	matchsvc "github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Fixtures ──────────────────────────────────────────────────────────────────

type matchHandlerFixtures struct {
	userID  uuid.UUID
	jobID   uuid.UUID
	matchID uuid.UUID
	sub     string
}

func seedMatchHandlerFixtures(t *testing.T, ctx context.Context) matchHandlerFixtures {
	t.Helper()
	pool := testhelper.NewTestPool(t)

	f := matchHandlerFixtures{
		userID:  uuid.New(),
		jobID:   uuid.New(),
		matchID: uuid.New(),
		sub:     fmt.Sprintf("sub-%s", uuid.New()),
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO jobs (id, external_id, source_board, title, company, url, is_active, discovered_at, created_at, updated_at)
		 VALUES ($1, $2, 'testboard', 'Handler Match Job', 'TestCo', 'https://example.com/job', true, NOW(), NOW(), NOW())`,
		f.jobID, fmt.Sprintf("ext-%s", f.jobID),
	)
	if err != nil {
		t.Fatalf("seedMatchHandlerFixtures: job: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, f.jobID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, cognito_sub, email, full_name, is_active)
		 VALUES ($1, $2, $3, 'Match Handler User', true)`,
		f.userID, f.sub, fmt.Sprintf("matchhandler-%s@example.com", f.userID),
	)
	if err != nil {
		t.Fatalf("seedMatchHandlerFixtures: user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, f.userID) //nolint:errcheck
	})

	_, err = pool.Exec(ctx,
		`INSERT INTO matches (id, user_id, job_id, match_score, status)
		 VALUES ($1, $2, $3, 0.9, 'pending')`,
		f.matchID, f.userID, f.jobID,
	)
	if err != nil {
		t.Fatalf("seedMatchHandlerFixtures: match: %v", err)
	}

	return f
}

// ── Router builder ────────────────────────────────────────────────────────────

func newMatchRouter(t *testing.T) http.Handler {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	repo := matchrepo.New(pool, testhelper.NopLogger())
	svc := matchsvc.New(pool, repo, testhelper.NopLogger())
	h := handler.New(svc, repo, testhelper.NopLogger())

	r := chi.NewRouter()
	h.Routes(r)
	return r
}

// withAuthClaims injects fake JWT claims into the request context.
func withAuthClaims(r *http.Request, sub string) *http.Request {
	claims := &auth.Claims{}
	claims.Sub = sub
	ctx := context.WithValue(r.Context(), auth.UserContextKey, claims)
	return r.WithContext(ctx)
}

// doMatch fires a request against the match router, optionally injecting auth.
func doMatch(t *testing.T, router http.Handler, method, target, sub string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("doMatch: marshal: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sub != "" {
		req = withAuthClaims(req, sub)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ── GET /user/{userID} ────────────────────────────────────────────────────────

func TestMatchHandler_ListMatches_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet,
		fmt.Sprintf("/user/%s", f.userID), "", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /user/{userID}: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["meta"] == nil {
		t.Error("GET /user/{userID}: missing 'meta' field")
	}
}

func TestMatchHandler_ListMatches_InvalidUserID(t *testing.T) {
	t.Parallel()
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet, "/user/not-a-uuid", "", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /user/bad: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMatchHandler_ListMatches_StatusFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet,
		fmt.Sprintf("/user/%s?status=pending", f.userID), "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /user/{id}?status=pending: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

// ── GET /{matchID}/user/{userID} ──────────────────────────────────────────────

func TestMatchHandler_GetMatch_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet,
		fmt.Sprintf("/%s/user/%s", f.matchID, f.userID), "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /{matchID}/user/{userID}: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestMatchHandler_GetMatch_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet,
		fmt.Sprintf("/%s/user/%s", uuid.New(), f.userID), "", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /{unknown}/user/{userID}: got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

// ── POST /{matchID}/approve ───────────────────────────────────────────────────

func TestMatchHandler_ApproveMatch_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"user_id": f.userID.String()}
	rr := doMatch(t, router, http.MethodPost,
		fmt.Sprintf("/%s/approve", f.matchID), "", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /{matchID}/approve: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["status"] != "queued" {
		t.Errorf("approve: status = %v; want queued", data["status"])
	}
}

func TestMatchHandler_ApproveMatch_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"user_id": f.userID.String()}
	rr := doMatch(t, router, http.MethodPost,
		fmt.Sprintf("/%s/approve", uuid.New()), "", body)
	if rr.Code != http.StatusNotFound {
		t.Errorf("approve (not found): got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

// ── POST /{matchID}/reject ────────────────────────────────────────────────────

func TestMatchHandler_RejectMatch_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"user_id": f.userID.String()}
	rr := doMatch(t, router, http.MethodPost,
		fmt.Sprintf("/%s/reject", f.matchID), "", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /{matchID}/reject: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["status"] != "skipped" {
		t.Errorf("reject: status = %v; want skipped", data["status"])
	}
}

// ── POST /{matchID}/feedback ──────────────────────────────────────────────────

func TestMatchHandler_SetFeedback_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{
		"user_id":  f.userID.String(),
		"feedback": "thumbs_up",
	}
	rr := doMatch(t, router, http.MethodPost,
		fmt.Sprintf("/%s/feedback", f.matchID), "", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /{matchID}/feedback: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestMatchHandler_SetFeedback_InvalidValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{
		"user_id":  f.userID.String(),
		"feedback": "meh",
	}
	rr := doMatch(t, router, http.MethodPost,
		fmt.Sprintf("/%s/feedback", f.matchID), "", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("feedback (bad value): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── PATCH /{matchID} (JWT-aware UpdateMatchMe) ────────────────────────────────

func TestMatchHandler_UpdateMatchMe_Approve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"status": "approved"}
	rr := doMatch(t, router, http.MethodPatch,
		fmt.Sprintf("/%s", f.matchID), f.sub, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH /{matchID} (approve): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestMatchHandler_UpdateMatchMe_Reject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"status": "rejected"}
	rr := doMatch(t, router, http.MethodPatch,
		fmt.Sprintf("/%s", f.matchID), f.sub, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH /{matchID} (reject): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestMatchHandler_UpdateMatchMe_InvalidStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"status": "maybe"}
	rr := doMatch(t, router, http.MethodPatch,
		fmt.Sprintf("/%s", f.matchID), f.sub, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PATCH /{matchID} (bad status): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMatchHandler_UpdateMatchMe_Unauthenticated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"status": "approved"}
	rr := doMatch(t, router, http.MethodPatch,
		fmt.Sprintf("/%s", f.matchID), "" /* no sub */, body)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("PATCH (no auth): got %d; want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ── POST /bulk-action ─────────────────────────────────────────────────────────

func TestMatchHandler_BulkAction_Approve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]any{
		"action":    "approve",
		"match_ids": []string{f.matchID.String()},
	}
	rr := doMatch(t, router, http.MethodPost, "/bulk-action", f.sub, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /bulk-action (approve): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["action"] != "approve" {
		t.Errorf("BulkAction: action = %v; want approve", data["action"])
	}
}

func TestMatchHandler_BulkAction_EmptyMatchIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]any{
		"action":    "approve",
		"match_ids": []string{},
	}
	rr := doMatch(t, router, http.MethodPost, "/bulk-action", f.sub, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("BulkAction (empty ids): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMatchHandler_BulkAction_InvalidAction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]any{
		"action":    "delete",
		"match_ids": []string{f.matchID.String()},
	}
	rr := doMatch(t, router, http.MethodPost, "/bulk-action", f.sub, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("BulkAction (bad action): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── PATCH /{matchID}/feedback (JWT-aware FeedbackMe) ─────────────────────────

func TestMatchHandler_FeedbackMe_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	body := map[string]string{"feedback": "thumbs_down"}
	rr := doMatch(t, router, http.MethodPatch,
		fmt.Sprintf("/%s/feedback", f.matchID), f.sub, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH /{matchID}/feedback: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

// ── GET / (ListMatchesMe) ─────────────────────────────────────────────────────

func TestMatchHandler_ListMatchesMe_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet, "/", f.sub, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / (ListMatchesMe): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestMatchHandler_ListMatchesMe_StatusFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := seedMatchHandlerFixtures(t, ctx)
	router := newMatchRouter(t)

	rr := doMatch(t, router, http.MethodGet, "/?status=pending", f.sub, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /?status=pending: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["meta"] == nil {
		t.Error("GET /?status=pending: missing 'meta'")
	}
}

// suppress unused import warning
var _ = matchmodels.MatchStatusPending
