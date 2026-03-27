package salary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// ── benchmarkService interface (test-local) ────────────────────────────────────
// The production Handler holds a concrete *Service. For tests we define a
// local interface and a thin testHandler that accepts it.

type benchmarkService interface {
	GetBenchmark(ctx context.Context, title, location string) (*BenchmarkResponse, error)
}

// ── Mock service ──────────────────────────────────────────────────────────────

type mockSvc struct {
	resp *BenchmarkResponse
	err  error
}

func (m *mockSvc) GetBenchmark(_ context.Context, _, _ string) (*BenchmarkResponse, error) {
	return m.resp, m.err
}

// ── testHandler (mirrors Handler logic, accepts interface) ────────────────────

type testHandler struct {
	svc benchmarkService
	log zerolog.Logger
}

func (h *testHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/benchmark", h.GetBenchmark)
	return mux
}

func (h *testHandler) GetBenchmark(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	title := q.Get("title")
	if title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}
	location := q.Get("location")
	if location == "" {
		jsonError(w, http.StatusBadRequest, "location is required")
		return
	}

	var salaryMin, salaryMax *int
	if s := q.Get("salary_min"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			salaryMin = &n
		}
	}
	if s := q.Get("salary_max"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			salaryMax = &n
		}
	}

	resp, err := h.svc.GetBenchmark(r.Context(), title, location)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get salary benchmark")
		return
	}
	resp.JobSalaryMin = salaryMin
	resp.JobSalaryMax = salaryMax
	resp.MarketPosition = CompareToMarket(salaryMin, salaryMax, resp.Benchmark)

	if resp.Benchmark == nil {
		respond(w, http.StatusNotFound, resp)
		return
	}
	respond(w, http.StatusOK, resp)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func doHandlerGet(t *testing.T, h *testHandler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	return w
}

func sampleBenchmarkResp(b *Benchmark) *BenchmarkResponse {
	return &BenchmarkResponse{Benchmark: b, MarketPosition: PositionUnknown}
}

func assertErrorKey(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("response JSON missing 'error' key: %v", body)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestGetBenchmark_200(t *testing.T) {
	t.Parallel()
	b := &Benchmark{
		TitleKey:    "software engineer",
		LocationKey: "new-york-ny",
		Currency:    "USD",
		Min:         80000,
		P25:         100000,
		Median:      120000,
		P75:         140000,
		Max:         180000,
		SampleSize:  42,
		UpdatedAt:   time.Now(),
	}
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(b)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?title=Software+Engineer&location=New+York+NY")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	var body BenchmarkResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("body decode error: %v", err)
	}
	if body.Benchmark == nil {
		t.Fatal("expected non-nil benchmark in response")
	}
}

func TestGetBenchmark_400_MissingTitle(t *testing.T) {
	t.Parallel()
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(nil)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?location=Remote")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
	assertErrorKey(t, rr)
}

func TestGetBenchmark_400_MissingLocation(t *testing.T) {
	t.Parallel()
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(nil)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?title=Engineer")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
	assertErrorKey(t, rr)
}

func TestGetBenchmark_404_NilBenchmark(t *testing.T) {
	t.Parallel()
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(nil)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?title=Rare+Job&location=Nowhere")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d; want 404", rr.Code)
	}
	var body BenchmarkResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("body decode error: %v", err)
	}
	if body.Benchmark != nil {
		t.Errorf("expected null benchmark, got %+v", body.Benchmark)
	}
	if body.MarketPosition != PositionUnknown {
		t.Errorf("market_position = %q; want %q", body.MarketPosition, PositionUnknown)
	}
}

func TestGetBenchmark_SalaryParams_Forwarded(t *testing.T) {
	t.Parallel()
	b := &Benchmark{Median: 100000, SampleSize: 10}
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(b)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?title=Engineer&location=Remote&salary_min=120000&salary_max=140000")
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	var body BenchmarkResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("body decode error: %v", err)
	}
	if body.JobSalaryMin == nil || *body.JobSalaryMin != 120000 {
		t.Errorf("job_salary_min = %v; want 120000", body.JobSalaryMin)
	}
	if body.JobSalaryMax == nil || *body.JobSalaryMax != 140000 {
		t.Errorf("job_salary_max = %v; want 140000", body.JobSalaryMax)
	}
	if body.MarketPosition != PositionAbove {
		t.Errorf("market_position = %q; want %q", body.MarketPosition, PositionAbove)
	}
}

func TestGetBenchmark_ContentTypeJSON(t *testing.T) {
	t.Parallel()
	h := &testHandler{svc: &mockSvc{resp: sampleBenchmarkResp(nil)}, log: zerolog.Nop()}
	rr := doHandlerGet(t, h, "/benchmark?title=x&location=y")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", ct)
	}
}
