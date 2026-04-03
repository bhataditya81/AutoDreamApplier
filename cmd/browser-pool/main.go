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

	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/logger"
	"github.com/bhata/AutoDreamApplier/pkg/middleware"
	pkgredis "github.com/bhata/AutoDreamApplier/pkg/redis"
)

func main() {
	// Load configuration
	cfg := config.Load()
	log := logger.New(cfg.App.Env)

	log.Info().
		Str("service", "browser-pool").
		Str("env", cfg.App.Env).
		Int("max_browsers", cfg.Browser.MaxConcurrent).
		Msg("Starting Browser Pool Manager")

	// Initialize Redis (for coordination and session tracking)
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
	r.Use(chimw.Timeout(180 * time.Second)) // Long timeout for browser operations

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"browser-pool"}`)
	})

	// TODO: Mount browser pool management routes
	// r.Route("/api/v1/browsers", func(r chi.Router) {
	//     r.Get("/status", poolHandler.Status)              // Pool status (active, available, total)
	//     r.Post("/acquire", poolHandler.Acquire)             // Acquire a browser session
	//     r.Post("/{sessionID}/release", poolHandler.Release) // Release a browser session
	//     r.Post("/{sessionID}/navigate", poolHandler.Navigate) // Navigate to URL
	//     r.Post("/{sessionID}/fill", poolHandler.FillForm)    // Fill form fields
	//     r.Post("/{sessionID}/click", poolHandler.Click)      // Click element
	//     r.Post("/{sessionID}/screenshot", poolHandler.Screenshot) // Take screenshot
	//     r.Post("/emergency-stop", poolHandler.EmergencyStop)  // Kill all sessions
	// })

	// TODO: Initialize Playwright browser pool
	// - Pre-launch N Chromium instances based on cfg.Browser.MaxConcurrent
	// - Each browser gets: random fingerprint, proxy rotation, stealth config
	// - Session manager tracks active/idle/total browsers
	// - Auto-recycle browsers after N uses (anti-fingerprinting)

	// Start server
	port := os.Getenv("BROWSER_POOL_PORT")
	if port == "" {
		port = "8085"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 180 * time.Second, // Very long for browser operations
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("port", port).Msg("Browser Pool Manager listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Browser Pool Manager...")

	// Extended shutdown time to allow browser sessions to cleanly close
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// TODO: Close all Playwright browser instances before server shutdown

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Browser Pool Manager stopped")
}
