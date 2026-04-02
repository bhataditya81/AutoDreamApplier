package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	aimodels "github.com/bhata/AutoDreamApplier/internal/ai"
	"github.com/bhata/AutoDreamApplier/internal/application/handler"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/scheduler"
	"github.com/bhata/AutoDreamApplier/internal/application/service"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/application/workers"
	"github.com/bhata/AutoDreamApplier/internal/ats"
	atsplugins "github.com/bhata/AutoDreamApplier/internal/ats/plugins"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
	"github.com/bhata/AutoDreamApplier/pkg/middleware"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

func main() {
	// ── Config & Logger ───────────────────────────────────────────────────────
	cfg := config.Load()

	// Use structured JSON logger in production, pretty console logger elsewhere.
	log := logger.New(cfg.App.LogLevel)
	if cfg.App.Env == "production" {
		log = logger.NewProduction(cfg.App.LogLevel)
	}

	log.Info().
		Str("service", "apply-engine").
		Str("env", cfg.App.Env).
		Msg("starting Application Engine")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL")
	}
	defer database.Close(pool)

	// ── River migrations ──────────────────────────────────────────────────────
	// Ensure River's internal job-queue tables exist before starting workers.
	// This is idempotent — safe to run on every startup.
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create River migrator")
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		log.Fatal().Err(err).Msg("River migration failed")
	}
	log.Info().Msg("River migrations applied")

	// ── S3 ────────────────────────────────────────────────────────────────────
	s3Client, err := pkgs3.New(ctx, cfg.S3, cfg.AWS, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise S3 client")
	}

	// ── Notification (AWS SES) ────────────────────────────────────────────────
	// Returns nil when SES_FROM_EMAIL is unset; all notification calls become
	// no-ops so local dev works without AWS credentials.
	notifier, err := notification.New(ctx, notification.Config{
		Region:          cfg.AWS.Region,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		FromEmail:       cfg.SES.FromEmail,
		DashboardURL:    cfg.SES.DashboardURL,
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise SES notification client")
	}
	if notifier == nil {
		log.Warn().Msg("SES notifications disabled (SES_FROM_EMAIL not set)")
	} else {
		log.Info().Str("from", cfg.SES.FromEmail).Msg("SES notification client ready")
	}

	// ── External service clients ──────────────────────────────────────────────
	// AI provider: selected via AI_PROVIDER env var ("python", "anthropic", "gemini", "openai").
	// Defaults to "python" (the Python FastAPI AI service) when unset.
	aiClient, err := aimodels.NewProvider(aimodels.ProviderConfig{
		Provider:       cfg.AI.Provider,
		ServiceURL:     cfg.AI.ServiceURL,
		AnthropicKey:   cfg.AI.AnthropicKey,
		AnthropicModel: cfg.AI.LLMModel,
		GeminiAPIKey:   cfg.AI.GeminiAPIKey,
		GeminiModel:    cfg.AI.GeminiModel,
		OpenAIAPIKey:   cfg.AI.OpenAIAPIKey,
		OpenAIModel:    cfg.AI.OpenAIModel,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise AI provider")
	}
	log.Info().Str("provider", cfg.AI.Provider).Msg("AI provider ready")

	// Browser pool microservice: EC2 Spot nodes running Playwright via HTTP API.
	browserClient := browser.New(cfg.Browser.PoolURL, log)

	// ── Repository ────────────────────────────────────────────────────────────
	repo := repository.New(pool, log)
	mRepo := matchrepo.New(pool, log)

	// ── S3 bucket names ───────────────────────────────────────────────────────
	buckets := workers.S3Buckets{
		Resumes:     cfg.S3.BucketResumes,
		Screenshots: cfg.S3.BucketScreenshots,
	}

	// ── ATS Plugin Registry ────────────────────────────────────────────────────
	// Register all supported ATS plugins. The registry is passed to the
	// BrowserApplyWorker so it can gate unsupported ATS types before a browser
	// slot is acquired.
	atsRegistry := ats.NewRegistry(log)
	atsRegistry.Register(atsplugins.NewGreenhousePlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewLeverPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewWorkdayPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewICIMSPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewTaleoPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewSuccessFactorsPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewSmartRecruitersPlugin(browserClient, log))
	atsRegistry.Register(atsplugins.NewBambooHRPlugin(browserClient, log))
	// Generic fallback MUST be registered last — it matches any URL.
	atsRegistry.Register(atsplugins.NewGenericPlugin(browserClient, log))

	// ── Webhook notifications (Slack / Discord) ───────────────────────────────
	// WebhookService is always created; it no-ops when the user hasn't
	// configured any webhook URLs in their preferences.
	webhookSvc := notification.NewWebhookService(log)

	// ── River workers ─────────────────────────────────────────────────────────
	// Workers are registered before the client is created so River can validate
	// that every enqueued job kind has a registered handler.
	workerRegistry := river.NewWorkers()

	aiPrepWorker := workers.NewAIPrepWorker(
		repo, aiClient, s3Client, nil /* riverClient set below */, buckets, log,
	).WithWebhookService(webhookSvc)
	browserApplyWorker := workers.NewBrowserApplyWorker(
		repo, browserClient, s3Client, buckets, atsRegistry, notifier, log,
	).WithWebhookService(webhookSvc, cfg.SES.DashboardURL)

	river.AddWorker(workerRegistry, aiPrepWorker)
	river.AddWorker(workerRegistry, browserApplyWorker)

	// ── River client ──────────────────────────────────────────────────────────
	// Concurrency is sized to the browser pool.
	// Stage 1 (AI prep) workers get more slots than Stage 2 (browser-bound).
	//
	// FetchPollInterval is set to 24 h — River uses PostgreSQL LISTEN/NOTIFY
	// for push-based wakeup, so the fallback poll almost never fires.
	// This eliminates all idle database polling.
	concurrency := cfg.Browser.PoolSize
	if concurrency <= 0 {
		concurrency = 10
	}
	if cfg.App.Env != "production" {
		concurrency = 1 // single worker slot is enough for staging
	}

	// Compute per-queue worker counts. Use proportional allocation but guarantee
	// at least 1 worker per queue so River doesn't panic on small concurrency
	// values (e.g. concurrency=1 in staging: 1*3/10 = 0 in integer division).
	aiWorkers := concurrency * 6 / 10
	if aiWorkers < 1 {
		aiWorkers = 1
	}
	browserWorkers := concurrency * 3 / 10
	if browserWorkers < 1 {
		browserWorkers = 1
	}

	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Workers: workerRegistry,
		Queues: map[string]river.QueueConfig{
			tasks.QueueAIPrep:       {MaxWorkers: aiWorkers},
			tasks.QueueBrowserApply: {MaxWorkers: browserWorkers},
		},
		// Fallback poll interval — LISTEN/NOTIFY wakes workers instantly.
		// Set high to eliminate background database polling.
		FetchPollInterval: 24 * time.Hour,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create River client")
	}

	// Inject the River client into aiPrepWorker so it can enqueue Stage 2 jobs.
	aiPrepWorker.SetRiverClient(riverClient)

	// ── Service & HTTP handler ────────────────────────────────────────────────
	svc := service.New(repo, riverClient, browserClient, notifier, log)

	// ── Auto-apply scheduler ──────────────────────────────────────────────────
	sched := scheduler.New(repo, mRepo, svc, log,
		cfg.Scheduler.TickInterval,
		cfg.Scheduler.MaxMatchesPerRun,
	)

	// ── Weekly digest scheduler ───────────────────────────────────────────────
	// Sends a summary email to every active user on Mondays at 09:00 UTC.
	digestSched := scheduler.NewDigestScheduler(repo, notifier, log)

	// ── HTTP router ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Use(chimw.Timeout(120 * time.Second)) // generous timeout for apply endpoints

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"apply-engine"}`)
	})

	r.Mount("/api/v1/applications", handler.Router(svc, log))

	port := os.Getenv("APPLY_ENGINE_PORT")
	if port == "" {
		port = "8084"
	}

	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Start servers and scheduler concurrently ─────────────────────────────
	errCh := make(chan error, 2)

	go func() {
		log.Info().Str("port", port).Msg("HTTP server listening")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	go func() {
		log.Info().Msg("River worker starting")
		if err := riverClient.Start(ctx); err != nil {
			errCh <- fmt.Errorf("River worker: %w", err)
		}
	}()

	// Auto-apply scheduler: runs until ctx is cancelled.
	go sched.Run(ctx)

	// Weekly digest scheduler: same lifecycle.
	go digestSched.Run(ctx)

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-errCh:
		log.Error().Err(err).Msg("server error; initiating shutdown")
	}

	log.Info().Msg("shutting down Application Engine...")

	// Signal the schedulers to stop.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	// Stop River: waits for in-flight jobs to finish (up to shutdownCtx deadline).
	if err := riverClient.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("River worker forced shutdown")
	}

	// Drain HTTP connections.
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server forced shutdown")
	}

	log.Info().Msg("Application Engine stopped")
}
