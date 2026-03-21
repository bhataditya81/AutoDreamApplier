package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	userrepo "github.com/bhata/AutoDreamApplier/internal/user/repository"
)

// AutoApproveService scans pending matches for users who have auto-apply
// enabled and approves matches that meet or exceed the user's threshold.
type AutoApproveService struct {
	matchRepo *matchrepo.MatchRepository
	userRepo  *userrepo.UserRepository
	log       zerolog.Logger
}

// NewAutoApproveService creates an AutoApproveService.
func NewAutoApproveService(
	matchRepo *matchrepo.MatchRepository,
	userRepo *userrepo.UserRepository,
	log zerolog.Logger,
) *AutoApproveService {
	return &AutoApproveService{
		matchRepo: matchRepo,
		userRepo:  userRepo,
		log:       log,
	}
}

// RunForAllUsers iterates over all active users with auto-apply enabled and
// calls ProcessPendingMatches for each.
func (s *AutoApproveService) RunForAllUsers(ctx context.Context) error {
	users, err := s.userRepo.ListAutoApplyUsers(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if err := s.ProcessPendingMatches(ctx, u.ID, u.AutoThreshold); err != nil {
			s.log.Error().Err(err).Str("user_id", u.ID.String()).Msg("auto-approve failed for user")
		}
	}
	return nil
}

// ProcessPendingMatches approves all pending matches for a user that have a
// match_score >= threshold.
func (s *AutoApproveService) ProcessPendingMatches(ctx context.Context, userID uuid.UUID, threshold float64) error {
	matches, err := s.matchRepo.ListPendingMatches(ctx, userID)
	if err != nil {
		return err
	}
	approved := 0
	for _, m := range matches {
		if m.MatchScore >= threshold {
			if err := s.matchRepo.UpdateMatchStatus(ctx, m.ID, "approved"); err != nil {
				s.log.Warn().Err(err).Str("match_id", m.ID.String()).Msg("failed to auto-approve match")
				continue
			}
			approved++
		}
	}
	if approved > 0 {
		s.log.Info().
			Str("user_id", userID.String()).
			Float64("threshold", threshold).
			Int("approved", approved).
			Msg("auto-approved matches")
	}
	return nil
}

// RunEvery starts a blocking loop that calls RunForAllUsers every interval.
// It stops when ctx is cancelled.
func (s *AutoApproveService) RunEvery(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RunForAllUsers(ctx); err != nil {
				s.log.Error().Err(err).Msg("auto-approve cycle failed")
			}
		}
	}
}
