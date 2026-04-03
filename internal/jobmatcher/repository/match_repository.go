package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
)

// MatchRepository handles persistence for job matches.
type MatchRepository struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// New creates a new MatchRepository.
func New(pool *pgxpool.Pool, log zerolog.Logger) *MatchRepository {
	return &MatchRepository{pool: pool, log: log}
}

// MatchInsert is the input for inserting a single match.
type MatchInsert struct {
	UserID    uuid.UUID
	JobID     uuid.UUID
	Score     float64
	Breakdown models.ScoreBreakdown
	Status    models.MatchStatus
}

// MatchWithJob combines a match with its job details for the API response.
type MatchWithJob struct {
	ID             uuid.UUID             `json:"id"`
	UserID         uuid.UUID             `json:"user_id"`
	JobID          uuid.UUID             `json:"job_id"`
	MatchScore     float64               `json:"match_score"`
	MatchBreakdown models.ScoreBreakdown `json:"match_breakdown"`
	Status         models.MatchStatus    `json:"status"`
	UserFeedback   string                `json:"user_feedback,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	// Denormalized job fields for display
	JobTitle     string             `json:"job_title"`
	JobCompany   string             `json:"job_company"`
	JobLocation  string             `json:"job_location"`
	JobIsRemote  bool               `json:"job_is_remote"`
	JobSalaryMin *int               `json:"job_salary_min"`
	JobSalaryMax *int               `json:"job_salary_max"`
	JobURL       string             `json:"job_url"`
	JobATSType   discmodels.ATSType `json:"job_ats_type"`
}

// GetUserIDBySub resolves a user's internal UUID from their cognito_sub.
// Used by JWT-aware handlers that receive a sub claim rather than a UUID.
func (r *MatchRepository) GetUserIDBySub(ctx context.Context, sub string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE cognito_sub = $1 AND is_active = true`, sub,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	return id, nil
}

// BulkInsert inserts multiple matches, skipping duplicates (same user+job).
// Returns the number of newly inserted rows.
func (r *MatchRepository) BulkInsert(ctx context.Context, matches []MatchInsert) (int, error) {
	if len(matches) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inserted := 0
	for _, m := range matches {
		breakdownJSON, err := json.Marshal(m.Breakdown)
		if err != nil {
			return inserted, fmt.Errorf("marshal breakdown: %w", err)
		}

		tag, err := tx.Exec(ctx, `
			INSERT INTO matches (id, user_id, job_id, match_score, match_breakdown, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			ON CONFLICT (user_id, job_id) DO NOTHING`,
			uuid.New(), m.UserID, m.JobID, m.Score, breakdownJSON, string(m.Status),
		)
		if err != nil {
			return inserted, fmt.Errorf("insert match (user=%s, job=%s): %w", m.UserID, m.JobID, err)
		}
		inserted += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

// ListForUser returns paginated matches for a user, with job details joined.
// status="" returns all statuses.
func (r *MatchRepository) ListForUser(
	ctx context.Context,
	userID uuid.UUID,
	status models.MatchStatus,
	limit, offset int,
) ([]MatchWithJob, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Build WHERE clause
	args := []any{userID}
	where := "m.user_id = $1"
	if status != "" {
		args = append(args, string(status))
		where += fmt.Sprintf(" AND m.status = $%d", len(args))
	}

	// Count total
	var total int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM matches m WHERE %s`, where),
		args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count matches: %w", err)
	}

	// Fetch page
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			m.id, m.user_id, m.job_id, m.match_score, m.match_breakdown, m.status,
			m.user_feedback, m.created_at,
			j.title, j.company, j.location, j.is_remote,
			j.salary_min, j.salary_max, j.url, j.ats_type
		FROM matches m
		JOIN jobs j ON j.id = m.job_id
		WHERE %s
		ORDER BY m.match_score DESC
		LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query matches: %w", err)
	}
	defer rows.Close()

	var results []MatchWithJob
	for rows.Next() {
		var mwj MatchWithJob
		var breakdownJSON []byte
		var feedback *string

		err := rows.Scan(
			&mwj.ID, &mwj.UserID, &mwj.JobID, &mwj.MatchScore, &breakdownJSON, &mwj.Status,
			&feedback, &mwj.CreatedAt,
			&mwj.JobTitle, &mwj.JobCompany, &mwj.JobLocation, &mwj.JobIsRemote,
			&mwj.JobSalaryMin, &mwj.JobSalaryMax, &mwj.JobURL, &mwj.JobATSType,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan match row: %w", err)
		}
		if feedback != nil {
			mwj.UserFeedback = *feedback
		}
		if err := json.Unmarshal(breakdownJSON, &mwj.MatchBreakdown); err != nil {
			r.log.Warn().Err(err).Str("match_id", mwj.ID.String()).Msg("failed to unmarshal match breakdown")
		}
		results = append(results, mwj)
	}
	return results, total, rows.Err()
}

// GetByID returns a single match with its job details, scoped to the given user.
func (r *MatchRepository) GetByID(ctx context.Context, matchID, userID uuid.UUID) (*MatchWithJob, error) {
	var mwj MatchWithJob
	var breakdownJSON []byte
	var feedback *string

	err := r.pool.QueryRow(ctx, `
		SELECT
			m.id, m.user_id, m.job_id, m.match_score, m.match_breakdown, m.status,
			m.user_feedback, m.created_at,
			j.title, j.company, j.location, j.is_remote,
			j.salary_min, j.salary_max, j.url, j.ats_type
		FROM matches m
		JOIN jobs j ON j.id = m.job_id
		WHERE m.id = $1 AND m.user_id = $2`,
		matchID, userID,
	).Scan(
		&mwj.ID, &mwj.UserID, &mwj.JobID, &mwj.MatchScore, &breakdownJSON, &mwj.Status,
		&feedback, &mwj.CreatedAt,
		&mwj.JobTitle, &mwj.JobCompany, &mwj.JobLocation, &mwj.JobIsRemote,
		&mwj.JobSalaryMin, &mwj.JobSalaryMax, &mwj.JobURL, &mwj.JobATSType,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get match by id: %w", err)
	}
	if feedback != nil {
		mwj.UserFeedback = *feedback
	}
	if len(breakdownJSON) > 0 {
		_ = json.Unmarshal(breakdownJSON, &mwj.MatchBreakdown)
	}
	return &mwj, nil
}

// BulkUpdateStatus transitions multiple matches to a new status in one statement,
// scoped to the given user. Returns the count of rows actually updated.
func (r *MatchRepository) BulkUpdateStatus(
	ctx context.Context,
	matchIDs []uuid.UUID,
	userID uuid.UUID,
	status models.MatchStatus,
) (int, error) {
	if len(matchIDs) == 0 {
		return 0, nil
	}
	// pgx accepts []uuid.UUID directly for ANY($1)
	tag, err := r.pool.Exec(ctx, `
		UPDATE matches SET status = $1, updated_at = NOW()
		WHERE id = ANY($2) AND user_id = $3`,
		string(status), matchIDs, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk update match status: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// UpdateStatus transitions a match to a new status, scoped to the user.
func (r *MatchRepository) UpdateStatus(ctx context.Context, matchID, userID uuid.UUID, status models.MatchStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE matches SET status = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3`,
		string(status), matchID, userID,
	)
	if err != nil {
		return fmt.Errorf("update match status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFeedback records a thumbs-up or thumbs-down on a match.
func (r *MatchRepository) SetFeedback(ctx context.Context, matchID, userID uuid.UUID, feedback string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE matches SET user_feedback = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3`,
		feedback, matchID, userID,
	)
	if err != nil {
		return fmt.Errorf("set match feedback: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetApprovedForUser returns approved matches ready for the Application Engine.
func (r *MatchRepository) GetApprovedForUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.Match, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, job_id, match_score, match_breakdown, status, created_at, updated_at
		FROM matches
		WHERE user_id = $1 AND status = 'approved'
		ORDER BY match_score DESC, created_at ASC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get queued matches: %w", err)
	}
	defer rows.Close()
	return scanMatches(rows)
}

// GetAlreadyMatchedJobIDs returns job IDs already matched for a user (any status).
// Used to prevent re-scoring jobs already in the match table.
func (r *MatchRepository) GetAlreadyMatchedJobIDs(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT job_id FROM matches WHERE user_id = $1`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get matched job ids: %w", err)
	}
	defer rows.Close()

	seen := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan job id: %w", err)
		}
		seen[id] = struct{}{}
	}
	return seen, rows.Err()
}

// GetAutoModeUserIDs returns IDs of active users with apply_mode='auto'
// who have at least one 'approved' match pending scheduler processing.
// The JOIN avoids a full table scan on matches; only users with pending
// work are returned.
func (r *MatchRepository) GetAutoModeUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT m.user_id
		FROM matches m
		JOIN users u ON u.id = m.user_id
		WHERE m.status = 'approved'
		  AND u.apply_mode = 'auto'
		  AND u.is_active = true`,
	)
	if err != nil {
		return nil, fmt.Errorf("get auto-mode user ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountByStatus returns the number of matches per status for a user.
func (r *MatchRepository) CountByStatus(ctx context.Context, userID uuid.UUID) (map[models.MatchStatus]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM matches WHERE user_id = $1 GROUP BY status`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.MatchStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		counts[models.MatchStatus(status)] = count
	}
	return counts, rows.Err()
}

// ListPendingMatches returns all pending matches for a user, ordered by score descending.
// Used by AutoApproveService to find candidates for auto-approval.
func (r *MatchRepository) ListPendingMatches(ctx context.Context, userID uuid.UUID) ([]models.Match, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, job_id, match_score, match_breakdown, status, created_at, updated_at
		FROM matches
		WHERE user_id = $1 AND status = 'pending'
		ORDER BY match_score DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending matches: %w", err)
	}
	defer rows.Close()
	return scanMatches(rows)
}

// UpdateMatchStatus transitions a match to a new status by ID (no user scope check).
// Intended for internal service use (e.g. AutoApproveService). For user-scoped
// updates use UpdateStatus instead.
func (r *MatchRepository) UpdateMatchStatus(ctx context.Context, matchID uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE matches SET status = $1, updated_at = NOW()
		WHERE id = $2`,
		status, matchID,
	)
	if err != nil {
		return fmt.Errorf("update match status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanMatches(rows pgx.Rows) ([]models.Match, error) {
	var results []models.Match
	for rows.Next() {
		var m models.Match
		var breakdownJSON []byte
		err := rows.Scan(
			&m.ID, &m.UserID, &m.JobID, &m.MatchScore,
			&breakdownJSON, &m.Status, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		if len(breakdownJSON) > 0 {
			bd := make(map[string]float64)
			if err := json.Unmarshal(breakdownJSON, &bd); err == nil {
				m.MatchBreakdown = bd
			}
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// containsAny reports whether s contains any of the given substrings (case-insensitive).
func containsAny(s string, subs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = fmt.Errorf("not found")

// Ensure containsAny is used (it's used indirectly via the service layer via export).
var _ = containsAny
