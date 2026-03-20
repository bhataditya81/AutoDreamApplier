package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const defaultFollowUpDays = 7

// FollowUp represents a pending follow-up action for an application.
type FollowUp struct {
	ApplicationID uuid.UUID
	JobTitle      string
	Company       string
	AppliedAt     time.Time
	DaysElapsed   int
}

// FollowUpRepository is implemented by the application repository.
type FollowUpRepository interface {
	// GetApplicationsNeedingFollowUp returns applied applications
	// where applied_at < now - followUpDays and no follow-up event exists.
	GetApplicationsNeedingFollowUp(ctx context.Context, userID uuid.UUID, followUpDays int) ([]FollowUp, error)
	// DismissFollowUp records that the user dismissed the follow-up.
	DismissFollowUp(ctx context.Context, applicationID uuid.UUID, userID uuid.UUID) error
}

// FollowUpService manages application follow-up reminders.
type FollowUpService struct {
	repo FollowUpRepository
	log  zerolog.Logger
}

// NewFollowUpService creates a FollowUpService.
func NewFollowUpService(repo FollowUpRepository, log zerolog.Logger) *FollowUpService {
	return &FollowUpService{
		repo: repo,
		log:  log.With().Str("component", "followup_service").Logger(),
	}
}

// GetPendingFollowUps returns applications that need user attention.
// Default: applications older than 7 days with no outcome recorded.
func (s *FollowUpService) GetPendingFollowUps(ctx context.Context, userID uuid.UUID) ([]FollowUp, error) {
	followUps, err := s.repo.GetApplicationsNeedingFollowUp(ctx, userID, defaultFollowUpDays)
	if err != nil {
		return nil, err
	}

	s.log.Debug().
		Str("user_id", userID.String()).
		Int("count", len(followUps)).
		Msg("fetched pending follow-ups")

	return followUps, nil
}

// DismissFollowUp marks a follow-up as dismissed so it doesn't recur.
func (s *FollowUpService) DismissFollowUp(ctx context.Context, applicationID uuid.UUID, userID uuid.UUID) error {
	if err := s.repo.DismissFollowUp(ctx, applicationID, userID); err != nil {
		return err
	}

	s.log.Info().
		Str("application_id", applicationID.String()).
		Str("user_id", userID.String()).
		Msg("follow-up dismissed")

	return nil
}
