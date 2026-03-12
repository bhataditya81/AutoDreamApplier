package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/pkg/response"
)

// Handler handles HTTP requests for the job-matcher service.
type Handler struct {
	svc       *service.MatchingService
	matchRepo *repository.MatchRepository
	log       zerolog.Logger
}

// New creates a new Handler.
func New(svc *service.MatchingService, matchRepo *repository.MatchRepository, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, matchRepo: matchRepo, log: log}
}

// Routes mounts all match routes on the given router.
// Expected to be mounted at: /api/v1/matches
func (h *Handler) Routes(r chi.Router) {
	// Trigger a matching run
	r.Post("/run/{userID}", h.RunForUser)
	r.Post("/run-all", h.RunForAllUsers)

	// JWT-aware routes used by the frontend (userID extracted from bearer token)
	r.Get("/", h.ListMatchesMe)
	r.Patch("/{matchID}", h.UpdateMatchMe)
	r.Patch("/{matchID}/feedback", h.FeedbackMe)
	r.Post("/bulk-action", h.BulkAction)

	// Legacy routes (scoped by userID path param — kept for internal/admin use)
	r.Get("/user/{userID}", h.ListMatches)
	r.Get("/{matchID}/user/{userID}", h.GetMatch)
	r.Post("/{matchID}/approve", h.ApproveMatch)
	r.Post("/{matchID}/reject", h.RejectMatch)
	r.Post("/{matchID}/feedback", h.SetFeedback)
}

// ── Run triggers ─────────────────────────────────────────────────────────────

// RunForUser triggers a matching run for a specific user.
// POST /api/v1/matches/run/{userID}
func (h *Handler) RunForUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		response.BadRequest(w, "invalid user ID")
		return
	}

	result, err := h.svc.RunForUser(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", userID.String()).Msg("matching run failed")
		response.InternalError(w, "matching run failed")
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// RunForAllUsers triggers a matching run for all active users.
// POST /api/v1/matches/run-all
func (h *Handler) RunForAllUsers(w http.ResponseWriter, r *http.Request) {
	results, err := h.svc.RunForAllActiveUsers(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("bulk matching run failed")
		response.InternalError(w, "bulk matching run failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"users_processed": len(results),
		"results":         results,
	})
}

// ── Match CRUD ────────────────────────────────────────────────────────────────

// ListMatches returns paginated matches for a user.
// GET /api/v1/matches/user/{userID}?status=pending&limit=20&offset=0
func (h *Handler) ListMatches(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		response.BadRequest(w, "invalid user ID")
		return
	}

	status := models.MatchStatus(r.URL.Query().Get("status"))
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	matches, total, err := h.matchRepo.ListForUser(r.Context(), userID, status, limit, offset)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", userID.String()).Msg("list matches failed")
		response.InternalError(w, "failed to list matches")
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	response.JSONWithMeta(w, http.StatusOK, matches, &response.Meta{
		PerPage:    limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetMatch returns a single match by ID.
// GET /api/v1/matches/{matchID}/user/{userID}
func (h *Handler) GetMatch(w http.ResponseWriter, r *http.Request) {
	matchID, err := parseUUID(chi.URLParam(r, "matchID"))
	if err != nil {
		response.BadRequest(w, "invalid match ID")
		return
	}
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		response.BadRequest(w, "invalid user ID")
		return
	}

	match, err := h.matchRepo.GetByID(r.Context(), matchID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		h.log.Error().Err(err).Msg("get match failed")
		response.InternalError(w, "failed to retrieve match")
		return
	}

	response.JSON(w, http.StatusOK, match)
}

// ApproveMatch transitions a match to "queued" (approved for application).
// POST /api/v1/matches/{matchID}/approve  body: {"user_id":"..."}
func (h *Handler) ApproveMatch(w http.ResponseWriter, r *http.Request) {
	matchID, userID, err := h.parseMatchAndUser(w, r)
	if err != nil {
		return
	}

	if err := h.svc.ApproveMatch(r.Context(), matchID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		h.log.Error().Err(err).Msg("approve match failed")
		response.InternalError(w, "failed to approve match")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// RejectMatch transitions a match to "skipped".
// POST /api/v1/matches/{matchID}/reject  body: {"user_id":"..."}
func (h *Handler) RejectMatch(w http.ResponseWriter, r *http.Request) {
	matchID, userID, err := h.parseMatchAndUser(w, r)
	if err != nil {
		return
	}

	if err := h.svc.RejectMatch(r.Context(), matchID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		h.log.Error().Err(err).Msg("reject match failed")
		response.InternalError(w, "failed to reject match")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "skipped"})
}

// SetFeedback records thumbs_up or thumbs_down feedback on a match.
// POST /api/v1/matches/{matchID}/feedback  body: {"user_id":"...","feedback":"thumbs_up"}
func (h *Handler) SetFeedback(w http.ResponseWriter, r *http.Request) {
	matchID, err := parseUUID(chi.URLParam(r, "matchID"))
	if err != nil {
		response.BadRequest(w, "invalid match ID")
		return
	}

	var req struct {
		UserID   string `json:"user_id"`
		Feedback string `json:"feedback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	userID, err := parseUUID(req.UserID)
	if err != nil {
		response.BadRequest(w, "invalid user_id")
		return
	}

	if err := h.svc.SetFeedback(r.Context(), matchID, userID, req.Feedback); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"feedback": req.Feedback})
}

// ── JWT-aware handlers (frontend) ─────────────────────────────────────────────

// ListMatchesMe returns paginated matches for the authenticated user.
// GET /api/v1/matches?status=pending&limit=20&offset=0
func (h *Handler) ListMatchesMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.resolveUserID(w, r)
	if !ok {
		return
	}

	status := models.MatchStatus(r.URL.Query().Get("status"))
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	matches, total, err := h.matchRepo.ListForUser(r.Context(), userID, status, limit, offset)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", userID.String()).Msg("list matches (me) failed")
		response.InternalError(w, "failed to list matches")
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	response.JSONWithMeta(w, http.StatusOK, matches, &response.Meta{
		PerPage:    limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// UpdateMatchMe approves or rejects a single match for the authenticated user.
// PATCH /api/v1/matches/{matchID}  body: {"status":"approved"|"rejected"}
func (h *Handler) UpdateMatchMe(w http.ResponseWriter, r *http.Request) {
	matchID, err := parseUUID(chi.URLParam(r, "matchID"))
	if err != nil {
		response.BadRequest(w, "invalid match ID")
		return
	}

	userID, ok := h.resolveUserID(w, r)
	if !ok {
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	var svcErr error
	switch req.Status {
	case "approved":
		svcErr = h.svc.ApproveMatch(r.Context(), matchID, userID)
	case "rejected":
		svcErr = h.svc.RejectMatch(r.Context(), matchID, userID)
	default:
		response.BadRequest(w, "status must be 'approved' or 'rejected'")
		return
	}

	if svcErr != nil {
		if errors.Is(svcErr, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		h.log.Error().Err(svcErr).Msg("update match (me) failed")
		response.InternalError(w, "failed to update match")
		return
	}

	// Return a lightweight updated object the frontend can merge into local state
	response.JSON(w, http.StatusOK, map[string]string{
		"id":     matchID.String(),
		"status": req.Status,
	})
}

// BulkAction approves or rejects multiple matches for the authenticated user.
// POST /api/v1/matches/bulk-action  body: {"action":"approve"|"reject","match_ids":["..."]}
func (h *Handler) BulkAction(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.resolveUserID(w, r)
	if !ok {
		return
	}

	var req struct {
		Action   string   `json:"action"`
		MatchIDs []string `json:"match_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if len(req.MatchIDs) == 0 {
		response.BadRequest(w, "match_ids must not be empty")
		return
	}
	if len(req.MatchIDs) > 100 {
		response.BadRequest(w, "bulk action limited to 100 matches at a time")
		return
	}

	matchIDs := make([]uuid.UUID, 0, len(req.MatchIDs))
	for _, raw := range req.MatchIDs {
		id, err := parseUUID(raw)
		if err != nil {
			response.BadRequest(w, "invalid match_id: "+raw)
			return
		}
		matchIDs = append(matchIDs, id)
	}

	var (
		updated int
		svcErr  error
	)
	switch req.Action {
	case "approve":
		updated, svcErr = h.svc.BulkApprove(r.Context(), userID, matchIDs)
	case "reject":
		updated, svcErr = h.svc.BulkReject(r.Context(), userID, matchIDs)
	default:
		response.BadRequest(w, "action must be 'approve' or 'reject'")
		return
	}

	if svcErr != nil {
		h.log.Error().Err(svcErr).Str("action", req.Action).Msg("bulk action failed")
		response.InternalError(w, "bulk action failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"action":  req.Action,
		"updated": updated,
	})
}

// FeedbackMe records thumbs_up or thumbs_down quality feedback on a match for
// the authenticated user (separate from approve/reject status).
// PATCH /api/v1/matches/{matchID}/feedback  body: {"feedback":"thumbs_up"|"thumbs_down"}
func (h *Handler) FeedbackMe(w http.ResponseWriter, r *http.Request) {
	matchID, err := parseUUID(chi.URLParam(r, "matchID"))
	if err != nil {
		response.BadRequest(w, "invalid match ID")
		return
	}

	userID, ok := h.resolveUserID(w, r)
	if !ok {
		return
	}

	var req struct {
		Feedback string `json:"feedback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if err := h.svc.SetFeedback(r.Context(), matchID, userID, req.Feedback); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "match not found")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"feedback": req.Feedback})
}

// resolveUserID extracts the authenticated user's internal UUID from the JWT
// claims stored in request context.
func (h *Handler) resolveUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "not authenticated")
		return uuid.Nil, false
	}

	userID, err := h.matchRepo.GetUserIDBySub(r.Context(), claims.Sub)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.NotFound(w, "user not found")
			return uuid.Nil, false
		}
		h.log.Error().Err(err).Str("sub", claims.Sub).Msg("resolve user ID failed")
		response.InternalError(w, "failed to resolve user")
		return uuid.Nil, false
	}

	return userID, true
}

// ── helpers ───────────────────────────────────────────────────────────────────

// parseMatchAndUser reads matchID from path and userID from the request body.
func (h *Handler) parseMatchAndUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, error) {
	matchID, err := parseUUID(chi.URLParam(r, "matchID"))
	if err != nil {
		response.BadRequest(w, "invalid match ID")
		return uuid.Nil, uuid.Nil, err
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return uuid.Nil, uuid.Nil, err
	}
	userID, err := parseUUID(req.UserID)
	if err != nil {
		response.BadRequest(w, "invalid user_id")
		return uuid.Nil, uuid.Nil, err
	}
	return matchID, userID, nil
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
