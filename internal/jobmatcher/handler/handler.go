package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

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

	// Match queue management (scoped by userID path param)
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
