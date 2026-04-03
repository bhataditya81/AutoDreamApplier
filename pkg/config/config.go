package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	App       AppConfig
	DB        DBConfig
	Redis     RedisConfig
	AWS       AWSConfig
	Cognito   CognitoConfig
	S3        S3Config
	SES       SESConfig
	AI        AIConfig
	Browser   BrowserConfig
	RateLimit RateLimitConfig
	Scheduler SchedulerConfig
}

type AppConfig struct {
	Env           string
	Port          string
	LogLevel      string
	DevJWTSecret  string // dev-only: signs local JWTs, ignored in production
	EncryptionKey string // 32-byte hex key for AES-256-GCM credential encryption
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	TLS      bool // true when REDIS_URL uses rediss:// scheme (e.g. Upstash)
}

func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

type CognitoConfig struct {
	UserPoolID  string
	AppClientID string
	Domain      string
}

type S3Config struct {
	BucketResumes     string
	BucketScreenshots string
	Endpoint          string
}

type SESConfig struct {
	FromEmail    string
	DashboardURL string
}

type AIConfig struct {
	ServiceURL   string
	Provider     string // "python", "anthropic", "gemini", "openai"
	AnthropicKey string
	LLMModel     string // used as model name for Anthropic
	GeminiAPIKey string
	GeminiModel  string // default: gemini-1.5-flash
	OpenAIAPIKey string
	OpenAIModel  string // default: gpt-4o-mini
}

type BrowserConfig struct {
	PoolSize      int
	PoolURL       string
	MaxConcurrent int
}

type RateLimitConfig struct {
	Indeed    int
	Glassdoor int
	LinkedIn  int
}

type SchedulerConfig struct {
	TickInterval     time.Duration // how often the auto-apply scheduler wakes up
	MaxMatchesPerRun int           // max matches submitted per user per tick
}

// Load reads configuration from environment variables.
// DATABASE_URL and REDIS_URL (if set) override individual component vars,
// allowing cloud deployments to pass a single connection string.
func Load() *Config {
	cfg := &Config{
		App: AppConfig{
			Env:           getEnv("APP_ENV", "development"),
			Port:          getEnv("APP_PORT", "8080"),
			LogLevel:      getEnv("APP_LOG_LEVEL", "debug"),
			DevJWTSecret:  getEnv("DEV_JWT_SECRET", "dev-secret-change-in-production-32b"),
			EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "autodream"),
			Password:        getEnv("DB_PASSWORD", "autodream_dev"),
			Name:            getEnv("DB_NAME", "autodreamapplier"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		},
		Cognito: CognitoConfig{
			UserPoolID:  getEnv("COGNITO_USER_POOL_ID", ""),
			AppClientID: getEnv("COGNITO_APP_CLIENT_ID", ""),
			Domain:      getEnv("COGNITO_DOMAIN", ""),
		},
		S3: S3Config{
			BucketResumes:     getEnv("S3_BUCKET_RESUMES", "autodream-resumes-dev"),
			BucketScreenshots: getEnv("S3_BUCKET_SCREENSHOTS", "autodream-screenshots-dev"),
			Endpoint:          getEnv("S3_ENDPOINT", ""),
		},
		SES: SESConfig{
			FromEmail:    getEnv("SES_FROM_EMAIL", ""),
			DashboardURL: getEnv("DASHBOARD_URL", "http://localhost:3000"),
		},
		AI: AIConfig{
			ServiceURL:   getEnv("AI_SERVICE_URL", "http://localhost:8081"),
			Provider:     getEnv("AI_PROVIDER", "python"),
			AnthropicKey: getEnv("ANTHROPIC_API_KEY", ""),
			LLMModel:     getEnv("LLM_MODEL", "claude-3-haiku-20240307"),
			GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
			GeminiModel:  getEnv("GEMINI_MODEL", ""),
			OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
			OpenAIModel:  getEnv("OPENAI_MODEL", ""),
		},
		Browser: BrowserConfig{
			PoolSize:      getEnvInt("BROWSER_POOL_SIZE", 3),
			PoolURL:       getEnv("BROWSER_POOL_URL", "http://localhost:8085"),
			MaxConcurrent: getEnvInt("BROWSER_MAX_CONCURRENT", 5),
		},
		RateLimit: RateLimitConfig{
			Indeed:    getEnvInt("RATE_LIMIT_INDEED", 10),
			Glassdoor: getEnvInt("RATE_LIMIT_GLASSDOOR", 8),
			LinkedIn:  getEnvInt("RATE_LIMIT_LINKEDIN", 3),
		},
		Scheduler: SchedulerConfig{
			TickInterval:     getEnvDuration("SCHEDULER_TICK_INTERVAL", 1*time.Hour),
			MaxMatchesPerRun: getEnvInt("SCHEDULER_MAX_MATCHES_PER_RUN", 20),
		},
	}

	// DATABASE_URL / REDIS_URL override individual component vars when present.
	applyURLOverrides(cfg)
	return cfg
}

// applyURLOverrides parses DATABASE_URL and REDIS_URL (if set) and overrides
// the individual component fields. This allows Terraform / cloud deployments to
// pass a single connection string while local dev continues to use individual vars.
func applyURLOverrides(cfg *Config) {
	// DATABASE_URL: postgres://user:pass@host:5432/dbname?sslmode=require
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if u, err := url.Parse(dbURL); err == nil {
			if h := u.Hostname(); h != "" {
				cfg.DB.Host = h
			}
			if p := u.Port(); p != "" {
				cfg.DB.Port = p
			}
			if u.User != nil {
				cfg.DB.User = u.User.Username()
				if pass, ok := u.User.Password(); ok {
					cfg.DB.Password = pass
				}
			}
			if name := strings.TrimPrefix(u.Path, "/"); name != "" {
				cfg.DB.Name = name
			}
			if ssl := u.Query().Get("sslmode"); ssl != "" {
				cfg.DB.SSLMode = ssl
			} else if u.Scheme != "postgres" {
				cfg.DB.SSLMode = "require" // default for cloud DBs
			}
		}
	}

	// REDIS_URL: redis://default:pass@host:6379  OR  rediss://... (TLS)
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		if u, err := url.Parse(redisURL); err == nil {
			if h := u.Hostname(); h != "" {
				cfg.Redis.Host = h
			}
			if p := u.Port(); p != "" {
				cfg.Redis.Port = p
			}
			if u.User != nil {
				if pass, ok := u.User.Password(); ok && pass != "" {
					cfg.Redis.Password = pass
				}
			}
			// rediss:// scheme signals TLS required (e.g. Upstash)
			cfg.Redis.TLS = u.Scheme == "rediss"
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
