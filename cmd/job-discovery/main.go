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

	dischandler "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/handler"
	discrepo "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/repository"
	discsvc "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/service"
	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/database"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
	"github.com/bhata/AutoDreamApplier/pkg/middleware"
	pkgredis "github.com/bhata/AutoDreamApplier/pkg/redis"
)

func main() {
	// Load configuration
	cfg := config.Load()
	log := logger.New(cfg.App.LogLevel)
	if cfg.App.Env == "production" {
		log = logger.NewProduction(cfg.App.LogLevel)
	}

	log.Info().
		Str("service", "job-discovery").
		Str("env", cfg.App.Env).
		Msg("Starting Job Discovery Service")

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
		fmt.Fprintf(w, `{"status":"ok","service":"job-discovery"}`)
	})

	// Initialize job discovery components
	jobRepo := discrepo.NewJobRepository(pool)
	jobSvc := discsvc.NewDiscoveryService(jobRepo, log)
	discoveryHandler := dischandler.New(jobSvc, log)

	// Mount job discovery routes
	r.Mount("/api/v1/jobs", discoveryHandler.Router())

	// Start server
	port := os.Getenv("JOB_DISCOVERY_PORT")
	if port == "" {
		port = "8082"
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
		log.Info().Str("port", port).Msg("Job Discovery Service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Job Discovery Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Job Discovery Service stopped")
}
