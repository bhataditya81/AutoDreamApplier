package main

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"

	analyticshandler "github.com/bhata/AutoDreamApplier/internal/analytics"
	"github.com/bhata/AutoDreamApplier/internal/salary"

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

// adapter is initialised once in init() and reused across Lambda invocations
// (warm-start optimisation — avoids re-connecting to DB/Redis on every request).
var adapter *chiadapter.ChiLambdaV2

func init() {
	cfg := config.Load()
	log := logger.NewProduction(cfg.App.LogLevel)

	ctx := context.Background()

	dbPool, err := database.NewPostgresPool(ctx, cfg.DB, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL")
	}

	redisClient, err := pkgredis.NewClient(ctx, cfg.Redis, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}

	// River client — insert-only (no workers run inside Lambda).
	// Job insertion triggers PostgreSQL LISTEN/NOTIFY; zero Redis operations.
	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(dbPool), &river.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create River client")
	}

	s3Client, err := s3.New(ctx, cfg.S3, cfg.AWS, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize S3 client")
	}

	// Auth: HS256 dev tokens (APP_ENV != production) or RS256 Cognito (production).
	// peekAlg() in the middleware auto-routes based on the token's algorithm header.
	cognitoAuth := auth.NewCognitoAuth(cfg.Cognito, cfg.AWS.Region, log)
	if cfg.App.Env != "production" {
		cognitoAuth.WithDevSecret(cfg.App.DevJWTSecret)
	}

	userRepo := repository.NewUserRepository(dbPool, log)
	appRepo := apprepo.New(dbPool, log)
	mRepo := matchrepo.New(dbPool, log)
	analyticsRepo := analyticshandler.New(dbPool, log)
	salaryRepo := salary.NewRepository(dbPool, log)
	salarySvc := salary.NewService(salaryRepo, redisClient, log)
	browserClient := browser.New(cfg.Browser.PoolURL, log)

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

	// Note: the follow-up scheduler goroutine is deliberately omitted here.
	// It runs as its own Lambda (deployments/lambda/followup-scheduler) triggered
	// by EventBridge every hour.

	appService := appsvc.New(appRepo, riverClient, browserClient, notifClient, log)
	matchService := matchsvc.New(dbPool, mRepo, log)

	userHandler := handlers.NewUserHandler(userRepo, s3Client, cfg.S3.BucketResumes, log).
		WithEncryptionKey(cfg.App.EncryptionKey)
	mHandler := matchhandler.New(matchService, mRepo, log)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Use(middleware.CORS([]string{
		"https://autodreamapplier.vercel.app",
		"https://autodreamapplier.com",
	}))
	// 28 s keeps us safely under Lambda's default 30 s function timeout.
	r.Use(chimiddleware.Timeout(28 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "api-gateway-lambda",
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		// Dev auth routes — enabled when APP_ENV != production.
		// Allows email/password login without Cognito during staging/testing.
		if cfg.App.Env != "production" {
			devAuth := auth.NewDevAuthHandler(userRepo, cfg.App.DevJWTSecret, log)
			r.Post("/auth/login", devAuth.Login)
			r.Post("/auth/register", devAuth.Register)
		}

		r.Group(func(r chi.Router) {
			r.Use(cognitoAuth.Middleware())

			r.Mount("/users", userHandler.Routes())
			r.Route("/matches", mHandler.Routes)
			r.Mount("/applications", apphandler.New(appService, log).WithUserResolver(userRepo).Routes())
			r.Mount("/analytics", analyticshandler.NewHandler(analyticsRepo, log).WithUserResolver(userRepo).Routes())
			r.Mount("/salary", salary.NewHandler(salarySvc, log).Routes())
		})
	})

	// salarySvc holds a reference to redisClient — suppress any unused-var lint.
	_ = salarySvc

	adapter = chiadapter.NewV2(r)
}

// Handler is the Lambda entry point. API Gateway HTTP API (payload format v2)
// invokes this for every incoming HTTP request.
func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return adapter.ProxyWithContextV2(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
