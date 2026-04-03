package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
	discrepo "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/scrapers"
	discsvc "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/service"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
)

// handler runs one full discovery cycle then embeds any new jobs.
// EventBridge invokes this function every 2 hours.
func handler(ctx context.Context, _ events.CloudWatchEvent) error {
	cfg := config.Load()
	log := logger.NewProduction(cfg.App.LogLevel)

	pool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	jobRepo := discrepo.NewJobRepository(pool, log)

	// Register the LinkedIn scraper only when a residential proxy URL is configured.
	var extra []scrapers.Scraper
	if proxy := os.Getenv("LINKEDIN_PROXY_URL"); proxy != "" {
		log.Info().Str("proxy", proxy).Msg("LinkedIn scraper enabled")
		extra = append(extra, scrapers.NewLinkedInScraper(proxy))
	}

	jobSvc := discsvc.NewDiscoveryService(jobRepo, log, extra...)

	// Default discovery parameters — keywords come from the environment so that
	// the Lambda can be reconfigured without a redeploy.
	params := discsvc.DiscoverParams{
		Keywords: []string{"software engineer", "backend engineer", "golang", "python"},
		Location: "United States",
		Remote:   true,
		MaxPages: 3,
	}
	if kw := os.Getenv("DISCOVERY_KEYWORDS"); kw != "" {
		params.Keywords = []string{kw}
	}
	if loc := os.Getenv("DISCOVERY_LOCATION"); loc != "" {
		params.Location = loc
	}

	log.Info().Msg("starting discovery run")
	results := jobSvc.RunAll(ctx, params)

	totalNew, totalFound := 0, 0
	for _, r := range results {
		if r.Err != nil {
			log.Warn().Err(r.Err).Str("source", string(r.Source)).Msg("scraper reported error (non-fatal)")
		}
		totalFound += r.JobsFound
		totalNew += r.JobsNew
	}
	log.Info().Int("total_found", totalFound).Int("total_new", totalNew).Msg("discovery run complete")

	// Embed newly discovered jobs. Non-fatal: keyword matching still works without embeddings.
	embClient := embedding.New(cfg.AI.ServiceURL)
	if err := jobSvc.EmbedNewJobs(ctx, embClient); err != nil {
		log.Warn().Err(err).Msg("embedding failed (non-fatal)")
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
