// Package handler provides HTTP handlers for the Application Engine API.
// Endpoints are mounted behind the API Gateway's JWT middleware; user identity
// is resolved from the JWT context via the auth package.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
	"github.com/bhata/AutoDreamApplier/internal/auth"
)

// UserResolver converts a Cognito subject string (e.g. "dev:alice@example.com")
// to a database user UUID. Implemented by UserRepository.GetByCognitoSub.
type UserResolver interface {
	GetUserIDBySub(ctx context.Context, sub string) (uuid.UUID, error)
}

// Handler holds dependencies for the application HTTP handlers.
type Handler struct {
	svc      *service.Service
	log      zerolog.Logger
	resolver UserResolver // may be nil (falls back to query-param user_id)
}

// New creates a new Handler.
func New(svc *service.Service, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// WithUserResolver attaches a UserResolver so handlers can derive the user UUID
// from the JWT context rather than requiring an explicit user_id query param.
func (h *Handler) WithUserResolver(r UserResolver) *Handler {
	h.resolver = r
	return h
}

// getUserID resolves the authenticated user's UUID. It first tries the JWT
// context (preferred), then falls back to the supplied fallback string (e.g.
// from a query param or request body). Returns an error if neither works.
func (h *Handler) getUserID(r *http.Request, fallback string) (uuid.UUID, error) {
	if h.resolver != nil {
		if claims, ok := auth.GetUserFromContext(r.Context()); ok && claims.Sub != "" {
			id, err := h.resolver.GetUserIDBySub(r.Context(), claims.Sub)
			if err == nil {
				return id, nil
			}
		}
	}
	return uuid.Parse(fallback)
}

// Routes returns a Chi sub-router wired to all application endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Post("/submit", h.Submit)
	r.Get("/stats", h.Stats)          // registered before /{appID} for clarity
	r.Get("/", h.List)
	r.Get("/{appID}", h.GetByID)
	r.Put("/{appID}/outcome", h.RecordOutcome)   // REST: full replace
	r.Patch("/{appID}/outcome", h.RecordOutcome) // REST: partial update (frontend uses PATCH)
	r.Post("/emergency-stop", h.EmergencyStop)

	return r
}

// Router is kept for backward compatibility (e.g. apply-engine internal use).
func Router(svc *service.Service, log zerolog.Logger) http.Handler {
	return New(svc, log).Routes()
}

// ── Local request types ───────────────────────────────────────────────────────

// outcomeBody extends models.UpdateOutcomeRequest with the user_id field so
// the service can enforce row-level ownership without a separate auth lookup.
type outcomeBody struct {
	UserID  string `json:"user_id"`
	Outcome string `json:"outcome"`
	Notes   string `json:"notes,omitempty"` // optional; stored in outcome_notes
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

	userID, err := h.getUserID(r, q.Get("user_id"))
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

	search := q.Get("search")

	apps, total, err := h.svc.ListForUser(r.Context(), userID, status, search, limit, offset)
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

	if err := h.svc.RecordOutcome(r.Context(), appID, userID, outcome, req.Notes); err != nil {
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
	// Body is optional when user_id can be resolved from JWT context.
	_ = json.NewDecoder(r.Body).Decode(&req)

	userID, err := h.getUserID(r, req.UserID)
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
	userID, err := h.getUserID(r, r.URL.Query().Get("user_id"))
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
