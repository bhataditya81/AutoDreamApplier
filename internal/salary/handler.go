package salary

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// Handler handles salary benchmark HTTP requests.
type Handler struct {
	svc *Service
	log zerolog.Logger
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes returns a Chi sub-router wired to all salary endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/benchmark", h.GetBenchmark)
	return r
}

// GetBenchmark returns salary benchmark data for a title+location combination.
//
// GET /benchmark?title=software+engineer&location=new+york+ny
// Query params: title (required), location (required), salary_min (optional int), salary_max (optional int)
// Response 200: BenchmarkResponse JSON
// Response 400: missing title or location
// Response 404: {"benchmark": null, "market_position": "unknown"} when sample_size < 5
func (h *Handler) GetBenchmark(w http.ResponseWriter, r *http.Request) {
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

	// Parse optional salary params.
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
		h.log.Error().Err(err).Msg("GetBenchmark: service error")
		jsonError(w, http.StatusInternalServerError, "failed to get salary benchmark")
		return
	}

	// Attach optional salary params and compute market position.
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

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
