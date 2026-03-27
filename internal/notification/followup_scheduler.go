package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// FollowUpUserRepo abstracts the queries needed by FollowUpScheduler to
// iterate active users. Implemented by application/repository.ApplicationRepository.
type FollowUpUserRepo interface {
	// GetAllActiveUsersForFollowUp returns every active user's ID and email.
	GetAllActiveUsersForFollowUp(ctx context.Context) ([]FollowUpUser, error)
	// MarkFollowUpSent records that a follow-up email was sent for an application.
	MarkFollowUpSent(ctx context.Context, applicationID uuid.UUID) error
}

// FollowUpUser is a minimal user projection used by the follow-up scheduler.
type FollowUpUser struct {
	ID       uuid.UUID
	Email    string
	FullName string
}

// followUpCheckInterval is how often the scheduler checks for pending follow-ups.
const followUpCheckInterval = 1 * time.Hour

// followUpDashboardURL is embedded in the reminder email CTA.
const followUpDashboardURL = "https://app.autodreamapplier.com/dashboard/applications"

// FollowUpScheduler sends weekly follow-up reminders for stale applications.
// It runs on a 1-hour tick and emails users about applications that have been
// in the 'applied' state for 7+ days with no recorded outcome.
type FollowUpScheduler struct {
	followupSvc *FollowUpService
	userRepo    FollowUpUserRepo
	emailClient *Client // SES client — may be nil in dev
	log         zerolog.Logger
}

// NewFollowUpScheduler creates a FollowUpScheduler.
// emailClient may be nil; all send methods are nil-safe (no-op).
func NewFollowUpScheduler(
	fs *FollowUpService,
	userRepo FollowUpUserRepo,
	ec *Client,
	log zerolog.Logger,
) *FollowUpScheduler {
	return &FollowUpScheduler{
		followupSvc: fs,
		userRepo:    userRepo,
		emailClient: ec,
		log:         log.With().Str("component", "followup_scheduler").Logger(),
	}
}

// Tick runs a single follow-up check synchronously.
// Called directly by the Lambda handler (deployments/lambda/followup-scheduler)
// instead of the long-running Run loop.
func (s *FollowUpScheduler) Tick(ctx context.Context) {
	s.tick(ctx)
}

// Run starts the scheduler loop. Checks immediately then every hour.
// Blocks until ctx is cancelled — call via go scheduler.Run(ctx).
func (s *FollowUpScheduler) Run(ctx context.Context) {
	s.log.Info().
		Str("check_interval", followUpCheckInterval.String()).
		Msg("follow-up scheduler started")

	// Run once immediately so a fresh deploy doesn't wait up to an hour.
	s.tick(ctx)

	ticker := time.NewTicker(followUpCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("follow-up scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick fetches all active users and dispatches follow-up emails as needed.
func (s *FollowUpScheduler) tick(ctx context.Context) {
	users, err := s.userRepo.GetAllActiveUsersForFollowUp(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to fetch active users for follow-up check")
		return
	}

	if len(users) == 0 {
		return
	}

	s.log.Debug().Int("users", len(users)).Msg("checking follow-up reminders")

	sent := 0
	failed := 0

	for _, u := range users {
		n, f := s.processUser(ctx, u)
		sent += n
		failed += f
	}

	if sent > 0 || failed > 0 {
		s.log.Info().
			Int("sent", sent).
			Int("failed", failed).
			Msg("follow-up tick complete")
	}
}

// processUser sends follow-up emails for one user's pending applications.
// Returns counts of (sent, failed).
func (s *FollowUpScheduler) processUser(ctx context.Context, u FollowUpUser) (sent, failed int) {
	followUps, err := s.followupSvc.GetPendingFollowUps(ctx, u.ID)
	if err != nil {
		s.log.Error().Err(err).Str("user_id", u.ID.String()).Msg("failed to get pending follow-ups")
		failed++
		return
	}

	for _, fu := range followUps {
		if err := s.sendFollowUp(ctx, u, fu); err != nil {
			s.log.Error().
				Err(err).
				Str("user_id", u.ID.String()).
				Str("application_id", fu.ApplicationID.String()).
				Msg("failed to send follow-up email")
			failed++
			continue
		}

		// Mark the application so we don't re-send.
		if err := s.userRepo.MarkFollowUpSent(ctx, fu.ApplicationID); err != nil {
			s.log.Warn().
				Err(err).
				Str("application_id", fu.ApplicationID.String()).
				Msg("failed to mark follow-up as sent")
		}

		sent++
	}
	return
}

// sendFollowUp composes and dispatches a single follow-up reminder email.
func (s *FollowUpScheduler) sendFollowUp(ctx context.Context, u FollowUpUser, fu FollowUp) error {
	name := u.FullName
	if name == "" {
		name = u.Email
	}

	subject := fmt.Sprintf("Following up on your application at %s", fu.Company)

	data := followUpEmailData{
		UserName:     name,
		JobTitle:     fu.JobTitle,
		Company:      fu.Company,
		DaysElapsed:  fu.DaysElapsed,
		DashboardURL: followUpDashboardURL,
		Year:         time.Now().Year(),
	}

	return s.emailClient.sendFollowUpReminder(ctx, u.Email, subject, data)
}

// followUpEmailData is the template context for the follow-up reminder email.
type followUpEmailData struct {
	UserName     string
	JobTitle     string
	Company      string
	DaysElapsed  int
	DashboardURL string
	Year         int
}

// sendFollowUpReminder sends a follow-up reminder email.
// It is a nil-safe method on *Client — when c is nil it returns nil.
func (c *Client) sendFollowUpReminder(ctx context.Context, toEmail, subject string, data followUpEmailData) error {
	if c == nil {
		return nil
	}
	return c.send(ctx, toEmail, subject, tmplFollowUpReminder, data)
}
