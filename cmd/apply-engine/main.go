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
	"github.com/hibiken/asynq"

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
		Provider:     cfg.AI.Provider,
		ServiceURL:   cfg.AI.ServiceURL,
		AnthropicKey: cfg.AI.AnthropicKey,
		AnthropicModel: cfg.AI.LLMModel,
		GeminiAPIKey: cfg.AI.GeminiAPIKey,
		GeminiModel:  cfg.AI.GeminiModel,
		OpenAIAPIKey: cfg.AI.OpenAIAPIKey,
		OpenAIModel:  cfg.AI.OpenAIModel,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise AI provider")
	}
	log.Info().Str("provider", cfg.AI.Provider).Msg("AI provider ready")

	// Browser pool microservice: EC2 Spot nodes running Playwright via HTTP API.
	browserClient := browser.New(cfg.Browser.PoolURL, log)

	// ── Redis / Asynq ─────────────────────────────────────────────────────────
	// redisOpt is shared between the Asynq producer (client) and consumer (server)
	// so both always connect to the same Redis instance.
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

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

	// ── Task workers ──────────────────────────────────────────────────────────
	aiPrepWorker := workers.NewAIPrepWorker(
		repo, aiClient, s3Client, asynqClient, buckets, log,
	)
	browserApplyWorker := workers.NewBrowserApplyWorker(
		repo, browserClient, s3Client, buckets, atsRegistry, notifier, log,
	)

	// ── Service & HTTP handler ────────────────────────────────────────────────
	svc := service.New(repo, asynqClient, browserClient, notifier, log)

	// ── Auto-apply scheduler ──────────────────────────────────────────────────
	sched := scheduler.New(repo, mRepo, svc, log)

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
		WriteTimeout: 120 * time.Second, // allow long-running apply responses
		IdleTimeout:  60 * time.Second,
	}

	// ── Asynq task server ─────────────────────────────────────────────────────
	// Concurrency is sized to the browser pool: Stage 2 tasks are the bottleneck
	// (one Playwright session each). Stage 1 (AI prep) tasks are CPU/network-bound
	// and can share the remaining worker slots.
	concurrency := cfg.Browser.PoolSize
	if concurrency <= 0 {
		concurrency = 10 // sensible default when not configured
	}

	asynqSrv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				tasks.QueueAIPrep:       6, // Stage 1: higher scheduling weight
				tasks.QueueBrowserApply: 3, // Stage 2: lower concurrency (browser-bound)
				tasks.QueueDefault:      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeAIPrep, aiPrepWorker.ProcessTask)
	mux.HandleFunc(tasks.TypeBrowserApply, browserApplyWorker.ProcessTask)

	// ── Start servers and scheduler concurrently ─────────────────────────────
	// errCh receives fatal startup/runtime errors from either server so the
	// main goroutine can initiate a clean shutdown.
	errCh := make(chan error, 2)

	go func() {
		log.Info().Str("port", port).Msg("HTTP server listening")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	go func() {
		log.Info().Msg("Asynq task server starting")
		if err := asynqSrv.Run(mux); err != nil {
			errCh <- fmt.Errorf("Asynq server: %w", err)
		}
	}()

	// Auto-apply scheduler: runs until ctx is cancelled (clean shutdown below).
	go sched.Run(ctx)

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

	// Signal the scheduler to stop before closing connections.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	// Stop the Asynq server first so no new tasks are picked up while
	// in-flight tasks are allowed to finish (up to their own deadlines).
	asynqSrv.Shutdown()

	// Drain HTTP connections before closing the database pool.
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server forced shutdown")
	}

	log.Info().Msg("Application Engine stopped")
}
