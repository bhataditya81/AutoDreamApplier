package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
)

const (
	dateLayout = "2006-01-02"
	maxDays    = 365
)

// UserResolver converts a Cognito subject string to a database user UUID.
// Implemented by UserRepository.GetUserIDBySub.
type UserResolver interface {
	GetUserIDBySub(ctx context.Context, sub string) (uuid.UUID, error)
}

// Handler holds dependencies for the analytics HTTP handlers.
type Handler struct {
	repo     *Repository
	log      zerolog.Logger
	resolver UserResolver // may be nil (falls back to query-param user_id)
}

// NewHandler creates a new Handler.
func NewHandler(repo *Repository, log zerolog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

// WithUserResolver attaches a UserResolver so handlers can derive the user UUID
// from the JWT context rather than requiring an explicit user_id query param.
func (h *Handler) WithUserResolver(r UserResolver) *Handler {
	h.resolver = r
	return h
}

// Routes returns a Chi sub-router wired to all analytics endpoints.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/funnel", h.Funnel)
	r.Get("/over-time", h.OverTime)
	r.Get("/by-resume", h.ByResume)
	r.Get("/top-companies", h.TopCompanies)
	return r
}

// getUserID resolves the authenticated user's UUID. It first tries the JWT
// context (preferred), then falls back to the supplied fallback string (e.g.
// from a query param). Returns an error if neither works.
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

// parseDateRange parses the ?from= and ?to= query params.
// Defaults: from = 30 days ago, to = today. Max range: 365 days (clamped silently).
func parseDateRange(q interface{ Get(string) string }) (from, to time.Time, err error) {
	now := time.Now().UTC().Truncate(24 * time.Hour)

	toStr := q.Get("to")
	if toStr == "" {
		to = now
	} else {
		to, err = time.Parse(dateLayout, toStr)
		if err != nil {
			return
		}
		to = to.UTC()
	}

	fromStr := q.Get("from")
	if fromStr == "" {
		from = to.AddDate(0, 0, -30)
	} else {
		from, err = time.Parse(dateLayout, fromStr)
		if err != nil {
			return
		}
		from = from.UTC()
	}

	// Clamp to maxDays silently.
	if to.Sub(from) > maxDays*24*time.Hour {
		from = to.AddDate(0, 0, -maxDays)
	}

	return from, to, nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Funnel returns the outcome funnel stats for the requesting user.
//
// GET /funnel?from=YYYY-MM-DD&to=YYYY-MM-DD&user_id=...
// Response 200: FunnelStats JSON
func (h *Handler) Funnel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	userID, err := h.getUserID(r, q.Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	from, to, err := parseDateRange(q)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid date format; use YYYY-MM-DD")
		return
	}

	stats, err := h.repo.GetFunnelStats(r.Context(), userID, from, to)
	if err != nil {
		h.log.Error().Err(err).Msg("GetFunnelStats failed")
		jsonError(w, http.StatusInternalServerError, "failed to get funnel stats")
		return
	}

	respond(w, http.StatusOK, stats)
}

// OverTime returns daily application counts, filling missing days with 0.
//
// GET /over-time?from=YYYY-MM-DD&to=YYYY-MM-DD&user_id=...
// Response 200: []DailyCount JSON
func (h *Handler) OverTime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	userID, err := h.getUserID(r, q.Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	from, to, err := parseDateRange(q)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid date format; use YYYY-MM-DD")
		return
	}

	raw, err := h.repo.GetDailyCounts(r.Context(), userID, from, to)
	if err != nil {
		h.log.Error().Err(err).Msg("GetDailyCounts failed")
		jsonError(w, http.StatusInternalServerError, "failed to get over-time data")
		return
	}

	// Build a map of date → count from the DB results.
	byDate := make(map[string]int, len(raw))
	for _, dc := range raw {
		byDate[dc.Date] = dc.Count
	}

	// Fill every calendar day in the range, including days with zero applications.
	days := int(to.Sub(from).Hours()/24) + 1
	result := make([]DailyCount, 0, days)
	for i := 0; i < days; i++ {
		d := from.AddDate(0, 0, i)
		key := d.Format(dateLayout)
		result = append(result, DailyCount{Date: key, Count: byDate[key]})
	}

	respond(w, http.StatusOK, result)
}

// ByResume returns per-resume performance metrics for the requesting user.
//
// GET /by-resume?user_id=...
// Response 200: []ResumeStats JSON
func (h *Handler) ByResume(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUserID(r, r.URL.Query().Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	stats, err := h.repo.GetResumeStats(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Msg("GetResumeStats failed")
		jsonError(w, http.StatusInternalServerError, "failed to get resume stats")
		return
	}

	if stats == nil {
		stats = []ResumeStats{}
	}
	respond(w, http.StatusOK, stats)
}

// TopCompanies returns the companies with the most applications for the user.
//
// GET /top-companies?limit=10&user_id=...
// Response 200: []CompanyStat JSON
func (h *Handler) TopCompanies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	userID, err := h.getUserID(r, q.Get("user_id"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	limit := 10
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	companies, err := h.repo.GetTopCompanies(r.Context(), userID, limit)
	if err != nil {
		h.log.Error().Err(err).Msg("GetTopCompanies failed")
		jsonError(w, http.StatusInternalServerError, "failed to get top companies")
		return
	}

	if companies == nil {
		companies = []CompanyStat{}
	}
	respond(w, http.StatusOK, companies)
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
