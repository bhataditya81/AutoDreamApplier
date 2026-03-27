package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// JobRepository handles job persistence.
type JobRepository struct {
	pool *pgxpool.Pool
}

// NewJobRepository creates a new job repository.
func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

// Upsert inserts a new job or updates it if it already exists (by external_id + source_board).
// Returns the job ID and whether it was newly inserted.
func (r *JobRepository) Upsert(ctx context.Context, job *models.ScrapedJob) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var isNew bool

	query := `
		INSERT INTO jobs (
			external_id, source_board, url, title, company, location,
			is_remote, salary_min, salary_max, salary_currency,
			description, ats_type, apply_url, posted_at, discovered_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, NOW()
		)
		ON CONFLICT (external_id, source_board)
		DO UPDATE SET
			title        = EXCLUDED.title,
			description  = EXCLUDED.description,
			is_active    = true,
			updated_at   = NOW()
		RETURNING id, (xmax = 0) AS is_new`

	// Use ATS type set by the scraper/service when available; fall back to
	// URL-based detection otherwise.
	atsType := job.ATSType
	if atsType == "" || atsType == models.ATSUnknown {
		atsType = detectATS(job.ApplyURL, job.URL)
	}

	err := r.pool.QueryRow(ctx, query,
		job.ExternalID,
		string(job.Source),
		job.URL,
		job.Title,
		job.Company,
		job.Location,
		job.IsRemote,
		job.SalaryMin,
		job.SalaryMax,
		job.SalaryCurrency,
		job.Description,
		string(atsType),
		job.ApplyURL,
		job.PostedAt,
	).Scan(&id, &isNew)

	if err != nil {
		return uuid.Nil, false, fmt.Errorf("upsert job: %w", err)
	}
	return id, isNew, nil
}

// BulkUpsert performs multiple upserts in a single transaction.
// Returns counts of new and duplicate jobs.
func (r *JobRepository) BulkUpsert(ctx context.Context, jobs []*models.ScrapedJob) (newCount, dupeCount int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, job := range jobs {
		var id uuid.UUID
		var isNew bool

		atsType := job.ATSType
		if atsType == "" || atsType == models.ATSUnknown {
			atsType = detectATS(job.ApplyURL, job.URL)
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO jobs (
				external_id, source_board, url, title, company, location,
				is_remote, salary_min, salary_max, salary_currency,
				description, ats_type, apply_url, posted_at, discovered_at,
				is_scam, scam_score
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13, $14, NOW(),
				$15, $16
			)
			ON CONFLICT (external_id, source_board)
			DO UPDATE SET
				title       = EXCLUDED.title,
				description = EXCLUDED.description,
				is_active   = true,
				is_scam     = EXCLUDED.is_scam,
				scam_score  = EXCLUDED.scam_score,
				updated_at  = NOW()
			RETURNING id, (xmax = 0) AS is_new`,
			job.ExternalID, string(job.Source), job.URL, job.Title, job.Company,
			job.Location, job.IsRemote, job.SalaryMin, job.SalaryMax, job.SalaryCurrency,
			job.Description, string(atsType), job.ApplyURL, job.PostedAt,
			job.IsScam, job.ScamScore,
		).Scan(&id, &isNew)

		if err != nil {
			return 0, 0, fmt.Errorf("upsert job %q: %w", job.ExternalID, err)
		}

		if isNew {
			newCount++
		} else {
			dupeCount++
		}
		_ = id
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit bulk upsert: %w", err)
	}

	return newCount, dupeCount, nil
}

// MarkInactive marks all jobs from a source as inactive, then re-activates
// only the ones we saw in the latest scrape (by external ID).
func (r *JobRepository) MarkInactiveExcept(ctx context.Context, source models.JobSource, activeExternalIDs []string) error {
	// Mark all as inactive
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET is_active = false WHERE source_board = $1`,
		string(source),
	)
	if err != nil {
		return fmt.Errorf("mark inactive: %w", err)
	}

	// Re-activate seen jobs
	if len(activeExternalIDs) > 0 {
		_, err = r.pool.Exec(ctx,
			`UPDATE jobs SET is_active = true WHERE source_board = $1 AND external_id = ANY($2)`,
			string(source), activeExternalIDs,
		)
		if err != nil {
			return fmt.Errorf("re-activate jobs: %w", err)
		}
	}
	return nil
}

// GetActiveJobs returns active jobs for matching, with pagination.
func (r *JobRepository) GetActiveJobs(ctx context.Context, limit, offset int) ([]*models.Job, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, external_id, source_board, url, title, company, location,
		       is_remote, salary_min, salary_max, salary_currency,
		       description, ats_type, apply_url, is_active,
		       posted_at, discovered_at, created_at, updated_at
		FROM jobs
		WHERE is_active = true
		ORDER BY discovered_at DESC
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get active jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var j models.Job
		err := rows.Scan(
			&j.ID, &j.ExternalID, &j.SourceBoard, &j.URL, &j.Title, &j.Company,
			&j.Location, &j.IsRemote, &j.SalaryMin, &j.SalaryMax, &j.SalaryCurrency,
			&j.Description, &j.ATSType, &j.ApplyURL, &j.IsActive,
			&j.PostedAt, &j.DiscoveredAt, &j.CreatedAt, &j.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// SourceStats holds total and active job counts for one source board.
type SourceStats struct {
	Total  int
	Active int
}

// CountBySource returns total and active job counts grouped by source_board.
// The returned map key is the raw source_board string (e.g. "indeed", "glassdoor").
func (r *JobRepository) CountBySource(ctx context.Context) (map[string]SourceStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			source_board,
			COUNT(*)                                 AS total,
			COUNT(*) FILTER (WHERE is_active = true) AS active
		FROM jobs
		GROUP BY source_board
		ORDER BY source_board
	`)
	if err != nil {
		return nil, fmt.Errorf("count jobs by source: %w", err)
	}
	defer rows.Close()

	result := make(map[string]SourceStats)
	for rows.Next() {
		var source string
		var stats SourceStats
		if err := rows.Scan(&source, &stats.Total, &stats.Active); err != nil {
			return nil, fmt.Errorf("scan source stats: %w", err)
		}
		result[source] = stats
	}
	return result, rows.Err()
}

// detectATS infers the ATS type from the apply URL using case-insensitive
// substring matching. If applyURL is empty, fallbackURL (the job listing
// page URL) is tried instead — useful when a scraper provides the company
// career page URL rather than a direct ATS apply link.
//
// Pattern coverage mirrors internal/ats/detector.go; keep both in sync
// when adding new ATS types.
func detectATS(applyURL, fallbackURL string) models.ATSType {
	url := applyURL
	if url == "" {
		url = fallbackURL
	}
	if url == "" {
		return models.ATSUnknown
	}

	lower := strings.ToLower(url)

	switch {
	// Greenhouse: boards.greenhouse.io, short-link grnh.se, and embedded widget
	case strings.Contains(lower, "boards.greenhouse.io"),
		strings.Contains(lower, "grnh.se"),
		strings.Contains(lower, "greenhouse.io/embed/job_app"):
		return models.ATSGreenhouse

	// Lever: canonical jobs.lever.co and company career pages with lever.co/
	case strings.Contains(lower, "jobs.lever.co"),
		strings.Contains(lower, "lever.co/"):
		return models.ATSLever

	// Workday: covers *.wd1.myworkdayjobs.com, *.wd3.*, *.wd5.*, and bare CNAME
	case strings.Contains(lower, "myworkdayjobs.com"):
		return models.ATSWorkday

	// iCIMS: hosted ATS and iCIMS-branded subdomains
	case strings.Contains(lower, "icims.com"),
		strings.Contains(lower, "jobs-icims"):
		return models.ATSICIMS

	// Taleo (Oracle): .net and .com variants
	case strings.Contains(lower, "taleo.net"),
		strings.Contains(lower, "taleo.com"):
		return models.ATSTaleo

	// SAP SuccessFactors: global and EU instances
	case strings.Contains(lower, "successfactors.com"),
		strings.Contains(lower, "successfactors.eu"):
		return models.ATSSuccessFactors

	// SmartRecruiters: main domain and short-link smrtr.io
	case strings.Contains(lower, "smartrecruiters.com"),
		strings.Contains(lower, "smrtr.io"):
		return models.ATSSmartRecruiters

	// BambooHR: covers /careers and /jobs paths
	case strings.Contains(lower, "bamboohr.com"):
		return models.ATSBambooHR

	default:
		return models.ATSUnknown
	}
}

// Pool returns the underlying connection pool.
// Used by background services (e.g. EmbedNewJobs) that need direct DB access.
func (r *JobRepository) Pool() *pgxpool.Pool {
	return r.pool
}
