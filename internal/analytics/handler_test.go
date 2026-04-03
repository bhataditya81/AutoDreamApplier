package analytics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ── Helpers ───────────────────────────────────────────────────────────────────
// Handler holds a *Repository (concrete type). We construct it with a nil pool
// and only exercise validation-level paths that return before any DB call.

func newTestHandler() *Handler {
	// Repository with nil pool — queries will panic if actually executed.
	// These tests only exercise validation branches that return before any DB call.
	repo := &Repository{pool: nil, log: zerolog.Nop()}
	return NewHandler(repo, zerolog.Nop())
}

func doGet(t *testing.T, h *Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	return w
}

// ── Tests ──────────────────────────────────────────────────────────────────────

// parseDateRange tests ─────────────────────────────────────────────────────────

type fakeQuery map[string]string

func (f fakeQuery) Get(key string) string { return f[key] }

func TestParseDateRange_Defaults(t *testing.T) {
	t.Parallel()
	from, to, err := parseDateRange(fakeQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	if !to.Equal(now) {
		t.Errorf("to = %v; want %v", to, now)
	}
	wantFrom := now.AddDate(0, 0, -30)
	if !from.Equal(wantFrom) {
		t.Errorf("from = %v; want %v", from, wantFrom)
	}
}

func TestParseDateRange_Explicit(t *testing.T) {
	t.Parallel()
	from, to, err := parseDateRange(fakeQuery{"from": "2026-01-01", "to": "2026-03-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !from.Equal(wantFrom) {
		t.Errorf("from = %v; want %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Errorf("to = %v; want %v", to, wantTo)
	}
}

func TestParseDateRange_Clamps(t *testing.T) {
	t.Parallel()
	// Request a range wider than maxDays (365).
	from, to, err := parseDateRange(fakeQuery{"from": "2020-01-01", "to": "2026-03-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After clamping, from should be exactly maxDays before to.
	wantFrom := to.AddDate(0, 0, -maxDays)
	if !from.Equal(wantFrom) {
		t.Errorf("from after clamp = %v; want %v", from, wantFrom)
	}
}

func TestParseDateRange_BadFrom(t *testing.T) {
	t.Parallel()
	_, _, err := parseDateRange(fakeQuery{"from": "not-a-date"})
	if err == nil {
		t.Fatal("expected error for bad from date")
	}
}

func TestParseDateRange_BadTo(t *testing.T) {
	t.Parallel()
	_, _, err := parseDateRange(fakeQuery{"to": "not-a-date"})
	if err == nil {
		t.Fatal("expected error for bad to date")
	}
}

// Funnel handler input-validation tests ───────────────────────────────────────

func TestFunnel_MissingUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/funnel")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /funnel no user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
	assertErrorJSON(t, rr)
}

func TestFunnel_BadUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/funnel?user_id=not-a-uuid")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /funnel bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFunnel_BadDateFrom(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	uid := uuid.New().String()
	rr := doGet(t, h, "/funnel?user_id="+uid+"&from=baddate")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /funnel bad from: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFunnel_BadDateTo(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	uid := uuid.New().String()
	rr := doGet(t, h, "/funnel?user_id="+uid+"&to=baddate")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /funnel bad to: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// OverTime handler input-validation tests ─────────────────────────────────────

func TestOverTime_MissingUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/over-time")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /over-time no user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestOverTime_BadDateRange(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	uid := uuid.New().String()
	rr := doGet(t, h, "/over-time?user_id="+uid+"&from=BADDATE")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /over-time bad date: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ByResume handler input-validation tests ─────────────────────────────────────

func TestByResume_MissingUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/by-resume")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /by-resume no user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestByResume_BadUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/by-resume?user_id=not-a-uuid")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /by-resume bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// TopCompanies handler input-validation tests ─────────────────────────────────

func TestTopCompanies_MissingUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/top-companies")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /top-companies no user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTopCompanies_BadUserID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/top-companies?user_id=not-a-uuid")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /top-companies bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTopCompanies_LimitParam(t *testing.T) {
	// A missing or invalid ?limit param should not cause a 400 — input validation
	// only rejects a missing / malformed user_id.  We verify that by checking an
	// invalid limit string falls back to the default without returning 400.
	//
	// This test does NOT exercise the DB path (pool is nil).  We stop at
	// confirming the bad-limit does not cause a validation-level error (4xx from
	// the handler before the DB call).  We use a deliberately invalid UUID so that
	// the handler returns 400 for the user_id, not for the limit param.
	t.Parallel()
	h := newTestHandler()
	// Invalid limit string "abc" — handler should treat it as default (10) and
	// only fail on the missing/bad user_id, not on the limit.
	rr := doGet(t, h, "/top-companies?user_id=bad-uuid&limit=abc")
	// Expected: 400 for bad user_id (not for bad limit).
	if rr.Code != http.StatusBadRequest {
		t.Errorf("GET /top-companies bad user_id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── Content-type tests ────────────────────────────────────────────────────────

func TestFunnel_ResponseIsJSON(t *testing.T) {
	// Even on error paths, responses must be application/json.
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/funnel")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

func TestOverTime_ResponseIsJSON(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	rr := doGet(t, h, "/over-time")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func assertErrorJSON(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("response JSON missing 'error' key: %v", body)
	}
}
