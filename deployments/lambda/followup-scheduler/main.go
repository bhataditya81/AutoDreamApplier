package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	apprepo "github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
)

// handler runs one follow-up tick synchronously.
// EventBridge invokes this function every hour.
func handler(ctx context.Context, _ events.CloudWatchEvent) error {
	cfg := config.Load()
	log := logger.NewProduction(cfg.App.LogLevel)

	pool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	notifClient, err := notification.New(ctx, notification.Config{
		Region:          cfg.AWS.Region,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		FromEmail:       cfg.SES.FromEmail,
		DashboardURL:    cfg.SES.DashboardURL,
	}, log)
	if err != nil {
		// Non-fatal: the scheduler can still mark follow_up_sent even if email fails.
		// notification.Client methods are nil-safe.
		log.Warn().Err(err).Msg("notification client failed (non-fatal)")
	}

	appRepo := apprepo.New(pool, log)
	followUpSvc := notification.NewFollowUpService(appRepo, log)
	scheduler := notification.NewFollowUpScheduler(followUpSvc, appRepo, notifClient, log)

	log.Info().Msg("running follow-up tick")
	scheduler.Tick(ctx)
	log.Info().Msg("follow-up tick complete")
	return nil
}

func main() {
	lambda.Start(handler)
}
