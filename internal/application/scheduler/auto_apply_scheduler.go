// Package scheduler contains the auto-apply background scheduler.
// It runs on a fixed tick interval and submits approved matches for users
// who have opted into auto-apply mode, subject to business-hours and
// per-user daily-limit constraints.
package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	matchmodels "github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	appRepo "github.com/bhata/AutoDreamApplier/internal/application/repository"
	appSvc "github.com/bhata/AutoDreamApplier/internal/application/service"
	matchRepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
)

const (
	// tickInterval is how often the scheduler wakes up to look for work.
	tickInterval = 2 * time.Minute

	// maxMatchesPerRun caps matches processed per user per tick.
	// Keeps each tick fast and prevents thundering-herd on the browser pool.
	maxMatchesPerRun = 5

	// defaultBusinessHourStart / End define the UTC fallback window.
	defaultBusinessHourStart = 9  // 09:00 UTC
	defaultBusinessHourEnd   = 17 // 17:00 UTC
)

// Scheduler picks up approved matches for auto-mode users and submits them.
type Scheduler struct {
	appRepo   *appRepo.ApplicationRepository
	matchRepo *matchRepo.MatchRepository
	svc       *appSvc.Service
	log       zerolog.Logger
}

// New creates a Scheduler.
func New(
	ar *appRepo.ApplicationRepository,
	mr *matchRepo.MatchRepository,
	svc *appSvc.Service,
	log zerolog.Logger,
) *Scheduler {
	return &Scheduler{
		appRepo:   ar,
		matchRepo: mr,
		svc:       svc,
		log:       log.With().Str("component", "auto-apply-scheduler").Logger(),
	}
}

// Run starts the scheduler loop. It blocks until ctx is cancelled.
// Intended to be launched as a goroutine alongside the HTTP and Asynq servers.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info().
		Dur("interval", tickInterval).
		Int("default_biz_hour_start_utc", defaultBusinessHourStart).
		Int("default_biz_hour_end_utc", defaultBusinessHourEnd).
		Msg("auto-apply scheduler started")

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	// Run once immediately so we don't wait a full tick after startup.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("auto-apply scheduler stopping")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick executes one scheduling cycle.
// Each user's apply window is evaluated individually using their own timezone.
func (s *Scheduler) tick(ctx context.Context) {
	userIDs, err := s.matchRepo.GetAutoModeUserIDs(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to fetch auto-mode user IDs")
		return
	}
	if len(userIDs) == 0 {
		return
	}

	s.log.Debug().Int("users", len(userIDs)).Msg("auto-mode users with pending approved matches")

	for _, userID := range userIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.processUser(ctx, userID)
	}
}

// processUser submits approved matches for a single user, respecting their
// daily_limit and per-user apply window (timezone + hours).
func (s *Scheduler) processUser(ctx context.Context, userID uuid.UUID) {
	log := s.log.With().Str("user_id", userID.String()).Logger()

	// Fetch user to get daily_limit.
	user, err := s.appRepo.GetUser(ctx, userID)
	if err != nil || user == nil {
		log.Warn().Err(err).Msg("user not found — skipping")
		return
	}

	// Fetch user preferences to get timezone and apply window.
	prefs, err := s.appRepo.GetPreferences(ctx, userID)
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch user preferences — using defaults")
	}

	// Determine per-user apply window (fall back to UTC defaults).
	applyTimezone := "UTC"
	applyStartHour := defaultBusinessHourStart
	applyEndHour := defaultBusinessHourEnd
	if prefs != nil {
		if prefs.ApplyTimezone != "" {
			applyTimezone = prefs.ApplyTimezone
		}
		if prefs.ApplyStartHour > 0 || prefs.ApplyEndHour > 0 {
			applyStartHour = prefs.ApplyStartHour
			applyEndHour = prefs.ApplyEndHour
		}
	}

	// Skip this user if outside their configured apply window.
	if !isWithinApplyWindow(applyTimezone, applyStartHour, applyEndHour) {
		log.Debug().
			Str("timezone", applyTimezone).
			Int("start_hour", applyStartHour).
			Int("end_hour", applyEndHour).
			Msg("outside apply window for user — skipping")
		return
	}

	dailyLimit := user.DailyLimit
	if dailyLimit <= 0 {
		dailyLimit = 5 // safety default if not set
	}

	// Count non-withdrawn applications already submitted today (UTC).
	appliedToday, err := s.appRepo.CountAppliedToday(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("failed to count applications today")
		return
	}

	remaining := dailyLimit - appliedToday
	if remaining <= 0 {
		log.Debug().
			Int("daily_limit", dailyLimit).
			Int("applied_today", appliedToday).
			Msg("daily limit reached — skipping user")
		return
	}

	// Cap at maxMatchesPerRun to keep each tick fast.
	limit := remaining
	if limit > maxMatchesPerRun {
		limit = maxMatchesPerRun
	}

	// Fetch approved matches for this user.
	matches, err := s.matchRepo.GetApprovedForUser(ctx, userID, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch approved matches")
		return
	}

	submitted := 0
	for _, m := range matches {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.submitMatch(ctx, m); err != nil {
			log.Error().Err(err).
				Str("match_id", m.ID.String()).
				Str("job_id", m.JobID.String()).
				Msg("auto-apply submission failed")
			// Continue to the next match rather than aborting the user.
			continue
		}
		submitted++
	}

	if submitted > 0 {
		log.Info().
			Int("submitted", submitted).
			Int("daily_limit", dailyLimit).
			Int("applied_today", appliedToday).
			Msg("auto-apply: submitted matches for user")
	}
}

// submitMatch calls svc.Submit and updates the match status to 'applied'
// so the scheduler does not re-process it on the next tick.
func (s *Scheduler) submitMatch(ctx context.Context, m matchmodels.Match) error {
	log := s.log.With().
		Str("match_id", m.ID.String()).
		Str("job_id", m.JobID.String()).
		Logger()

	log.Debug().Msg("auto-submitting match")

	if _, err := s.svc.Submit(ctx, m.UserID, m.JobID, m.ID); err != nil {
		return err
	}

	// Mark the match as 'applied' so the scheduler won't pick it up again.
	// The application pipeline will update this further as it progresses.
	if err := s.matchRepo.UpdateStatus(ctx, m.ID, m.UserID, matchmodels.MatchStatusApplied); err != nil {
		// Non-fatal: the application was enqueued. Log and continue.
		log.Warn().Err(err).Msg("submitted but failed to update match status to 'applied'")
	}

	return nil
}

// isBusinessHours returns true when the current UTC time falls within
// Mon–Fri using the default UTC business hours window.
// Kept for backward compatibility; prefer isWithinApplyWindow for per-user checks.
func isBusinessHours() bool {
	return isWithinApplyWindow("UTC", defaultBusinessHourStart, defaultBusinessHourEnd)
}

// isWithinApplyWindow returns true when the current time (in the given timezone)
// falls within Mon–Fri startHour–endHour.
// Falls back to UTC if the timezone string is invalid.
func isWithinApplyWindow(tz string, startHour, endHour int) bool {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	wd := now.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h := now.Hour()
	return h >= startHour && h < endHour
}
