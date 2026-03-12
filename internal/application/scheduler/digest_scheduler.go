package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	appRepo "github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/notification"
)

const (
	// digestCheckInterval is how often the scheduler wakes up to check whether
	// it should dispatch the weekly digest. Hourly ticks are cheap and ensure
	// the digest fires within 60 minutes of the target window even if the
	// process was just restarted.
	digestCheckInterval = 1 * time.Hour

	// digestDayOfWeek is the weekday on which the digest is sent.
	digestDayOfWeek = time.Monday

	// digestHourUTC is the UTC hour at which the digest is sent (09:00 UTC).
	digestHourUTC = 9

	// digestDashboardURL is the base URL embedded in digest emails.
	// TODO: make this a config value once the domain is confirmed.
	digestDashboardURL = "https://app.autodreamapplier.com/dashboard"
)

// DigestScheduler fires a weekly summary email to every active user on
// Mondays at 09:00 UTC. It is designed to be run as a long-lived goroutine
// inside the apply-engine process alongside the auto-apply scheduler.
//
// The scheduler uses an in-memory `lastSent` timestamp to prevent duplicate
// sends if the process is restarted within the same Monday morning window.
// On a fresh startup (lastSent is zero) it calls maybeDispatch immediately,
// so a process that restarts at 09:30 on a Monday will still send the digest.
type DigestScheduler struct {
	appRepo  *appRepo.ApplicationRepository
	notifier *notification.Client
	log      zerolog.Logger
	lastSent time.Time
}

// NewDigestScheduler creates a DigestScheduler. The notification.Client may
// be nil in development (SES not configured); all notification methods are
// nil-safe and will simply log a no-op.
func NewDigestScheduler(
	ar *appRepo.ApplicationRepository,
	n *notification.Client,
	log zerolog.Logger,
) *DigestScheduler {
	return &DigestScheduler{
		appRepo:  ar,
		notifier: n,
		log:      log.With().Str("component", "digest_scheduler").Logger(),
	}
}

// Run starts the digest scheduler loop. It blocks until ctx is cancelled.
// Call it in a goroutine: go digestSched.Run(ctx).
func (s *DigestScheduler) Run(ctx context.Context) {
	s.log.Info().
		Str("check_interval", digestCheckInterval.String()).
		Int("dispatch_hour_utc", digestHourUTC).
		Str("dispatch_weekday", digestDayOfWeek.String()).
		Msg("digest scheduler started")

	// Check immediately on startup so a Monday-morning restart doesn't miss
	// the window.
	s.maybeDispatch(ctx)

	ticker := time.NewTicker(digestCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("digest scheduler stopped")
			return
		case <-ticker.C:
			s.maybeDispatch(ctx)
		}
	}
}

// maybeDispatch checks whether the current moment falls within the Monday
// 09:00 UTC dispatch window and, if so, calls dispatch. It guards against
// double-sends by comparing lastSent to the start of the current window.
func (s *DigestScheduler) maybeDispatch(ctx context.Context) {
	now := time.Now().UTC()

	// Only fire on the configured weekday and hour.
	if now.Weekday() != digestDayOfWeek || now.Hour() != digestHourUTC {
		return
	}

	// Compute the start of this week's dispatch window (Monday 09:00:00 UTC).
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), digestHourUTC, 0, 0, 0, time.UTC)

	// Guard: don't send twice within the same window.
	if !s.lastSent.IsZero() && !s.lastSent.Before(windowStart) {
		s.log.Debug().
			Time("last_sent", s.lastSent).
			Time("window_start", windowStart).
			Msg("digest already sent for this window, skipping")
		return
	}

	s.log.Info().Time("window_start", windowStart).Msg("dispatching weekly digest")
	s.dispatch(ctx, now)
}

// dispatch fetches all active users, computes their weekly stats, and sends
// each user a digest email. Errors per user are logged but do not abort the
// remaining recipients.
func (s *DigestScheduler) dispatch(ctx context.Context, now time.Time) {
	// The "week" window starts 7 days before now.
	since := now.AddDate(0, 0, -7)

	users, err := s.appRepo.GetAllActiveUsersForDigest(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to fetch active users for digest")
		return
	}

	s.log.Info().Int("user_count", len(users)).Msg("sending weekly digest emails")

	sent := 0
	failed := 0

	for _, u := range users {
		if err := s.sendDigest(ctx, u, since, now); err != nil {
			s.log.Error().
				Err(err).
				Str("user_id", u.ID.String()).
				Str("email", u.Email).
				Msg("failed to send digest email")
			failed++
			continue
		}
		sent++
	}

	// Record lastSent regardless of partial failures so we don't retry
	// within the same window for the users that already received it.
	s.lastSent = now

	s.log.Info().
		Int("sent", sent).
		Int("failed", failed).
		Int("total", len(users)).
		Msg("weekly digest dispatch complete")
}

// sendDigest fetches stats for a single user and calls the notifier.
func (s *DigestScheduler) sendDigest(
	ctx context.Context,
	u *appRepo.ActiveUserDigest,
	since time.Time,
	now time.Time,
) error {
	stats, err := s.appRepo.GetWeeklyStats(ctx, u.ID, since)
	if err != nil {
		return fmt.Errorf("GetWeeklyStats: %w", err)
	}

	// Derive a friendly display name: use FullName if set, fall back to email.
	name := u.FullName
	if name == "" {
		name = u.Email
	}

	data := notification.WeeklyDigestData{
		UserName:     name,
		WeekOf:       since.Format("January 2, 2006"),
		Applied:      stats.Applied,
		Interviews:   stats.Interviews,
		Offers:       stats.Offers,
		Rejections:   stats.Rejections,
		Pending:      stats.Pending,
		DashboardURL: digestDashboardURL,
		Year:         now.Year(),
	}

	if err := s.notifier.SendWeeklyDigest(ctx, u.Email, data); err != nil {
		return fmt.Errorf("SendWeeklyDigest: %w", err)
	}
	return nil
}
