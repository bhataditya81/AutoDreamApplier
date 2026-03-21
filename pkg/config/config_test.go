package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/bhata/AutoDreamApplier/pkg/config"
)

// setenv sets an env var and returns a cleanup function that restores the original.
func setenv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

// unsetenv unsets an env var and restores it on cleanup.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(key, prev)
		}
	})
}

// ── Load with defaults (no env vars set) ─────────────────────────────────────

func TestLoad_Defaults(t *testing.T) {
	// Unset all env vars that could bleed in from the shell
	vars := []string{
		"APP_ENV", "APP_PORT", "APP_LOG_LEVEL",
		"DEV_JWT_SECRET", "ENCRYPTION_KEY",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE",
		"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"AWS_REGION",
		"S3_BUCKET_RESUMES", "S3_BUCKET_SCREENSHOTS",
		"AI_SERVICE_URL", "AI_PROVIDER", "LLM_MODEL",
		"BROWSER_POOL_SIZE", "BROWSER_POOL_URL", "BROWSER_MAX_CONCURRENT",
		"RATE_LIMIT_INDEED", "RATE_LIMIT_GLASSDOOR", "RATE_LIMIT_LINKEDIN",
		"DASHBOARD_URL",
	}
	for _, v := range vars {
		unsetenv(t, v)
	}

	cfg := config.Load()

	// App
	if cfg.App.Env != "development" {
		t.Errorf("App.Env = %q; want 'development'", cfg.App.Env)
	}
	if cfg.App.Port != "8080" {
		t.Errorf("App.Port = %q; want '8080'", cfg.App.Port)
	}
	if cfg.App.LogLevel != "debug" {
		t.Errorf("App.LogLevel = %q; want 'debug'", cfg.App.LogLevel)
	}
	if cfg.App.DevJWTSecret == "" {
		t.Error("App.DevJWTSecret default must not be empty")
	}

	// DB
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q; want 'localhost'", cfg.DB.Host)
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("DB.Port = %q; want '5432'", cfg.DB.Port)
	}
	if cfg.DB.User != "autodream" {
		t.Errorf("DB.User = %q; want 'autodream'", cfg.DB.User)
	}
	if cfg.DB.Name != "autodreamapplier" {
		t.Errorf("DB.Name = %q; want 'autodreamapplier'", cfg.DB.Name)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Errorf("DB.SSLMode = %q; want 'disable'", cfg.DB.SSLMode)
	}
	if cfg.DB.MaxOpenConns != 25 {
		t.Errorf("DB.MaxOpenConns = %d; want 25", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns != 5 {
		t.Errorf("DB.MaxIdleConns = %d; want 5", cfg.DB.MaxIdleConns)
	}
	if cfg.DB.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("DB.ConnMaxLifetime = %v; want 5m", cfg.DB.ConnMaxLifetime)
	}

	// Redis
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Redis.Host = %q; want 'localhost'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6379" {
		t.Errorf("Redis.Port = %q; want '6379'", cfg.Redis.Port)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB = %d; want 0", cfg.Redis.DB)
	}

	// AWS
	if cfg.AWS.Region != "us-east-1" {
		t.Errorf("AWS.Region = %q; want 'us-east-1'", cfg.AWS.Region)
	}

	// S3
	if cfg.S3.BucketResumes != "autodream-resumes-dev" {
		t.Errorf("S3.BucketResumes = %q; want 'autodream-resumes-dev'", cfg.S3.BucketResumes)
	}
	if cfg.S3.BucketScreenshots != "autodream-screenshots-dev" {
		t.Errorf("S3.BucketScreenshots = %q; want 'autodream-screenshots-dev'", cfg.S3.BucketScreenshots)
	}

	// AI
	if cfg.AI.ServiceURL != "http://localhost:8081" {
		t.Errorf("AI.ServiceURL = %q; want 'http://localhost:8081'", cfg.AI.ServiceURL)
	}
	if cfg.AI.Provider != "python" {
		t.Errorf("AI.Provider = %q; want 'python'", cfg.AI.Provider)
	}

	// Browser
	if cfg.Browser.PoolSize != 3 {
		t.Errorf("Browser.PoolSize = %d; want 3", cfg.Browser.PoolSize)
	}
	if cfg.Browser.MaxConcurrent != 5 {
		t.Errorf("Browser.MaxConcurrent = %d; want 5", cfg.Browser.MaxConcurrent)
	}

	// RateLimit
	if cfg.RateLimit.Indeed != 10 {
		t.Errorf("RateLimit.Indeed = %d; want 10", cfg.RateLimit.Indeed)
	}
	if cfg.RateLimit.Glassdoor != 8 {
		t.Errorf("RateLimit.Glassdoor = %d; want 8", cfg.RateLimit.Glassdoor)
	}
	if cfg.RateLimit.LinkedIn != 3 {
		t.Errorf("RateLimit.LinkedIn = %d; want 3", cfg.RateLimit.LinkedIn)
	}
}

// ── Load with env vars set ────────────────────────────────────────────────────

func TestLoad_EnvVarsOverrideDefaults(t *testing.T) {
	setenv(t, "APP_ENV", "production")
	setenv(t, "APP_PORT", "9090")
	setenv(t, "APP_LOG_LEVEL", "warn")
	setenv(t, "DB_HOST", "db.internal")
	setenv(t, "DB_PORT", "5433")
	setenv(t, "DB_USER", "pguser")
	setenv(t, "DB_NAME", "mydb")
	setenv(t, "REDIS_HOST", "redis.internal")
	setenv(t, "REDIS_PORT", "6380")
	setenv(t, "AWS_REGION", "eu-west-1")
	setenv(t, "BROWSER_POOL_SIZE", "10")
	setenv(t, "RATE_LIMIT_INDEED", "20")

	cfg := config.Load()

	if cfg.App.Env != "production" {
		t.Errorf("App.Env = %q; want 'production'", cfg.App.Env)
	}
	if cfg.App.Port != "9090" {
		t.Errorf("App.Port = %q; want '9090'", cfg.App.Port)
	}
	if cfg.App.LogLevel != "warn" {
		t.Errorf("App.LogLevel = %q; want 'warn'", cfg.App.LogLevel)
	}
	if cfg.DB.Host != "db.internal" {
		t.Errorf("DB.Host = %q; want 'db.internal'", cfg.DB.Host)
	}
	if cfg.DB.Port != "5433" {
		t.Errorf("DB.Port = %q; want '5433'", cfg.DB.Port)
	}
	if cfg.DB.User != "pguser" {
		t.Errorf("DB.User = %q; want 'pguser'", cfg.DB.User)
	}
	if cfg.DB.Name != "mydb" {
		t.Errorf("DB.Name = %q; want 'mydb'", cfg.DB.Name)
	}
	if cfg.Redis.Host != "redis.internal" {
		t.Errorf("Redis.Host = %q; want 'redis.internal'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6380" {
		t.Errorf("Redis.Port = %q; want '6380'", cfg.Redis.Port)
	}
	if cfg.AWS.Region != "eu-west-1" {
		t.Errorf("AWS.Region = %q; want 'eu-west-1'", cfg.AWS.Region)
	}
	if cfg.Browser.PoolSize != 10 {
		t.Errorf("Browser.PoolSize = %d; want 10", cfg.Browser.PoolSize)
	}
	if cfg.RateLimit.Indeed != 20 {
		t.Errorf("RateLimit.Indeed = %d; want 20", cfg.RateLimit.Indeed)
	}
}

func TestLoad_InvalidInt_FallsBackToDefault(t *testing.T) {
	setenv(t, "DB_MAX_OPEN_CONNS", "not-a-number")

	cfg := config.Load()
	if cfg.DB.MaxOpenConns != 25 {
		t.Errorf("DB.MaxOpenConns = %d; want 25 (default) when env is invalid int", cfg.DB.MaxOpenConns)
	}
}

func TestLoad_InvalidDuration_FallsBackToDefault(t *testing.T) {
	setenv(t, "DB_CONN_MAX_LIFETIME", "not-a-duration")

	cfg := config.Load()
	if cfg.DB.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("DB.ConnMaxLifetime = %v; want 5m (default) when env is invalid duration", cfg.DB.ConnMaxLifetime)
	}
}

func TestLoad_ValidDuration_IsUsed(t *testing.T) {
	setenv(t, "DB_CONN_MAX_LIFETIME", "10m")

	cfg := config.Load()
	if cfg.DB.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("DB.ConnMaxLifetime = %v; want 10m", cfg.DB.ConnMaxLifetime)
	}
}

// ── DBConfig.DSN ──────────────────────────────────────────────────────────────

func TestDBConfig_DSN_Format(t *testing.T) {
	t.Parallel()
	cfg := config.DBConfig{
		Host:     "dbhost",
		Port:     "5432",
		User:     "user",
		Password: "pass",
		Name:     "mydb",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	expectations := []string{"dbhost", "5432", "user", "pass", "mydb", "disable"}
	for _, s := range expectations {
		found := false
		for _, part := range []string{dsn} {
			if len(part) > 0 {
				found = containsStr(dsn, s)
			}
		}
		if !found {
			t.Errorf("DSN(%q) does not contain %q", dsn, s)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// ── RedisConfig.Addr ──────────────────────────────────────────────────────────

func TestRedisConfig_Addr_Format(t *testing.T) {
	t.Parallel()
	rc := config.RedisConfig{Host: "redis.host", Port: "6380"}
	got := rc.Addr()
	want := "redis.host:6380"
	if got != want {
		t.Errorf("Addr() = %q; want %q", got, want)
	}
}

func TestRedisConfig_Addr_Default(t *testing.T) {
	t.Parallel()
	rc := config.RedisConfig{Host: "localhost", Port: "6379"}
	got := rc.Addr()
	want := "localhost:6379"
	if got != want {
		t.Errorf("Addr() = %q; want %q", got, want)
	}
}

// ── Load returns non-nil ──────────────────────────────────────────────────────

func TestLoad_NotNil(t *testing.T) {
	t.Parallel()
	cfg := config.Load()
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
}
