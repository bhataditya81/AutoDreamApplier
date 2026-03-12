// Package handler provides the HTTP layer for the job-discovery service.
// It exposes three endpoints:
//
//	POST /discover       – trigger a scraping run (all boards or a single board)
//	GET  /sources        – list registered job-board scrapers
//	GET  /stats          – return per-source job counts
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/service"
)

// Handler is the HTTP handler for job discovery endpoints.
type Handler struct {
	svc *service.DiscoveryService
	log zerolog.Logger
}

// New creates a new discovery Handler.
func New(svc *service.DiscoveryService, log zerolog.Logger) *Handler {
	return &Handler{
		svc: svc,
		log: log.With().Str("handler", "discovery").Logger(),
	}
}

// Router returns a chi.Router with all discovery routes mounted.
// Intended to be passed to r.Mount("/api/v1/jobs", handler.Router()).
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/discover", h.Discover)
	r.Get("/sources", h.ListSources)
	r.Get("/stats", h.Stats)
	return r
}

// ── Request / response types ──────────────────────────────────────────────────

// discoverRequest is the JSON body accepted by POST /discover.
type discoverRequest struct {
	Keywords  []string `json:"keywords"`
	Location  string   `json:"location"`
	Remote    bool     `json:"remote"`
	MaxPages  int      `json:"max_pages"`
	SalaryMin int      `json:"salary_min"`
	// Source is optional. When set, only that board is scraped.
	// Accepted values: "indeed", "glassdoor". Omit to scrape all boards.
	Source string `json:"source,omitempty"`
}

// discoverResult is the per-scraper result included in the POST /discover response.
type discoverResult struct {
	Source     string `json:"source"`
	JobsFound  int    `json:"jobs_found"`
	JobsNew    int    `json:"jobs_new"`
	JobsDupe   int    `json:"jobs_dupe"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// sourceResponse is one entry in the GET /sources response.
type sourceResponse struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// Discover triggers a discovery run against one or all registered job boards.
//
//	POST /api/v1/jobs/discover
//	Body: { "keywords": ["golang", "backend"], "location": "Remote", "max_pages": 5 }
func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	var req discoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Keywords) == 0 {
		writeError(w, http.StatusBadRequest, "at least one keyword is required")
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 5 // sensible default
	}

	params := service.DiscoverParams{
		Keywords:  req.Keywords,
		Location:  req.Location,
		Remote:    req.Remote,
		MaxPages:  req.MaxPages,
		SalaryMin: req.SalaryMin,
	}

	var raw []service.RunResult
	if req.Source != "" {
		raw = []service.RunResult{
			h.svc.RunSingle(r.Context(), models.JobSource(req.Source), params),
		}
	} else {
		raw = h.svc.RunAll(r.Context(), params)
	}

	results := make([]discoverResult, 0, len(raw))
	for _, rr := range raw {
		dr := discoverResult{
			Source:     string(rr.Source),
			JobsFound:  rr.JobsFound,
			JobsNew:    rr.JobsNew,
			JobsDupe:   rr.JobsDupe,
			DurationMs: rr.Duration.Milliseconds(),
		}
		if rr.Err != nil {
			dr.Error = rr.Err.Error()
		}
		results = append(results, dr)
	}

	h.log.Info().
		Int("scrapers", len(results)).
		Msg("discovery run complete")

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// ListSources returns the list of job-board scrapers registered with the service.
//
//	GET /api/v1/jobs/sources
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	infos := h.svc.Sources()
	sources := make([]sourceResponse, 0, len(infos))
	for _, info := range infos {
		sources = append(sources, sourceResponse{
			Source: string(info.Source),
			Name:   info.Name,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

// Stats returns job-discovery statistics (counts per source).
//
//	GET /api/v1/jobs/stats
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("failed to retrieve discovery stats")
		writeError(w, http.StatusInternalServerError, "failed to retrieve statistics")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
