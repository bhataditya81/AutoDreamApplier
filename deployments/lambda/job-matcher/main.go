package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
	matchsvc "github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
)

// handler runs one full matching cycle across all active users.
// EventBridge invokes this function every 2 hours (after job-discovery completes).
func handler(ctx context.Context, _ events.CloudWatchEvent) error {
	cfg := config.Load()
	log := logger.NewProduction(cfg.App.LogLevel)

	pool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	matchRepo := matchrepo.New(pool, log)
	matchSvc := matchsvc.New(pool, matchRepo, log)

	// Attach semantic scorer. Falls back to 0.5 (neutral) internally when the
	// AI service is unreachable, so matching is never blocked by it.
	embClient := embedding.New(cfg.AI.ServiceURL)
	matchSvc.WithSemanticScorer(scorer.NewSemanticScorer(embClient))

	log.Info().Msg("starting matching run")
	results, err := matchSvc.RunForAllActiveUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("matching run failed")
		return err
	}

	totalNew, totalScored := 0, 0
	for _, r := range results {
		totalNew += r.MatchesNew
		totalScored += r.JobsScored
	}
	log.Info().
		Int("users", len(results)).
		Int("total_scored", totalScored).
		Int("total_new_matches", totalNew).
		Msg("matching run complete")

	return nil
}

func main() {
	lambda.Start(handler)
}
