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
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/handler"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/repository"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/service"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
	"github.com/bhata/AutoDreamApplier/pkg/middleware"
	pkgredis "github.com/bhata/AutoDreamApplier/pkg/redis"
)

func main() {
	// Load configuration
	cfg := config.Load()
	log := logger.New(cfg.App.Env)

	log.Info().
		Str("service", "job-matcher").
		Str("env", cfg.App.Env).
		Msg("Starting Job Matcher Service")

	// Initialize database
	pool, err := database.NewPostgresPool(context.Background(), cfg.DB, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	// Initialize Redis
	rdb, err := pkgredis.NewClient(context.Background(), cfg.Redis, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer rdb.Close()

	// Wire up dependencies
	matchRepo := repository.New(pool, log)
	matchSvc := service.New(pool, matchRepo, log)

	// Attach semantic scorer using the configured AI service URL.
	// If the AI service is down, the scorer falls back to 0.5 (neutral) — matching continues.
	embClient := embedding.New(cfg.AI.ServiceURL)
	semanticScorer := scorer.NewSemanticScorer(embClient)
	matchSvc.WithSemanticScorer(semanticScorer)

	matchHandler := handler.New(matchSvc, matchRepo, log)

	// Setup router
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Use(chimw.Timeout(60 * time.Second))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"job-matcher"}`)
	})

	// Mount matching routes
	r.Route("/api/v1/matches", func(r chi.Router) {
		matchHandler.Routes(r)
	})

	// Start server
	port := os.Getenv("JOB_MATCHER_PORT")
	if port == "" {
		port = "8083"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("port", port).Msg("Job Matcher Service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Job Matcher Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Job Matcher Service stopped")
}
