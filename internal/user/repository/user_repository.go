package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/user/models"
)

// ErrResumeNotFound is returned when a resume does not exist or belongs to a different user.
var ErrResumeNotFound = errors.New("resume not found")

// UserRepository handles user database operations.
type UserRepository struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool, log zerolog.Logger) *UserRepository {
	return &UserRepository{pool: pool, log: log}
}

// FindByID retrieves a user by their UUID.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, cognito_sub, email, full_name, tier, apply_mode,
		        auto_threshold, daily_limit, is_active, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by ID: %w", err)
	}
	return user, nil
}

// FindByCognitoSub retrieves a user by their Cognito sub.
// GetUserIDBySub returns the database UUID for the given Cognito sub.
// Satisfies the handler.UserResolver interface used by the application handler.
func (r *UserRepository) GetUserIDBySub(ctx context.Context, sub string) (uuid.UUID, error) {
	user, err := r.FindByCognitoSub(ctx, sub)
	if err != nil {
		return uuid.Nil, err
	}
	if user == nil {
		return uuid.Nil, fmt.Errorf("user not found for sub %q", sub)
	}
	return user.ID, nil
}

func (r *UserRepository) FindByCognitoSub(ctx context.Context, sub string) (*models.User, error) {
	user := &models.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, cognito_sub, email, full_name, tier, apply_mode,
		        auto_threshold, daily_limit, is_active, created_at, updated_at
		 FROM users WHERE cognito_sub = $1`, sub,
	).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by cognito sub: %w", err)
	}
	return user, nil
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	user := &models.User{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (cognito_sub, email, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id, cognito_sub, email, full_name, tier, apply_mode,
		           auto_threshold, daily_limit, is_active, created_at, updated_at`,
		req.CognitoSub, req.Email, req.FullName,
	).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	r.log.Info().
		Str("user_id", user.ID.String()).
		Str("email", user.Email).
		Msg("user created")

	return user, nil
}

// Update updates a user's profile.
func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, req *models.UpdateUserRequest) (*models.User, error) {
	user := &models.User{}

	query := `UPDATE users SET `
	args := []interface{}{}
	argNum := 1
	updates := []string{}

	if req.FullName != nil {
		updates = append(updates, fmt.Sprintf("full_name = $%d", argNum))
		args = append(args, *req.FullName)
		argNum++
	}
	if req.ApplyMode != nil {
		updates = append(updates, fmt.Sprintf("apply_mode = $%d", argNum))
		args = append(args, *req.ApplyMode)
		argNum++
	}
	if req.AutoThreshold != nil {
		updates = append(updates, fmt.Sprintf("auto_threshold = $%d", argNum))
		args = append(args, *req.AutoThreshold)
		argNum++
	}

	if len(updates) == 0 {
		return r.FindByID(ctx, id)
	}

	query += joinStrings(updates, ", ")
	query += fmt.Sprintf(` WHERE id = $%d
		 RETURNING id, cognito_sub, email, full_name, tier, apply_mode,
		           auto_threshold, daily_limit, is_active, created_at, updated_at`, argNum)
	args = append(args, id)

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	return user, nil
}

// GetPreferences retrieves a user's job search preferences.
func (r *UserRepository) GetPreferences(ctx context.Context, userID uuid.UUID) (*models.UserPreferences, error) {
	prefs := &models.UserPreferences{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, target_titles, locations, remote_pref,
		        salary_min, salary_max, salary_currency, exclusions,
		        ai_tailor_enabled,
		        COALESCE(auto_apply_enabled, false),
		        COALESCE(daily_application_limit, 10),
		        COALESCE(apply_start_hour, 9),
		        COALESCE(apply_end_hour, 17),
		        COALESCE(apply_timezone, 'UTC'),
		        COALESCE(slack_webhook_url, ''),
		        COALESCE(discord_webhook_url, ''),
		        COALESCE(webhook_events, '{}'),
		        COALESCE(email_digest_enabled, true),
		        created_at, updated_at
		 FROM user_preferences WHERE user_id = $1`, userID,
	).Scan(
		&prefs.ID, &prefs.UserID, &prefs.TargetTitles, &prefs.Locations,
		&prefs.RemotePref, &prefs.SalaryMin, &prefs.SalaryMax,
		&prefs.SalaryCurrency, &prefs.Exclusions, &prefs.AiTailorEnabled,
		&prefs.AutoApplyEnabled, &prefs.DailyApplicationLimit,
		&prefs.ApplyStartHour, &prefs.ApplyEndHour, &prefs.ApplyTimezone,
		&prefs.SlackWebhookURL, &prefs.DiscordWebhookURL, &prefs.WebhookEvents,
		&prefs.EmailDigestEnabled,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting preferences: %w", err)
	}
	return prefs, nil
}

// UpsertPreferences creates or updates user preferences.
func (r *UserRepository) UpsertPreferences(ctx context.Context, userID uuid.UUID, req *models.UpdatePreferencesRequest) (*models.UserPreferences, error) {
	prefs := &models.UserPreferences{}
	currency := req.SalaryCurrency
	if currency == "" {
		currency = "USD"
	}
	timezone := req.ApplyTimezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Normalise webhook events — default to empty array when nil.
	webhookEvents := req.WebhookEvents
	if webhookEvents == nil {
		webhookEvents = []string{}
	}

	err := r.pool.QueryRow(ctx,
		`INSERT INTO user_preferences (user_id, target_titles, locations, remote_pref,
		                                salary_min, salary_max, salary_currency, exclusions, ai_tailor_enabled,
		                                auto_apply_enabled, daily_application_limit,
		                                apply_start_hour, apply_end_hour, apply_timezone,
		                                slack_webhook_url, discord_webhook_url, webhook_events, email_digest_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, true),
		         COALESCE($10, false), COALESCE($11, 10), COALESCE($12, 9), COALESCE($13, 17), $14,
		         NULLIF($15, ''), NULLIF($16, ''), $17, COALESCE($18, true))
		 ON CONFLICT (user_id) DO UPDATE SET
		   target_titles           = EXCLUDED.target_titles,
		   locations               = EXCLUDED.locations,
		   remote_pref             = EXCLUDED.remote_pref,
		   salary_min              = EXCLUDED.salary_min,
		   salary_max              = EXCLUDED.salary_max,
		   salary_currency         = EXCLUDED.salary_currency,
		   exclusions              = EXCLUDED.exclusions,
		   ai_tailor_enabled       = COALESCE($9,  user_preferences.ai_tailor_enabled),
		   auto_apply_enabled      = COALESCE($10, user_preferences.auto_apply_enabled),
		   daily_application_limit = COALESCE($11, user_preferences.daily_application_limit),
		   apply_start_hour        = COALESCE($12, user_preferences.apply_start_hour),
		   apply_end_hour          = COALESCE($13, user_preferences.apply_end_hour),
		   apply_timezone          = $14,
		   slack_webhook_url       = COALESCE(NULLIF($15, ''), user_preferences.slack_webhook_url),
		   discord_webhook_url     = COALESCE(NULLIF($16, ''), user_preferences.discord_webhook_url),
		   webhook_events          = $17,
		   email_digest_enabled    = COALESCE($18, user_preferences.email_digest_enabled)
		 RETURNING id, user_id, target_titles, locations, remote_pref,
		           salary_min, salary_max, salary_currency, exclusions,
		           ai_tailor_enabled,
		           COALESCE(auto_apply_enabled, false),
		           COALESCE(daily_application_limit, 10),
		           COALESCE(apply_start_hour, 9),
		           COALESCE(apply_end_hour, 17),
		           COALESCE(apply_timezone, 'UTC'),
		           COALESCE(slack_webhook_url, ''),
		           COALESCE(discord_webhook_url, ''),
		           COALESCE(webhook_events, '{}'),
		           COALESCE(email_digest_enabled, true),
		           created_at, updated_at`,
		userID, req.TargetTitles, req.Locations, req.RemotePref,
		req.SalaryMin, req.SalaryMax, currency, req.Exclusions, req.AiTailorEnabled,
		req.AutoApplyEnabled, req.DailyApplicationLimit, req.ApplyStartHour, req.ApplyEndHour, timezone,
		req.SlackWebhookURL, req.DiscordWebhookURL, webhookEvents, req.EmailDigestEnabled,
	).Scan(
		&prefs.ID, &prefs.UserID, &prefs.TargetTitles, &prefs.Locations,
		&prefs.RemotePref, &prefs.SalaryMin, &prefs.SalaryMax,
		&prefs.SalaryCurrency, &prefs.Exclusions, &prefs.AiTailorEnabled,
		&prefs.AutoApplyEnabled, &prefs.DailyApplicationLimit,
		&prefs.ApplyStartHour, &prefs.ApplyEndHour, &prefs.ApplyTimezone,
		&prefs.SlackWebhookURL, &prefs.DiscordWebhookURL, &prefs.WebhookEvents,
		&prefs.EmailDigestEnabled,
		&prefs.CreatedAt, &prefs.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting preferences: %w", err)
	}

	return prefs, nil
}

// GetResumes retrieves all resumes for a user, including A/B testing columns.
func (r *UserRepository) GetResumes(ctx context.Context, userID uuid.UUID) ([]models.Resume, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, file_name, s3_key, is_primary, interview_count,
		        COALESCE(ab_enabled, false), COALESCE(ab_weight, 1),
		        COALESCE(total_applications, 0), COALESCE(total_views, 0),
		        COALESCE(total_interviews, 0), COALESCE(total_offers, 0),
		        created_at
		 FROM resumes WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting resumes: %w", err)
	}
	defer rows.Close()

	var resumes []models.Resume
	for rows.Next() {
		var resume models.Resume
		if err := rows.Scan(
			&resume.ID, &resume.UserID, &resume.FileName, &resume.S3Key,
			&resume.IsPrimary, &resume.InterviewCount,
			&resume.AbEnabled, &resume.AbWeight,
			&resume.TotalApplications, &resume.TotalViews,
			&resume.TotalInterviews, &resume.TotalOffers,
			&resume.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning resume: %w", err)
		}
		// Compute interview rate as a percentage
		if resume.TotalApplications > 0 {
			resume.InterviewRate = float64(resume.TotalInterviews) / float64(resume.TotalApplications) * 100
		}
		resumes = append(resumes, resume)
	}

	return resumes, nil
}

// GetABResume selects a resume for A/B testing using weighted random sampling.
// If the user has any resumes with ab_enabled=true, pick one weighted by ab_weight.
// Otherwise, return the primary resume.
func (r *UserRepository) GetABResume(ctx context.Context, userID uuid.UUID) (*models.Resume, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, file_name, s3_key, is_primary, interview_count,
		        COALESCE(ab_enabled, false), COALESCE(ab_weight, 1),
		        COALESCE(total_applications, 0), COALESCE(total_views, 0),
		        COALESCE(total_interviews, 0), COALESCE(total_offers, 0),
		        created_at
		 FROM resumes
		 WHERE user_id = $1 AND COALESCE(ab_enabled, false) = true
		 ORDER BY ab_weight DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting ab resumes: %w", err)
	}
	defer rows.Close()

	var candidates []models.Resume
	for rows.Next() {
		var resume models.Resume
		if err := rows.Scan(
			&resume.ID, &resume.UserID, &resume.FileName, &resume.S3Key,
			&resume.IsPrimary, &resume.InterviewCount,
			&resume.AbEnabled, &resume.AbWeight,
			&resume.TotalApplications, &resume.TotalViews,
			&resume.TotalInterviews, &resume.TotalOffers,
			&resume.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning ab resume: %w", err)
		}
		if resume.TotalApplications > 0 {
			resume.InterviewRate = float64(resume.TotalInterviews) / float64(resume.TotalApplications) * 100
		}
		candidates = append(candidates, resume)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ab resumes: %w", err)
	}

	if len(candidates) == 0 {
		return nil, nil // caller should fall back to primary
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}

	// Weighted random selection
	totalWeight := 0
	for i := range candidates {
		totalWeight += candidates[i].AbWeight
	}
	pick := rand.Intn(totalWeight)
	cumulative := 0
	for i := range candidates {
		cumulative += candidates[i].AbWeight
		if pick < cumulative {
			return &candidates[i], nil
		}
	}
	return &candidates[len(candidates)-1], nil
}

// UpdateResumeAB updates the A/B testing settings for a resume owned by the user.
func (r *UserRepository) UpdateResumeAB(ctx context.Context, resumeID, userID uuid.UUID, enabled bool, weight int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE resumes SET ab_enabled = $1, ab_weight = $2 WHERE id = $3 AND user_id = $4`,
		enabled, weight, resumeID, userID,
	)
	if err != nil {
		return fmt.Errorf("updating resume ab settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resume not found")
	}
	return nil
}

// CreateResume inserts a new resume record.
func (r *UserRepository) CreateResume(ctx context.Context, resume *models.Resume) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO resumes (user_id, file_name, s3_key, parsed_json, raw_text, is_primary)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		resume.UserID, resume.FileName, resume.S3Key,
		resume.ParsedJSON, resume.RawText, resume.IsPrimary,
	).Scan(&resume.ID, &resume.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating resume: %w", err)
	}

	return nil
}

// SetPrimaryResume marks a resume as primary and unsets others.
func (r *UserRepository) SetPrimaryResume(ctx context.Context, userID, resumeID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err.Error() != "tx closed" {
			r.log.Err(err).Msg("failed to rollback transaction")
		}
	}()

	// Unset all
	if _, err := tx.Exec(ctx,
		`UPDATE resumes SET is_primary = false WHERE user_id = $1`, userID,
	); err != nil {
		return fmt.Errorf("unsetting primary resumes: %w", err)
	}

	// Set the chosen one
	tag, err := tx.Exec(ctx,
		`UPDATE resumes SET is_primary = true WHERE id = $1 AND user_id = $2`,
		resumeID, userID,
	)
	if err != nil {
		return fmt.Errorf("setting primary resume: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrResumeNotFound
	}

	return tx.Commit(ctx)
}

// GetStats retrieves summary statistics for a user.
func (r *UserRepository) GetStats(ctx context.Context, userID uuid.UUID) (*models.UserStats, error) {
	stats := &models.UserStats{}

	// Total applications
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM applications WHERE user_id = $1`, userID,
	).Scan(&stats.TotalApplications); err != nil {
		return nil, fmt.Errorf("getting total applications: %w", err)
	}

	// Applied today
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM applications WHERE user_id = $1 AND created_at >= CURRENT_DATE`, userID,
	).Scan(&stats.AppliedToday); err != nil {
		return nil, fmt.Errorf("getting applications today: %w", err)
	}

	// Pending matches
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM matches WHERE user_id = $1 AND status = 'pending'`, userID,
	).Scan(&stats.PendingMatches); err != nil {
		return nil, fmt.Errorf("getting pending matches: %w", err)
	}

	// Interviews
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM applications WHERE user_id = $1 AND outcome = 'interview'`, userID,
	).Scan(&stats.InterviewsReceived); err != nil {
		return nil, fmt.Errorf("getting interviews received: %w", err)
	}

	return stats, nil
}

// FindByEmail retrieves a user by their email address (used for dev auth only).
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, cognito_sub, email, full_name, tier, apply_mode,
		        auto_threshold, daily_limit, is_active, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(
		&user.ID, &user.CognitoSub, &user.Email, &user.FullName,
		&user.Tier, &user.ApplyMode, &user.AutoThreshold, &user.DailyLimit,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return user, nil
}

// FindPasswordHash retrieves the bcrypt password hash for a user (dev auth only).
func (r *UserRepository) FindPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash *string
	err := r.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id = $1`, userID,
	).Scan(&hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("finding password hash: %w", err)
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

// SetPasswordHash stores the bcrypt hash for a user (dev auth only).
func (r *UserRepository) SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, hash, userID,
	)
	if err != nil {
		return fmt.Errorf("setting password hash: %w", err)
	}
	return nil
}

// DeleteResume deletes a single resume owned by the user.
func (r *UserRepository) DeleteResume(ctx context.Context, userID, resumeID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM resumes WHERE id = $1 AND user_id = $2`, resumeID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting resume: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resume not found")
	}
	return nil
}

// SaveBoardCredential upserts an encrypted board credential for the user.
// ciphertext and iv are the outputs of the crypto.Encryptor.
func (r *UserRepository) SaveBoardCredential(ctx context.Context, userID uuid.UUID, boardName string, ciphertext, iv []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO board_credentials (user_id, board_name, encrypted_creds, iv)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, board_name) DO UPDATE SET
		   encrypted_creds = EXCLUDED.encrypted_creds,
		   iv              = EXCLUDED.iv,
		   is_valid        = true`,
		userID, boardName, ciphertext, iv,
	)
	if err != nil {
		return fmt.Errorf("saving board credential: %w", err)
	}
	return nil
}

// ListAutoApplyUsers returns all active users who have auto_apply_enabled set
// in their preferences. Used by AutoApproveService.
func (r *UserRepository) ListAutoApplyUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT u.id, u.cognito_sub, u.email, u.full_name, u.tier, u.apply_mode,
		        u.auto_threshold, u.daily_limit, u.is_active, u.created_at, u.updated_at
		 FROM users u
		 JOIN user_preferences p ON p.user_id = u.id
		 WHERE u.is_active = true
		   AND p.auto_apply_enabled = true`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing auto-apply users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.CognitoSub, &u.Email, &u.FullName,
			&u.Tier, &u.ApplyMode, &u.AutoThreshold, &u.DailyLimit,
			&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning auto-apply user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SaveContactMessage persists a contact form submission.
func (r *UserRepository) SaveContactMessage(ctx context.Context, name, email, subject, message string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO contact_messages (name, email, subject, message, created_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		name, email, subject, message,
	)
	if err != nil {
		return fmt.Errorf("SaveContactMessage: %w", err)
	}
	return nil
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
