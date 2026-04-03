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
			if part != "" {
				found = containsStr(dsn, s)
			}
		}
		if !found {
			t.Errorf("DSN(%q) does not contain %q", dsn, s)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || sub == "" ||
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

// ── Targeted env-var parsing tests ───────────────────────────────────────────

// TestLoad_DBPort_StringParsed verifies DB_PORT is captured as a string field.
func TestLoad_DBPort_StringParsed(t *testing.T) {
	setenv(t, "DB_PORT", "5435")
	cfg := config.Load()
	if cfg.DB.Port != "5435" {
		t.Errorf("DB.Port = %q; want '5435'", cfg.DB.Port)
	}
}

// TestLoad_AppEnv_Parsed verifies APP_ENV is read and stored correctly.
func TestLoad_AppEnv_Parsed(t *testing.T) {
	setenv(t, "APP_ENV", "staging")
	cfg := config.Load()
	if cfg.App.Env != "staging" {
		t.Errorf("App.Env = %q; want 'staging'", cfg.App.Env)
	}
}

// TestLoad_LogLevel_Parsed verifies APP_LOG_LEVEL is read and stored correctly.
func TestLoad_LogLevel_Parsed(t *testing.T) {
	setenv(t, "APP_LOG_LEVEL", "info")
	cfg := config.Load()
	if cfg.App.LogLevel != "info" {
		t.Errorf("App.LogLevel = %q; want 'info'", cfg.App.LogLevel)
	}
}

// TestLoad_MaxOpenConns_IntParsed verifies that DB_MAX_OPEN_CONNS is parsed as
// an integer and stored in DB.MaxOpenConns.
func TestLoad_MaxOpenConns_IntParsed(t *testing.T) {
	setenv(t, "DB_MAX_OPEN_CONNS", "42")
	cfg := config.Load()
	if cfg.DB.MaxOpenConns != 42 {
		t.Errorf("DB.MaxOpenConns = %d; want 42", cfg.DB.MaxOpenConns)
	}
}

// TestLoad_AllVarsUnset_NoPanic verifies that calling Load() with no env vars
// set does not panic and returns a fully-populated config struct.
func TestLoad_AllVarsUnset_NoPanic(t *testing.T) {
	// Unset a representative selection of env vars to confirm the function
	// handles empty env gracefully.  (Full default coverage is in TestLoad_Defaults.)
	keysToClear := []string{
		"APP_ENV", "APP_PORT", "APP_LOG_LEVEL", "DB_HOST", "DB_PORT",
		"DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE",
		"REDIS_HOST", "REDIS_PORT", "AWS_REGION",
		"AI_SERVICE_URL", "AI_PROVIDER", "DASHBOARD_URL",
	}
	for _, k := range keysToClear {
		unsetenv(t, k)
	}

	// Must not panic.
	cfg := config.Load()
	if cfg == nil {
		t.Fatal("Load() returned nil with all vars unset")
	}
	// Spot-check a default that proves defaults were applied.
	if cfg.App.Env != "development" {
		t.Errorf("App.Env = %q; want 'development' default", cfg.App.Env)
	}
	if cfg.App.Port != "8080" {
		t.Errorf("App.Port = %q; want '8080' default", cfg.App.Port)
	}
}

// TestLoad_RateLimitLinkedIn_IntParsed verifies RATE_LIMIT_LINKEDIN is parsed
// as an integer.
func TestLoad_RateLimitLinkedIn_IntParsed(t *testing.T) {
	setenv(t, "RATE_LIMIT_LINKEDIN", "7")
	cfg := config.Load()
	if cfg.RateLimit.LinkedIn != 7 {
		t.Errorf("RateLimit.LinkedIn = %d; want 7", cfg.RateLimit.LinkedIn)
	}
}

// ── DATABASE_URL override tests ───────────────────────────────────────────────

// TestLoad_DatabaseURL_FullURL verifies that a complete postgres:// URL is parsed
// into individual DB fields, overriding any individual env vars set beforehand.
func TestLoad_DatabaseURL_FullURL(t *testing.T) {
	setenv(t, "DB_HOST", "old-host")
	setenv(t, "DB_PORT", "9999")
	setenv(t, "DB_USER", "old-user")
	setenv(t, "DB_PASSWORD", "old-pass")
	setenv(t, "DB_NAME", "old-db")
	setenv(t, "DB_SSL_MODE", "disable")
	setenv(t, "DATABASE_URL", "postgres://neonuser:s3cr3t@ep-xxx.us-east-1.aws.neon.tech:5432/autodream?sslmode=require")

	cfg := config.Load()

	if cfg.DB.Host != "ep-xxx.us-east-1.aws.neon.tech" {
		t.Errorf("DB.Host = %q; want neon host", cfg.DB.Host)
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("DB.Port = %q; want '5432'", cfg.DB.Port)
	}
	if cfg.DB.User != "neonuser" {
		t.Errorf("DB.User = %q; want 'neonuser'", cfg.DB.User)
	}
	if cfg.DB.Password != "s3cr3t" {
		t.Errorf("DB.Password = %q; want 's3cr3t'", cfg.DB.Password)
	}
	if cfg.DB.Name != "autodream" {
		t.Errorf("DB.Name = %q; want 'autodream'", cfg.DB.Name)
	}
	if cfg.DB.SSLMode != "require" {
		t.Errorf("DB.SSLMode = %q; want 'require'", cfg.DB.SSLMode)
	}
}

// TestLoad_DatabaseURL_WithoutPort verifies that a URL with no explicit port
// does not overwrite the individual-var port.
func TestLoad_DatabaseURL_WithoutPort(t *testing.T) {
	setenv(t, "DB_PORT", "5432")
	setenv(t, "DATABASE_URL", "postgres://user:pass@myhost/mydb?sslmode=require")

	cfg := config.Load()

	if cfg.DB.Host != "myhost" {
		t.Errorf("DB.Host = %q; want 'myhost'", cfg.DB.Host)
	}
	if cfg.DB.Port != "5432" {
		t.Errorf("DB.Port = %q; want '5432' (unchanged — URL had no port)", cfg.DB.Port)
	}
}

// TestLoad_DatabaseURL_WithoutSSLMode verifies that when the URL carries no
// sslmode query param, the individual DB_SSL_MODE value is preserved.
func TestLoad_DatabaseURL_WithoutSSLMode(t *testing.T) {
	setenv(t, "DB_SSL_MODE", "verify-full")
	setenv(t, "DATABASE_URL", "postgres://user:pass@host:5432/db")

	cfg := config.Load()

	if cfg.DB.SSLMode != "verify-full" {
		t.Errorf("DB.SSLMode = %q; want 'verify-full' (unchanged when URL has no sslmode)", cfg.DB.SSLMode)
	}
}

// TestLoad_DatabaseURL_SpecialCharsInPassword verifies that URL-encoded
// characters in the password are decoded correctly.
func TestLoad_DatabaseURL_SpecialCharsInPassword(t *testing.T) {
	// '@' → %40, '/' → %2F
	setenv(t, "DATABASE_URL", "postgres://user:p%40ssw%2Frd@host:5432/db?sslmode=disable")

	cfg := config.Load()

	if cfg.DB.Password != "p@ssw/rd" {
		t.Errorf("DB.Password = %q; want 'p@ssw/rd' (URL-decoded)", cfg.DB.Password)
	}
}

// TestLoad_DatabaseURL_Absent_IndividualVarsUsed verifies backward compatibility:
// when DATABASE_URL is not set, DB_* individual env vars work as before.
func TestLoad_DatabaseURL_Absent_IndividualVarsUsed(t *testing.T) {
	unsetenv(t, "DATABASE_URL")
	setenv(t, "DB_HOST", "mylocal")
	setenv(t, "DB_PORT", "5433")
	setenv(t, "DB_USER", "localuser")
	setenv(t, "DB_NAME", "localdb")
	setenv(t, "DB_SSL_MODE", "disable")

	cfg := config.Load()

	if cfg.DB.Host != "mylocal" {
		t.Errorf("DB.Host = %q; want 'mylocal'", cfg.DB.Host)
	}
	if cfg.DB.Port != "5433" {
		t.Errorf("DB.Port = %q; want '5433'", cfg.DB.Port)
	}
	if cfg.DB.User != "localuser" {
		t.Errorf("DB.User = %q; want 'localuser'", cfg.DB.User)
	}
	if cfg.DB.Name != "localdb" {
		t.Errorf("DB.Name = %q; want 'localdb'", cfg.DB.Name)
	}
}

// TestLoad_DatabaseURL_Malformed_IndividualVarsKept verifies that a malformed
// DATABASE_URL does not panic and leaves existing individual fields intact.
func TestLoad_DatabaseURL_Malformed_IndividualVarsKept(t *testing.T) {
	setenv(t, "DB_HOST", "safe-host")
	setenv(t, "DATABASE_URL", "://not a valid url %%")

	cfg := config.Load()

	if cfg.DB.Host != "safe-host" {
		t.Errorf("DB.Host = %q; want 'safe-host' (unchanged after malformed URL)", cfg.DB.Host)
	}
}

// ── REDIS_URL override tests ──────────────────────────────────────────────────

// TestLoad_RedisURL_FullURL verifies that a complete redis:// URL is parsed
// into Redis host, port, and password, overriding individual env vars.
func TestLoad_RedisURL_FullURL(t *testing.T) {
	setenv(t, "REDIS_HOST", "old-redis")
	setenv(t, "REDIS_PORT", "6000")
	setenv(t, "REDIS_PASSWORD", "old-pass")
	setenv(t, "REDIS_URL", "redis://default:upstash-token@xxx.upstash.io:6379")

	cfg := config.Load()

	if cfg.Redis.Host != "xxx.upstash.io" {
		t.Errorf("Redis.Host = %q; want 'xxx.upstash.io'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6379" {
		t.Errorf("Redis.Port = %q; want '6379'", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "upstash-token" {
		t.Errorf("Redis.Password = %q; want 'upstash-token'", cfg.Redis.Password)
	}
}

// TestLoad_RedisURL_TLSScheme verifies that a rediss:// (TLS) URL sets TLS=true.
func TestLoad_RedisURL_TLSScheme(t *testing.T) {
	setenv(t, "REDIS_URL", "rediss://default:tls-secret@secure.upstash.io:6380")

	cfg := config.Load()

	if cfg.Redis.Host != "secure.upstash.io" {
		t.Errorf("Redis.Host = %q; want 'secure.upstash.io'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6380" {
		t.Errorf("Redis.Port = %q; want '6380'", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "tls-secret" {
		t.Errorf("Redis.Password = %q; want 'tls-secret'", cfg.Redis.Password)
	}
	if !cfg.Redis.TLS {
		t.Error("Redis.TLS = false; want true for rediss:// scheme")
	}
}

// TestLoad_RedisURL_NonTLSScheme verifies that redis:// (no TLS) leaves TLS=false.
func TestLoad_RedisURL_NonTLSScheme(t *testing.T) {
	setenv(t, "REDIS_URL", "redis://default:pass@localhost:6379")

	cfg := config.Load()

	if cfg.Redis.TLS {
		t.Error("Redis.TLS = true; want false for redis:// scheme")
	}
}

// TestLoad_RedisURL_NoPassword_DoesNotClearExistingPassword verifies that a
// Redis URL with no password component does not blank out a password already
// set via REDIS_PASSWORD.
func TestLoad_RedisURL_NoPassword_DoesNotClearExistingPassword(t *testing.T) {
	setenv(t, "REDIS_PASSWORD", "local-pass")
	setenv(t, "REDIS_URL", "redis://localhost:6379")

	cfg := config.Load()

	if cfg.Redis.Password != "local-pass" {
		t.Errorf("Redis.Password = %q; want 'local-pass' (URL had no password — individual var kept)", cfg.Redis.Password)
	}
}

// TestLoad_RedisURL_Absent_IndividualVarsUsed verifies backward compatibility:
// when REDIS_URL is not set, REDIS_* individual env vars work as before.
func TestLoad_RedisURL_Absent_IndividualVarsUsed(t *testing.T) {
	unsetenv(t, "REDIS_URL")
	setenv(t, "REDIS_HOST", "my-redis")
	setenv(t, "REDIS_PORT", "6380")
	setenv(t, "REDIS_PASSWORD", "my-pass")

	cfg := config.Load()

	if cfg.Redis.Host != "my-redis" {
		t.Errorf("Redis.Host = %q; want 'my-redis'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6380" {
		t.Errorf("Redis.Port = %q; want '6380'", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "my-pass" {
		t.Errorf("Redis.Password = %q; want 'my-pass'", cfg.Redis.Password)
	}
}

// TestLoad_RedisURL_Malformed_IndividualVarsKept verifies that a malformed
// REDIS_URL does not panic and leaves individual fields intact.
func TestLoad_RedisURL_Malformed_IndividualVarsKept(t *testing.T) {
	setenv(t, "REDIS_HOST", "safe-redis")
	setenv(t, "REDIS_URL", "not a url at all %%")

	cfg := config.Load()

	if cfg.Redis.Host != "safe-redis" {
		t.Errorf("Redis.Host = %q; want 'safe-redis' (unchanged after malformed REDIS_URL)", cfg.Redis.Host)
	}
}

// TestLoad_BothURLs_BothParsed verifies that DATABASE_URL and REDIS_URL are
// processed independently without interfering with each other.
func TestLoad_BothURLs_BothParsed(t *testing.T) {
	setenv(t, "DATABASE_URL", "postgres://dbuser:dbpass@dbhost:5432/dbname?sslmode=require")
	setenv(t, "REDIS_URL", "redis://default:redispass@redishost:6379")

	cfg := config.Load()

	if cfg.DB.Host != "dbhost" {
		t.Errorf("DB.Host = %q; want 'dbhost'", cfg.DB.Host)
	}
	if cfg.DB.Password != "dbpass" {
		t.Errorf("DB.Password = %q; want 'dbpass'", cfg.DB.Password)
	}
	if cfg.Redis.Host != "redishost" {
		t.Errorf("Redis.Host = %q; want 'redishost'", cfg.Redis.Host)
	}
	if cfg.Redis.Password != "redispass" {
		t.Errorf("Redis.Password = %q; want 'redispass'", cfg.Redis.Password)
	}
}
