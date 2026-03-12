package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/application/models"
	usermodels "github.com/bhata/AutoDreamApplier/internal/user/models"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = fmt.Errorf("not found")

// ApplicationRepository handles DB persistence for applications.
type ApplicationRepository struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// New creates a new ApplicationRepository.
func New(pool *pgxpool.Pool, log zerolog.Logger) *ApplicationRepository {
	return &ApplicationRepository{pool: pool, log: log}
}

// Create inserts a new application row and records a "created" event.
// Returns the newly created Application.
func (r *ApplicationRepository) Create(ctx context.Context, userID, jobID, matchID, resumeID uuid.UUID) (*models.Application, error) {
	app := &models.Application{
		ID:       uuid.New(),
		UserID:   userID,
		JobID:    jobID,
		MatchID:  matchID,
		ResumeID: resumeID,
		Status:   models.StatusQueued,
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO applications
			(id, user_id, job_id, match_id, resume_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
		app.ID, app.UserID, app.JobID, app.MatchID, app.ResumeID, string(app.Status),
	)
	if err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}

	if err := r.RecordEvent(ctx, app.ID, models.EventCreated, nil); err != nil {
		r.log.Warn().Err(err).Str("app_id", app.ID.String()).Msg("failed to record created event")
	}

	return app, nil
}

// GetByID fetches a single application, scoped to the given user.
func (r *ApplicationRepository) GetByID(ctx context.Context, appID, userID uuid.UUID) (*models.Application, error) {
	var app models.Application
	var outcome *string
	var appliedAt, outcomeUpdatedAt *time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT
			id, user_id, job_id, match_id, resume_id, status, outcome,
			tailored_resume_s3, cover_letter_s3, screenshot_s3,
			form_answers_json, error_message, applied_at, outcome_updated_at, created_at
		FROM applications
		WHERE id = $1 AND user_id = $2`,
		appID, userID,
	).Scan(
		&app.ID, &app.UserID, &app.JobID, &app.MatchID, &app.ResumeID,
		&app.Status, &outcome,
		&app.TailoredResumeS3, &app.CoverLetterS3, &app.ScreenshotS3,
		&app.FormAnswersJSON, &app.ErrorMessage,
		&appliedAt, &outcomeUpdatedAt, &app.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get application by id: %w", err)
	}

	if outcome != nil {
		o := models.ApplicationOutcome(*outcome)
		app.Outcome = &o
	}
	app.AppliedAt = appliedAt
	app.OutcomeUpdatedAt = outcomeUpdatedAt

	return &app, nil
}

// ListForUser returns applications for a user, newest first, with optional status filter.
func (r *ApplicationRepository) ListForUser(ctx context.Context, userID uuid.UUID, status models.ApplicationStatus, limit, offset int) ([]*models.Application, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	args := []any{userID}
	where := "user_id = $1"
	if status != "" {
		args = append(args, string(status))
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM applications WHERE %s`, where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count applications: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			id, user_id, job_id, match_id, resume_id, status, outcome,
			tailored_resume_s3, cover_letter_s3, screenshot_s3,
			form_answers_json, error_message, applied_at, outcome_updated_at, created_at
		FROM applications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	var results []*models.Application
	for rows.Next() {
		var app models.Application
		var outcome *string
		var appliedAt, outcomeUpdatedAt *time.Time

		if err := rows.Scan(
			&app.ID, &app.UserID, &app.JobID, &app.MatchID, &app.ResumeID,
			&app.Status, &outcome,
			&app.TailoredResumeS3, &app.CoverLetterS3, &app.ScreenshotS3,
			&app.FormAnswersJSON, &app.ErrorMessage,
			&appliedAt, &outcomeUpdatedAt, &app.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan application: %w", err)
		}

		if outcome != nil {
			o := models.ApplicationOutcome(*outcome)
			app.Outcome = &o
		}
		app.AppliedAt = appliedAt
		app.OutcomeUpdatedAt = outcomeUpdatedAt

		results = append(results, &app)
	}
	return results, total, rows.Err()
}

// UpdateStatus transitions an application to a new status.
func (r *ApplicationRepository) UpdateStatus(ctx context.Context, appID uuid.UUID, status models.ApplicationStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE applications SET status = $1 WHERE id = $2`,
		string(status), appID,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateOutcome records the post-submission outcome on an application.
func (r *ApplicationRepository) UpdateOutcome(ctx context.Context, appID, userID uuid.UUID, outcome models.ApplicationOutcome) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE applications
		SET outcome = $1, outcome_updated_at = NOW()
		WHERE id = $2 AND user_id = $3`,
		string(outcome), appID, userID,
	)
	if err != nil {
		return fmt.Errorf("update outcome: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetError records an error message and transitions to failed status.
func (r *ApplicationRepository) SetError(ctx context.Context, appID uuid.UUID, errMsg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applications
		SET status = $1, error_message = $2
		WHERE id = $3`,
		string(models.StatusFailed), errMsg, appID,
	)
	if err != nil {
		return fmt.Errorf("set error: %w", err)
	}
	return nil
}

// SetAppliedAt marks the application as applied with a timestamp.
func (r *ApplicationRepository) SetAppliedAt(ctx context.Context, appID uuid.UUID, appliedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applications
		SET status = $1, applied_at = $2
		WHERE id = $3`,
		string(models.StatusApplied), appliedAt, appID,
	)
	if err != nil {
		return fmt.Errorf("set applied at: %w", err)
	}
	return nil
}

// UpdateAIOutput stores AI-generated content keys and transitions to ai_ready.
func (r *ApplicationRepository) UpdateAIOutput(ctx context.Context, appID uuid.UUID, tailoredResumeS3, coverLetterS3 string, formAnswers map[string]string) error {
	answersJSON, err := json.Marshal(formAnswers)
	if err != nil {
		return fmt.Errorf("marshal form answers: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE applications
		SET status = $1, tailored_resume_s3 = $2, cover_letter_s3 = $3, form_answers_json = $4
		WHERE id = $5`,
		string(models.StatusAIReady), tailoredResumeS3, coverLetterS3, answersJSON, appID,
	)
	if err != nil {
		return fmt.Errorf("update ai output: %w", err)
	}
	return nil
}

// SetTailoredResumeAuth instantly maps an S3 key to the tailored output.
// Used exclusively when UserPreferences bypasses the AI Tailoring engine.
func (r *ApplicationRepository) SetTailoredResumeAuth(ctx context.Context, appID uuid.UUID, tailoredS3 string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applications
		SET tailored_resume_s3 = $1
		WHERE id = $2`,
		tailoredS3, appID,
	)
	if err != nil {
		return fmt.Errorf("bypass tailored resume map: %w", err)
	}
	return nil
}

// UpdateScreenshot stores the submission screenshot S3 key.
func (r *ApplicationRepository) UpdateScreenshot(ctx context.Context, appID uuid.UUID, screenshotS3 string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applications SET screenshot_s3 = $1 WHERE id = $2`,
		screenshotS3, appID,
	)
	if err != nil {
		return fmt.Errorf("update screenshot: %w", err)
	}
	return nil
}

// RecordEvent appends an audit-trail event for an application.
func (r *ApplicationRepository) RecordEvent(ctx context.Context, appID uuid.UUID, eventType string, details map[string]any) error {
	var detailsJSON []byte
	if details != nil {
		var err error
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal event details: %w", err)
		}
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO application_events (id, application_id, event_type, details, created_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		uuid.New(), appID, eventType, detailsJSON,
	)
	if err != nil {
		return fmt.Errorf("record event: %w", err)
	}
	return nil
}

// CountByStatus returns a breakdown of application counts by status for a user.
func (r *ApplicationRepository) CountByStatus(ctx context.Context, userID uuid.UUID) (map[models.ApplicationStatus]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM applications WHERE user_id = $1 GROUP BY status`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.ApplicationStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		counts[models.ApplicationStatus(status)] = count
	}
	return counts, rows.Err()
}

// CountAppliedToday returns the number of non-withdrawn applications
// created today (UTC midnight to now) for the given user.
// Used by the auto-apply scheduler to enforce the user's daily_limit.
func (r *ApplicationRepository) CountAppliedToday(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM applications
		WHERE user_id = $1
		  AND created_at >= CURRENT_DATE::timestamptz
		  AND status <> $2`,
		userID, string(models.StatusWithdrawn),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count applied today: %w", err)
	}
	return count, nil
}

// GetPrimaryResume fetches the user's primary resume.
// Used by workers to load resume data without calling the User Service HTTP API.
func (r *ApplicationRepository) GetPrimaryResume(ctx context.Context, userID uuid.UUID) (*usermodels.Resume, error) {
	var res usermodels.Resume

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, file_name, s3_key, raw_text, is_primary, interview_count, created_at
		FROM resumes
		WHERE user_id = $1 AND is_primary = true
		LIMIT 1`,
		userID,
	).Scan(
		&res.ID, &res.UserID, &res.FileName, &res.S3Key,
		&res.RawText, &res.IsPrimary, &res.InterviewCount, &res.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get primary resume: %w", err)
	}
	return &res, nil
}

// GetResume fetches a resume by ID, scoped to the user.
func (r *ApplicationRepository) GetResume(ctx context.Context, resumeID, userID uuid.UUID) (*usermodels.Resume, error) {
	var res usermodels.Resume

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, file_name, s3_key, raw_text, is_primary, interview_count, created_at
		FROM resumes
		WHERE id = $1 AND user_id = $2`,
		resumeID, userID,
	).Scan(
		&res.ID, &res.UserID, &res.FileName, &res.S3Key,
		&res.RawText, &res.IsPrimary, &res.InterviewCount, &res.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get resume: %w", err)
	}
	return &res, nil
}

// GetJob fetches a job by ID. Workers use this to get job details without
// making an HTTP call to the Job Discovery Service.
func (r *ApplicationRepository) GetJob(ctx context.Context, jobID uuid.UUID) (*discmodels.Job, error) {
	var j discmodels.Job

	err := r.pool.QueryRow(ctx, `
		SELECT
			id, external_id, source_board, url, title, company, location, is_remote,
			salary_min, salary_max, salary_currency, description, ats_type, apply_url,
			is_active, posted_at, discovered_at, created_at, updated_at
		FROM jobs
		WHERE id = $1`,
		jobID,
	).Scan(
		&j.ID, &j.ExternalID, &j.SourceBoard, &j.URL,
		&j.Title, &j.Company, &j.Location, &j.IsRemote,
		&j.SalaryMin, &j.SalaryMax, &j.SalaryCurrency,
		&j.Description, &j.ATSType, &j.ApplyURL,
		&j.IsActive, &j.PostedAt, &j.DiscoveredAt,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return &j, nil
}

// GetUser fetches a user by ID. Workers use this to get profile data without
// making an HTTP call to the User Service.
func (r *ApplicationRepository) GetUser(ctx context.Context, userID uuid.UUID) (*usermodels.User, error) {
	var u usermodels.User

	err := r.pool.QueryRow(ctx, `
		SELECT id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit, is_active, created_at, updated_at
		FROM users
		WHERE id = $1`,
		userID,
	).Scan(
		&u.ID, &u.CognitoSub, &u.Email, &u.FullName,
		&u.Tier, &u.ApplyMode, &u.AutoThreshold, &u.DailyLimit,
		&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// GetPreferences fetches user's job search preferences without making an HTTP call to the User Service.
func (r *ApplicationRepository) GetPreferences(ctx context.Context, userID uuid.UUID) (*usermodels.UserPreferences, error) {
	var prefs usermodels.UserPreferences
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, target_titles, locations, remote_pref,
		        salary_min, salary_max, salary_currency, exclusions,
		        ai_tailor_enabled, created_at, updated_at
		 FROM user_preferences WHERE user_id = $1`, userID,
	).Scan(
		&prefs.ID, &prefs.UserID, &prefs.TargetTitles, &prefs.Locations,
		&prefs.RemotePref, &prefs.SalaryMin, &prefs.SalaryMax,
		&prefs.SalaryCurrency, &prefs.Exclusions, &prefs.AiTailorEnabled,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No preferences exist yet
		}
		return nil, fmt.Errorf("getting preferences: %w", err)
	}
	return &prefs, nil
}
