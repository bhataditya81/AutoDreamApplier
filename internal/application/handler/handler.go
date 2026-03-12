// Package handler provides HTTP handlers for the Application Engine API.
// All endpoints are internal-service facing; caller passes user_id explicitly
// (the API Gateway validates the JWT and forwards the claim downstream).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
)

// Handler holds dependencies for the application HTTP handlers.
type Handler struct {
	svc *service.Service
	log zerolog.Logger
}

// New creates a new Handler.
func New(svc *service.Service, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Router returns a Chi sub-router wired to all application endpoints.
// Mount this at a prefix such as /api/v1/applications in main.go.
func Router(svc *service.Service, log zerolog.Logger) http.Handler {
	h := New(svc, log)
	r := chi.NewRouter()

	r.Post("/submit", h.Submit)
	r.Get("/stats", h.Stats)          // registered before /{appID} for clarity
	r.Get("/", h.List)
	r.Get("/{appID}", h.GetByID)
	r.Put("/{appID}/outcome", h.RecordOutcome)
	r.Post("/emergency-stop", h.EmergencyStop)

	return r
}

// ── Local request types ───────────────────────────────────────────────────────

// outcomeBody extends models.UpdateOutcomeRequest with the user_id field so
// the service can enforce row-level ownership without a separate auth lookup.
type outcomeBody struct {
	UserID  string `json:"user_id"`
	Outcome string `json:"outcome"`
}

type emergencyStopBody struct {
	UserID string `json:"user_id"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Submit creates a new application and enqueues Stage 1 (AI prep).
//
// POST /submit
// Body: { "user_id": "...", "job_id": "...", "match_id": "..." }
// Response 202: models.Application JSON
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	var req models.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	jobID, err := uuid.Parse(req.JobID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid job_id")
		return
	}
	matchID, err := uuid.Parse(req.MatchID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid match_id")
		return
	}

	app, err := h.svc.Submit(r.Context(), userID, jobID, matchID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			// ErrNotFound here means the user has no primary resume.
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		h.log.Error().Err(err).Msg("submit application failed")
		jsonError(w, http.StatusInternalServerError, "failed to submit application")
		return
	}

	respond(w, http.StatusAccepted, app)
}

// GetByID returns a single application scoped to the requesting user.
//
// GET /{appID}?user_id=...
// Response 200: models.Application JSON
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid app_id")
		return
	}

	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	app, err := h.svc.GetByID(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			jsonError(w, http.StatusNotFound, "application not found")
			return
		}
		h.log.Error().Err(err).Msg("get application failed")
		jsonError(w, http.StatusInternalServerError, "failed to get application")
		return
	}

	respond(w, http.StatusOK, app)
}

// List returns a paginated list of applications for a user with an optional
// status filter.
//
// GET /?user_id=...&status=...&limit=20&offset=0
// Response 200: { "applications": [...], "total": N, "limit": N, "offset": N }
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	userID, err := uuid.Parse(q.Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	// Optional status filter — empty string means "all statuses".
	status := models.ApplicationStatus(q.Get("status"))

	limit := 20
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	apps, total, err := h.svc.ListForUser(r.Context(), userID, status, limit, offset)
	if err != nil {
		h.log.Error().Err(err).Msg("list applications failed")
		jsonError(w, http.StatusInternalServerError, "failed to list applications")
		return
	}

	respond(w, http.StatusOK, map[string]any{
		"applications": apps,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// RecordOutcome updates the post-submission outcome for an application.
// Valid outcomes: viewed, rejected, interview, offer.
//
// PUT /{appID}/outcome
// Body: { "user_id": "...", "outcome": "viewed|rejected|interview|offer" }
// Response 200: { "status": "ok" }
func (h *Handler) RecordOutcome(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid app_id")
		return
	}

	var req outcomeBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	outcome := models.ApplicationOutcome(req.Outcome)
	switch outcome {
	case models.OutcomeViewed, models.OutcomeRejected,
		models.OutcomeInterview, models.OutcomeOffer:
		// valid
	default:
		jsonError(w, http.StatusBadRequest,
			"outcome must be one of: viewed, rejected, interview, offer")
		return
	}

	if err := h.svc.RecordOutcome(r.Context(), appID, userID, outcome); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			jsonError(w, http.StatusNotFound, "application not found")
			return
		}
		h.log.Error().Err(err).Msg("record outcome failed")
		jsonError(w, http.StatusInternalServerError, "failed to record outcome")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// EmergencyStop kills all active browser sessions for the given user.
//
// POST /emergency-stop
// Body: { "user_id": "..." }
// Response 200: { "status": "ok" }
func (h *Handler) EmergencyStop(w http.ResponseWriter, r *http.Request) {
	var req emergencyStopBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	if err := h.svc.EmergencyStop(r.Context(), userID); err != nil {
		h.log.Error().Err(err).Msg("emergency stop failed")
		jsonError(w, http.StatusInternalServerError, "emergency stop failed")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Stats returns application counts grouped by status for a user.
//
// GET /stats?user_id=...
// Response 200: { "counts": { "queued": N, "applied": N, ... } }
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.URL.Query().Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	counts, err := h.svc.CountByStatus(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("count by status failed")
		jsonError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	respond(w, http.StatusOK, map[string]any{"counts": counts})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Response header already written; log is best-effort only.
		_ = err
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
