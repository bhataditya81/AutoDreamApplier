// Package service contains business logic for application orchestration.
// It sits between the HTTP handler and the repository / task queue,
// keeping both sides thin and testable.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"
)

// ErrNotFound is re-exported so callers (e.g. the HTTP handler) do not need to
// import the repository package solely for the sentinel error.
var ErrNotFound = repository.ErrNotFound

// Service provides business logic for application orchestration.
type Service struct {
	repo          *repository.ApplicationRepository
	riverClient   *river.Client[pgx.Tx]
	browserClient *browser.Client
	notifier      *notification.Client // nil-safe; no-ops when SES is unconfigured
	rateLimiter   *RateLimiter         // optional; nil = no rate limiting (backward compat)
	log           zerolog.Logger
}

// New creates a new application Service.
// notifier may be nil — all notification calls become no-ops.
func New(
	repo *repository.ApplicationRepository,
	riverClient *river.Client[pgx.Tx],
	browserClient *browser.Client,
	notifier *notification.Client,
	log zerolog.Logger,
) *Service {
	return &Service{
		repo:          repo,
		riverClient:   riverClient,
		browserClient: browserClient,
		notifier:      notifier,
		log:           log,
	}
}

// WithRateLimiter sets an optional Redis-backed rate limiter on the service.
// When set, Submit will enforce per-user daily application limits.
func (s *Service) WithRateLimiter(rl *RateLimiter) *Service {
	s.rateLimiter = rl
	return s
}

// Submit creates a new application record and enqueues Stage 1 (AI prep).
// If the user has any A/B-enabled resumes, one is selected via weighted random
// sampling. Otherwise the primary resume is used.
func (s *Service) Submit(ctx context.Context, userID, jobID, matchID uuid.UUID) (*models.Application, error) {
	// ── Resolve resume (A/B selection with primary fallback) ────────────────────
	resume, err := s.repo.GetABResume(ctx, userID)
	if err != nil {
		s.log.Warn().Err(err).Str("user_id", userID.String()).Msg("ab resume selection failed; falling back to primary")
		resume = nil
	}
	if resume == nil {
		resume, err = s.repo.GetPrimaryResume(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, fmt.Errorf("no primary resume found for user: %w", ErrNotFound)
			}
			return nil, fmt.Errorf("get primary resume: %w", err)
		}
	}

	// ── Check User Preferences for AI Bypass + rate limiting ────────────────────
	prefs, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user preferences: %w", err)
	}

	// ── Enforce daily rate limit (if configured) ────────────────────────────────
	if s.rateLimiter != nil && prefs != nil {
		limit := prefs.DailyApplicationLimit
		if limit <= 0 {
			limit = 10
		}
		timezone := prefs.ApplyTimezone
		if timezone == "" {
			timezone = "UTC"
		}
		if err := s.rateLimiter.CheckAndIncrement(ctx, userID, limit, timezone); err != nil {
			return nil, fmt.Errorf("rate limit: %w", err)
		}
	}

	// ── Create the application row ──────────────────────────────────────────────
	app, err := s.repo.Create(ctx, userID, jobID, matchID, resume.ID)
	if err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}

	if prefs != nil && !prefs.AiTailorEnabled {
		// Bypass AI Tailoring
		// We explicitly map the raw resume S3 key into the 'tailored' state so the browser picks it up.
		if err := s.repo.SetTailoredResumeAuth(ctx, app.ID, resume.S3Key); err != nil {
			return nil, err
		}

		if _, err := s.riverClient.Insert(ctx, tasks.BrowserApplyArgs{
			ApplicationID: app.ID,
			UserID:        userID,
			JobID:         jobID,
		}, nil); err != nil {
			return nil, fmt.Errorf("insert browser_apply job: %w", err)
		}

		s.log.Info().
			Str("app_id", app.ID.String()).
			Str("user_id", userID.String()).
			Msg("application submitted; ai bypass active, browser_apply task enqueued")
		return app, nil
	}

	// ── Insert Stage 1 job — NOTIFY wakes AI prep worker immediately ─────────
	if _, err := s.riverClient.Insert(ctx, tasks.AIPrepArgs{
		ApplicationID: app.ID,
		UserID:        userID,
		JobID:         jobID,
		ResumeID:      resume.ID,
	}, nil); err != nil {
		return nil, fmt.Errorf("insert ai_prep job: %w", err)
	}

	s.log.Info().
		Str("app_id", app.ID.String()).
		Str("user_id", userID.String()).
		Str("job_id", jobID.String()).
		Str("resume_id", resume.ID.String()).
		Msg("application submitted; ai_prep task enqueued")

	return app, nil
}

// GetByID returns a single application scoped to the given user.
// Returns ErrNotFound if the application does not exist or belongs to a different user.
func (s *Service) GetByID(ctx context.Context, appID, userID uuid.UUID) (*models.Application, error) {
	app, err := s.repo.GetByID(ctx, appID, userID)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}
	return app, nil
}

// ListForUser returns paginated applications for a user with an optional status filter.
// Pass an empty status to return all statuses. limit is capped at 100 by the repository.
func (s *Service) ListForUser(
	ctx context.Context,
	userID uuid.UUID,
	status models.ApplicationStatus,
	search string,
	limit, offset int,
) ([]*models.Application, int64, error) {
	apps, total, err := s.repo.ListForUser(ctx, userID, status, search, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list applications: %w", err)
	}
	return apps, total, nil
}

// RecordOutcome updates the post-submission outcome of an application and
// sends an email notification for interview/offer outcomes.
// Valid outcomes: viewed, rejected, interview, offer.
// notes is optional free-text stored in outcome_notes; pass "" to leave unchanged.
func (s *Service) RecordOutcome(ctx context.Context, appID, userID uuid.UUID, outcome models.ApplicationOutcome, notes string) error {
	if err := s.repo.UpdateOutcome(ctx, appID, userID, outcome, notes); err != nil {
		return fmt.Errorf("update outcome: %w", err)
	}

	s.log.Info().
		Str("app_id", appID.String()).
		Str("user_id", userID.String()).
		Str("outcome", string(outcome)).
		Msg("application outcome recorded")

	// ── Notify user for high-value outcomes ────────────────────────────────────
	// Silently skip notifications if SES is unconfigured (notifier is nil-safe).
	switch outcome {
	case models.OutcomeInterview, models.OutcomeOffer:
		s.sendOutcomeNotification(ctx, appID, userID, outcome)
	}

	return nil
}

// sendOutcomeNotification fetches the details needed for the email and sends it.
// Errors are logged but never propagated — the outcome update already succeeded.
func (s *Service) sendOutcomeNotification(
	ctx context.Context,
	appID, userID uuid.UUID,
	outcome models.ApplicationOutcome,
) {
	app, err := s.repo.GetByID(ctx, appID, userID)
	if err != nil {
		s.log.Warn().Err(err).Str("app_id", appID.String()).Msg("outcome notify: failed to load application")
		return
	}

	job, err := s.repo.GetJob(ctx, app.JobID)
	if err != nil {
		s.log.Warn().Err(err).Str("app_id", appID.String()).Msg("outcome notify: failed to load job")
		return
	}

	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		s.log.Warn().Err(err).Str("user_id", userID.String()).Msg("outcome notify: failed to load user")
		return
	}

	label, emoji := outcomeLabel(outcome)

	if notifyErr := s.notifier.SendOutcomeUpdate(ctx, user.Email, notification.OutcomeData{
		UserName:     user.FullName,
		JobTitle:     job.Title,
		Company:      job.Company,
		Outcome:      label,
		OutcomeEmoji: emoji,
	}); notifyErr != nil {
		s.log.Warn().Err(notifyErr).
			Str("app_id", appID.String()).
			Str("outcome", string(outcome)).
			Msg("outcome notify: SES send failed (non-fatal)")
	}
}

// outcomeLabel maps an ApplicationOutcome to a human-friendly label and emoji.
func outcomeLabel(o models.ApplicationOutcome) (label, emoji string) {
	switch o {
	case models.OutcomeInterview:
		return "Interview Request", "🎉"
	case models.OutcomeOffer:
		return "Job Offer", "🚀"
	case models.OutcomeRejected:
		return "Application Rejected", "😔"
	default:
		return string(o), "📬"
	}
}

// EmergencyStop kills all active browser sessions for the given user.
func (s *Service) EmergencyStop(ctx context.Context, userID uuid.UUID) error {
	if err := s.browserClient.EmergencyStop(ctx, userID.String()); err != nil {
		return fmt.Errorf("emergency stop: %w", err)
	}
	s.log.Warn().Str("user_id", userID.String()).Msg("emergency stop triggered for user")
	return nil
}

// CountByStatus returns the count of applications grouped by status for a user.
func (s *Service) CountByStatus(ctx context.Context, userID uuid.UUID) (map[models.ApplicationStatus]int, error) {
	counts, err := s.repo.CountByStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	return counts, nil
}
