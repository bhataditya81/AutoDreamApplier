package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
	usermodels "github.com/bhata/AutoDreamApplier/internal/user/models"
)

const (
	// minMatchScore is the floor below which we never surface a match.
	minMatchScore = 0.35
	// batchSize is how many new matches to insert per DB round-trip.
	batchSize = 50
	// maxJobsPerRun caps how many active jobs we score per user per run.
	maxJobsPerRun = 2000
)

// MatchingService orchestrates the matching loop.
type MatchingService struct {
	pool           *pgxpool.Pool
	matchRepo      *repository.MatchRepository
	scorer         *scorer.Scorer
	semanticScorer *scorer.SemanticScorer // optional; nil means keyword-only mode
	log            zerolog.Logger
}

// New creates a new MatchingService.
func New(pool *pgxpool.Pool, matchRepo *repository.MatchRepository, log zerolog.Logger) *MatchingService {
	return &MatchingService{
		pool:      pool,
		matchRepo: matchRepo,
		scorer:    scorer.New(),
		log:       log,
	}
}

// WithSemanticScorer attaches a SemanticScorer for combined keyword + semantic scoring.
// When set, the final match score = 0.6 * keyword_score + 0.4 * semantic_score.
// When nil (default), keyword score is used as-is (fully backward-compatible).
func (s *MatchingService) WithSemanticScorer(ss *scorer.SemanticScorer) *MatchingService {
	s.semanticScorer = ss
	return s
}

// RunResult summarises a matching run for one user.
type RunResult struct {
	UserID       uuid.UUID
	JobsScored   int
	JobsFiltered int // excluded by score or exclusion rules
	MatchesNew   int
	Duration     time.Duration
}

// RunForUser scores all un-matched active jobs against one user's preferences.
func (s *MatchingService) RunForUser(ctx context.Context, userID uuid.UUID) (*RunResult, error) {
	start := time.Now()
	log := s.log.With().Str("user_id", userID.String()).Logger()
	log.Info().Msg("starting matching run")

	// 1. Load user, preferences, and primary resume
	user, prefs, resume, err := s.loadUserContext(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user context: %w", err)
	}
	if prefs == nil {
		log.Warn().Msg("user has no preferences set — skipping")
		return &RunResult{UserID: userID}, nil
	}
	if resume == nil {
		log.Warn().Msg("user has no primary resume — skipping")
		return &RunResult{UserID: userID}, nil
	}

	// 2. Fetch already-matched job IDs so we don't re-score them
	alreadyMatched, err := s.matchRepo.GetAlreadyMatchedJobIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch matched job ids: %w", err)
	}

	// 3. Fetch active jobs (paginated, up to maxJobsPerRun)
	jobs, err := s.loadActiveJobs(ctx, maxJobsPerRun)
	if err != nil {
		return nil, fmt.Errorf("load active jobs: %w", err)
	}

	// 4. Score each job and collect candidates
	result := &RunResult{UserID: userID}
	candidates := make([]repository.MatchInsert, 0, len(jobs))

	for _, job := range jobs {
		// Skip already-matched
		if _, seen := alreadyMatched[job.ID]; seen {
			continue
		}
		result.JobsScored++

		// Apply exclusion filters (company name, job title, description keywords)
		if s.isExcluded(job, prefs.Exclusions) {
			result.JobsFiltered++
			continue
		}

		// Keyword score (5-dimension breakdown)
		bd := s.scorer.Score(job, user, prefs, resume)
		keywordScore := bd.Weighted()

		// Semantic score — only computed when scorer is attached and content is available.
		// Falls back to 0.5 (neutral) internally when AI service is unreachable.
		var weighted float64
		if s.semanticScorer != nil && job.Description != "" && resume.RawText != "" {
			semanticScore := s.semanticScorer.Score(ctx, resume.RawText, job.Description)
			// Combined: 60% keyword + 40% semantic
			weighted = 0.6*keywordScore + 0.4*semanticScore
		} else {
			weighted = keywordScore
		}

		if weighted < minMatchScore {
			result.JobsFiltered++
			continue
		}

		// Determine initial status
		status := models.MatchStatusPending // review mode default
		if user.ApplyMode == "auto" && weighted >= user.AutoThreshold {
			status = models.MatchStatusApproved
		}

		candidates = append(candidates, repository.MatchInsert{
			UserID:    userID,
			JobID:     job.ID,
			Score:     weighted,
			Breakdown: bd,
			Status:    status,
		})

		// Flush batch
		if len(candidates) >= batchSize {
			n, err := s.matchRepo.BulkInsert(ctx, candidates)
			if err != nil {
				log.Error().Err(err).Int("batch_size", len(candidates)).Msg("batch insert failed")
			}
			result.MatchesNew += n
			candidates = candidates[:0]
		}
	}

	// Flush remainder
	if len(candidates) > 0 {
		n, err := s.matchRepo.BulkInsert(ctx, candidates)
		if err != nil {
			log.Error().Err(err).Int("batch_size", len(candidates)).Msg("final batch insert failed")
		}
		result.MatchesNew += n
	}

	result.Duration = time.Since(start)
	log.Info().
		Int("scored", result.JobsScored).
		Int("filtered", result.JobsFiltered).
		Int("new_matches", result.MatchesNew).
		Dur("duration", result.Duration).
		Msg("matching run complete")

	return result, nil
}

// RunForAllActiveUsers runs the matching loop for every active user.
// Intended to be called by a cron job or a triggered endpoint.
func (s *MatchingService) RunForAllActiveUsers(ctx context.Context) ([]*RunResult, error) {
	userIDs, err := s.loadActiveUserIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active user ids: %w", err)
	}
	s.log.Info().Int("users", len(userIDs)).Msg("running matching for all active users")

	results := make([]*RunResult, 0, len(userIDs))
	for _, uid := range userIDs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		res, err := s.RunForUser(ctx, uid)
		if err != nil {
			s.log.Error().Err(err).Str("user_id", uid.String()).Msg("matching run failed for user")
			continue
		}
		results = append(results, res)
	}
	return results, nil
}

// ApproveMatch transitions a match from pending → approved (user approved it).
func (s *MatchingService) ApproveMatch(ctx context.Context, matchID, userID uuid.UUID) error {
	return s.matchRepo.UpdateStatus(ctx, matchID, userID, models.MatchStatusApproved)
}

// RejectMatch transitions a match from pending → rejected (user dismissed it).
func (s *MatchingService) RejectMatch(ctx context.Context, matchID, userID uuid.UUID) error {
	return s.matchRepo.UpdateStatus(ctx, matchID, userID, models.MatchStatusRejected)
}

// BulkApprove transitions multiple matches to approved in a single DB statement.
func (s *MatchingService) BulkApprove(ctx context.Context, userID uuid.UUID, matchIDs []uuid.UUID) (int, error) {
	n, err := s.matchRepo.BulkUpdateStatus(ctx, matchIDs, userID, models.MatchStatusApproved)
	if err != nil {
		return 0, fmt.Errorf("bulk approve: %w", err)
	}
	return n, nil
}

// BulkReject transitions multiple matches to rejected in a single DB statement.
func (s *MatchingService) BulkReject(ctx context.Context, userID uuid.UUID, matchIDs []uuid.UUID) (int, error) {
	n, err := s.matchRepo.BulkUpdateStatus(ctx, matchIDs, userID, models.MatchStatusRejected)
	if err != nil {
		return 0, fmt.Errorf("bulk reject: %w", err)
	}
	return n, nil
}

// SetFeedback records thumbs-up/thumbs-down on a match (for future ML training).
func (s *MatchingService) SetFeedback(ctx context.Context, matchID, userID uuid.UUID, feedback string) error {
	if feedback != "thumbs_up" && feedback != "thumbs_down" {
		return fmt.Errorf("invalid feedback value: must be thumbs_up or thumbs_down")
	}
	return s.matchRepo.SetFeedback(ctx, matchID, userID, feedback)
}

// ── private helpers ──────────────────────────────────────────────────────────

// loadUserContext fetches user, preferences, and primary resume in a single round-trip.
func (s *MatchingService) loadUserContext(
	ctx context.Context, userID uuid.UUID,
) (*usermodels.User, *usermodels.UserPreferences, *usermodels.Resume, error) {
	// Load user
	var user usermodels.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active
		FROM users WHERE id = $1 AND is_active = true`, userID,
	).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit, &user.IsActive,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load user: %w", err)
	}

	// Load preferences
	var prefs usermodels.UserPreferences
	err = s.pool.QueryRow(ctx, `
		SELECT id, user_id, target_titles, locations, remote_pref,
		       salary_min, salary_max, salary_currency, exclusions
		FROM user_preferences WHERE user_id = $1`, userID,
	).Scan(
		&prefs.ID, &prefs.UserID, &prefs.TargetTitles, &prefs.Locations, &prefs.RemotePref,
		&prefs.SalaryMin, &prefs.SalaryMax, &prefs.SalaryCurrency, &prefs.Exclusions,
	)
	if err != nil {
		// No preferences set is not a fatal error
		s.log.Warn().Str("user_id", userID.String()).Msg("no preferences found")
		return &user, nil, nil, nil
	}

	// Load primary resume
	var resume usermodels.Resume
	err = s.pool.QueryRow(ctx, `
		SELECT id, user_id, file_name, s3_key, raw_text, is_primary
		FROM resumes WHERE user_id = $1 AND is_primary = true
		LIMIT 1`, userID,
	).Scan(
		&resume.ID, &resume.UserID, &resume.FileName, &resume.S3Key, &resume.RawText, &resume.IsPrimary,
	)
	if err != nil {
		s.log.Warn().Str("user_id", userID.String()).Msg("no primary resume found")
		return &user, &prefs, nil, nil
	}

	return &user, &prefs, &resume, nil
}

// loadActiveJobs fetches active, non-scam jobs ordered by discovery time.
func (s *MatchingService) loadActiveJobs(ctx context.Context, limit int) ([]*discmodels.Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, external_id, source_board, url, title, company, location, is_remote,
		       salary_min, salary_max, salary_currency, description, ats_type, apply_url, posted_at
		FROM jobs
		WHERE is_active = true AND is_scam = false
		ORDER BY discovered_at DESC
		LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query active jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*discmodels.Job
	for rows.Next() {
		j := &discmodels.Job{}
		if err := rows.Scan(
			&j.ID, &j.ExternalID, &j.SourceBoard, &j.URL, &j.Title, &j.Company,
			&j.Location, &j.IsRemote, &j.SalaryMin, &j.SalaryMax, &j.SalaryCurrency,
			&j.Description, &j.ATSType, &j.ApplyURL, &j.PostedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// loadActiveUserIDs returns IDs of all active users.
func (s *MatchingService) loadActiveUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM users WHERE is_active = true`)
	if err != nil {
		return nil, fmt.Errorf("query active users: %w", err)
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

// isExcluded returns true if the job matches any of the user's exclusion keywords.
// Checks company name, job title, and description.
func (s *MatchingService) isExcluded(job *discmodels.Job, exclusions []string) bool {
	if len(exclusions) == 0 {
		return false
	}
	lower := func(ss string) string { return strings.ToLower(ss) }
	for _, excl := range exclusions {
		ex := lower(excl)
		if strings.Contains(lower(job.Company), ex) ||
			strings.Contains(lower(job.Title), ex) ||
			strings.Contains(lower(job.Description), ex) {
			return true
		}
	}
	return false
}
