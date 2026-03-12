package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"

	apphandler "github.com/bhata/AutoDreamApplier/internal/application/handler"
	apprepo "github.com/bhata/AutoDreamApplier/internal/application/repository"
	appsvc "github.com/bhata/AutoDreamApplier/internal/application/service"

	matchhandler "github.com/bhata/AutoDreamApplier/internal/jobmatcher/handler"
	matchrepo "github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	matchsvc "github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"

	"github.com/bhata/AutoDreamApplier/internal/user/handlers"
	"github.com/bhata/AutoDreamApplier/internal/user/repository"

	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
	"github.com/bhata/AutoDreamApplier/pkg/middleware"
	pkgredis "github.com/bhata/AutoDreamApplier/pkg/redis"
	"github.com/bhata/AutoDreamApplier/pkg/response"
	"github.com/bhata/AutoDreamApplier/pkg/s3"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	var log = logger.New(cfg.App.LogLevel)
	if cfg.App.Env == "production" {
		log = logger.NewProduction(cfg.App.LogLevel)
	}

	log.Info().
		Str("env", cfg.App.Env).
		Str("port", cfg.App.Port).
		Msg("starting AutoDreamApplier API Gateway")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	dbPool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL")
	}
	defer database.Close(dbPool)

	// Initialize Redis
	redisClient, err := pkgredis.NewClient(ctx, cfg.Redis, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer pkgredis.Close(redisClient)

	// Initialize Asynq client (task queue backed by Redis)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer asynqClient.Close()

	// Initialize S3
	s3Client, err := s3.New(ctx, cfg.S3, cfg.AWS, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize S3 client")
	}

	// Initialize Cognito auth
	cognitoAuth := auth.NewCognitoAuth(cfg.Cognito, cfg.AWS.Region, log)
	if cfg.App.Env != "production" {
		cognitoAuth.WithDevSecret(cfg.App.DevJWTSecret)
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepository(dbPool, log)
	appRepo := apprepo.New(dbPool, log)
	mRepo := matchrepo.New(dbPool, log)

	// ── External service clients ───────────────────────────────────────────────
	browserClient := browser.New(cfg.Browser.PoolURL, log)

	// Initialize notification client (AWS SES).
	// Returns nil when SES_FROM_EMAIL is unset — all methods become no-ops.
	notifClient, err := notification.New(ctx, notification.Config{
		Region:          cfg.AWS.Region,
		AccessKeyID:     cfg.AWS.AccessKeyID,
		SecretAccessKey: cfg.AWS.SecretAccessKey,
		FromEmail:       cfg.SES.FromEmail,
		DashboardURL:    cfg.SES.DashboardURL,
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize notification client")
	}

	// ── Services ──────────────────────────────────────────────────────────────
	appService := appsvc.New(appRepo, asynqClient, browserClient, notifClient, log)
	matchService := matchsvc.New(dbPool, mRepo, log)

	// ── Handlers ──────────────────────────────────────────────────────────────
	userHandler := handlers.NewUserHandler(userRepo, s3Client, cfg.S3.BucketResumes, log).
		WithEncryptionKey(cfg.App.EncryptionKey)
	mHandler := matchhandler.New(matchService, mRepo, log)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Use(middleware.CORS([]string{"http://localhost:3000", "https://autodreamapplier.com"}))
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Health check (unauthenticated)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "api-gateway",
			"version": "0.1.0",
		})
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		// Dev-only auth routes (email/password login without Cognito).
		// Only registered when APP_ENV != production so they can never reach prod.
		if cfg.App.Env != "production" {
			devAuth := auth.NewDevAuthHandler(userRepo, cfg.App.DevJWTSecret, log)
			r.Post("/auth/login", devAuth.Login)
			r.Post("/auth/register", devAuth.Register)
			log.Info().Msg("dev auth routes registered (POST /api/v1/auth/login, /auth/register)")
		}

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(cognitoAuth.Middleware())

			// User routes – profile, preferences, resume upload
			r.Mount("/users", userHandler.Routes())

			// Match routes – review queue, approve/reject, feedback
			r.Route("/matches", mHandler.Routes)

			// Application routes – submit, list, outcome tracking, emergency stop
			r.Mount("/applications", apphandler.Router(appService, log))

			// Note: /jobs is served by the job-discovery service on :8082
		})
	})

	// redisClient is retained for future rate-limiter / cache middleware.
	_ = redisClient

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("shutting down gracefully...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("server shutdown error")
		}
	}()

	log.Info().Str("addr", srv.Addr).Msg("server listening")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}

	log.Info().Msg("server stopped")
}
