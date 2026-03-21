package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/handler"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/service"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
)

// ── Router builder ────────────────────────────────────────────────────────────

func newDiscoveryRouter(t *testing.T) http.Handler {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	repo := repository.NewJobRepository(pool)
	svc := service.NewDiscoveryService(repo, testhelper.NopLogger())
	h := handler.New(svc, testhelper.NopLogger())
	return h.Router()
}

// doDiscovery fires a request against the discovery handler.
func doDiscovery(t *testing.T, router http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("doDiscovery: marshal: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ── GET /sources ──────────────────────────────────────────────────────────────

func TestDiscoveryHandler_ListSources_OK(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	rr := doDiscovery(t, router, http.MethodGet, "/sources", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /sources: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sources, ok := resp["sources"]
	if !ok {
		t.Fatal("GET /sources: missing 'sources' key")
	}
	list, _ := sources.([]any)
	if len(list) == 0 {
		t.Error("GET /sources: expected at least one source")
	}
}

// ── GET /stats ────────────────────────────────────────────────────────────────

func TestDiscoveryHandler_Stats_OK(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	rr := doDiscovery(t, router, http.MethodGet, "/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /stats: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["stats"]; !ok {
		t.Error("GET /stats: missing 'stats' key")
	}
}

// ── POST /discover ────────────────────────────────────────────────────────────

func TestDiscoveryHandler_Discover_NoKeywords_BadRequest(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	body := map[string]any{
		"keywords": []string{},
		"location": "Remote",
	}
	rr := doDiscovery(t, router, http.MethodPost, "/discover", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /discover (no keywords): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDiscoveryHandler_Discover_InvalidBody_BadRequest(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/discover", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /discover (bad JSON): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDiscoveryHandler_Discover_WithKeywords_OK(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)
	ctx := context.Background()
	_ = ctx

	// The scrapers are stubs (Glassdoor, ZipRecruiter are stub implementations)
	// so this exercises the service/handler path without real HTTP scraping.
	body := map[string]any{
		"keywords":  []string{"golang"},
		"location":  "Remote",
		"max_pages": 1,
	}
	rr := doDiscovery(t, router, http.MethodPost, "/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /discover (valid): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["results"]; !ok {
		t.Error("POST /discover: missing 'results' key")
	}
}

func TestDiscoveryHandler_Discover_SingleSource_OK(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	body := map[string]any{
		"keywords":  []string{"engineer"},
		"source":    "glassdoor", // stubbed scraper
		"max_pages": 1,
	}
	rr := doDiscovery(t, router, http.MethodPost, "/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /discover (single source): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestDiscoveryHandler_Discover_DefaultMaxPages(t *testing.T) {
	t.Parallel()
	router := newDiscoveryRouter(t)

	// max_pages not set → defaults to 5 in handler
	body := map[string]any{
		"keywords": []string{"backend"},
	}
	rr := doDiscovery(t, router, http.MethodPost, "/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /discover (default pages): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}
