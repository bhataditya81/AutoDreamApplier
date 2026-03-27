// Package analytics provides repository and HTTP handler for application
// analytics endpoints (funnel, over-time chart, resume performance, top companies).
package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Repository holds a database connection pool and logger for analytics queries.
type Repository struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// New creates a new Repository.
func New(pool *pgxpool.Pool, log zerolog.Logger) *Repository {
	return &Repository{pool: pool, log: log}
}

// ── Result types ──────────────────────────────────────────────────────────────

// FunnelStats breaks down the outcome funnel for a user in a date range.
type FunnelStats struct {
	Applied       int     `json:"applied"`
	Viewed        int     `json:"viewed"`
	Interviews    int     `json:"interviews"`
	Offers        int     `json:"offers"`
	ViewRate      float64 `json:"view_rate"`      // viewed/applied * 100
	InterviewRate float64 `json:"interview_rate"` // interviews/applied * 100
	OfferRate     float64 `json:"offer_rate"`     // offers/applied * 100
}

// DailyCount is one data point in the over-time chart.
type DailyCount struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Count int    `json:"count"`
}

// ResumeStats holds interview performance per resume.
type ResumeStats struct {
	ResumeID          string  `json:"resume_id"`
	FileName          string  `json:"file_name"`
	TotalApplications int     `json:"total_applications"`
	TotalInterviews   int     `json:"total_interviews"`
	TotalOffers       int     `json:"total_offers"`
	InterviewRate     float64 `json:"interview_rate"`
}

// CompanyStat is one entry in the top-companies list.
type CompanyStat struct {
	Company string `json:"company"`
	Count   int    `json:"count"`
}

// ── Query methods ─────────────────────────────────────────────────────────────

// GetFunnelStats returns outcome counts and derived rates for a user over the
// given date range. Only rows with status = 'applied' are included.
func (r *Repository) GetFunnelStats(ctx context.Context, userID uuid.UUID, from, to time.Time) (FunnelStats, error) {
	const q = `
		SELECT
			COUNT(*)                                             AS applied,
			COUNT(*) FILTER (WHERE outcome = 'viewed')          AS viewed,
			COUNT(*) FILTER (WHERE outcome = 'interview')       AS interviews,
			COUNT(*) FILTER (WHERE outcome = 'offer')           AS offers
		FROM applications
		WHERE user_id  = $1
		  AND status   = 'applied'
		  AND created_at BETWEEN $2 AND $3`

	var fs FunnelStats
	err := r.pool.QueryRow(ctx, q, userID, from, to).
		Scan(&fs.Applied, &fs.Viewed, &fs.Interviews, &fs.Offers)
	if err != nil {
		r.log.Error().Err(err).Msg("GetFunnelStats: query failed")
		return FunnelStats{}, err
	}

	if fs.Applied > 0 {
		fs.ViewRate = float64(fs.Viewed) / float64(fs.Applied) * 100
		fs.InterviewRate = float64(fs.Interviews) / float64(fs.Applied) * 100
		fs.OfferRate = float64(fs.Offers) / float64(fs.Applied) * 100
	}
	return fs, nil
}

// GetDailyCounts returns the number of applications created each day in the
// given date range. Days with zero applications are not included here; the
// handler fills those in.
func (r *Repository) GetDailyCounts(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]DailyCount, error) {
	const q = `
		SELECT
			created_at::date AS date,
			COUNT(*)         AS count
		FROM applications
		WHERE user_id    = $1
		  AND created_at BETWEEN $2 AND $3
		GROUP BY 1
		ORDER BY 1`

	rows, err := r.pool.Query(ctx, q, userID, from, to)
	if err != nil {
		r.log.Error().Err(err).Msg("GetDailyCounts: query failed")
		return nil, err
	}
	defer rows.Close()

	var counts []DailyCount
	for rows.Next() {
		var dc DailyCount
		var t time.Time
		if err := rows.Scan(&t, &dc.Count); err != nil {
			return nil, err
		}
		dc.Date = t.Format("2006-01-02")
		counts = append(counts, dc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// GetResumeStats returns per-resume performance metrics for a user, ordered by
// interview rate descending.
func (r *Repository) GetResumeStats(ctx context.Context, userID uuid.UUID) ([]ResumeStats, error) {
	const q = `
		SELECT
			r.id,
			r.file_name,
			r.total_applications,
			r.total_interviews,
			r.total_offers,
			CASE WHEN r.total_applications > 0
			     THEN ROUND(r.total_interviews::numeric / r.total_applications * 100, 1)
			     ELSE 0 END AS interview_rate
		FROM resumes r
		WHERE r.user_id = $1
		ORDER BY interview_rate DESC, r.total_applications DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		r.log.Error().Err(err).Msg("GetResumeStats: query failed")
		return nil, err
	}
	defer rows.Close()

	var stats []ResumeStats
	for rows.Next() {
		var rs ResumeStats
		var id uuid.UUID
		if err := rows.Scan(&id, &rs.FileName, &rs.TotalApplications, &rs.TotalInterviews, &rs.TotalOffers, &rs.InterviewRate); err != nil {
			return nil, err
		}
		rs.ResumeID = id.String()
		stats = append(stats, rs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetTopCompanies returns the companies with the most applications for a user,
// ordered by count descending.
func (r *Repository) GetTopCompanies(ctx context.Context, userID uuid.UUID, limit int) ([]CompanyStat, error) {
	const q = `
		SELECT j.company, COUNT(*) AS count
		FROM applications a
		JOIN jobs j ON j.id = a.job_id
		WHERE a.user_id  = $1
		  AND j.company IS NOT NULL
		  AND j.company != ''
		GROUP BY j.company
		ORDER BY count DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		r.log.Error().Err(err).Msg("GetTopCompanies: query failed")
		return nil, err
	}
	defer rows.Close()

	var companies []CompanyStat
	for rows.Next() {
		var cs CompanyStat
		if err := rows.Scan(&cs.Company, &cs.Count); err != nil {
			return nil, err
		}
		companies = append(companies, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return companies, nil
}
