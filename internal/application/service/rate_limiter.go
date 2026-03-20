package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// ErrDailyLimitReached is returned when a user has hit their daily application limit.
var ErrDailyLimitReached = errors.New("daily application limit reached")

// RateLimiter enforces per-user daily application limits using Redis.
// Key format: "daily_apply:{userID}:{YYYY-MM-DD}" (date in user's timezone)
// TTL: 25 hours (safe expiry regardless of timezone offset)
type RateLimiter struct {
	rdb *goredis.Client
	log zerolog.Logger
}

// NewRateLimiter creates a RateLimiter backed by Redis.
func NewRateLimiter(rdb *goredis.Client, log zerolog.Logger) *RateLimiter {
	return &RateLimiter{
		rdb: rdb,
		log: log.With().Str("component", "rate-limiter").Logger(),
	}
}

// CheckAndIncrement atomically checks if the user is under their daily limit and
// increments the counter. Returns ErrDailyLimitReached if count >= limit.
// If Redis is unavailable, logs a warning and returns nil (fail-open).
func (r *RateLimiter) CheckAndIncrement(ctx context.Context, userID uuid.UUID, limit int, timezone string) error {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		r.log.Warn().
			Str("timezone", timezone).
			Err(err).
			Msg("invalid timezone for rate limiter; falling back to UTC")
		loc = time.UTC
	}

	today := time.Now().In(loc).Format("2006-01-02")
	key := fmt.Sprintf("daily_apply:%s:%s", userID.String(), today)
	const ttl = 25 * time.Hour

	// Use a Lua script to atomically GET, check, and INCR with TTL.
	// Returns the new count, or -1 if the limit was already reached.
	script := goredis.NewScript(`
		local key   = KEYS[1]
		local limit = tonumber(ARGV[1])
		local ttl   = tonumber(ARGV[2])

		local current = tonumber(redis.call("GET", key) or "0")
		if current >= limit then
			return -1
		end
		local new = redis.call("INCR", key)
		if new == 1 then
			redis.call("EXPIRE", key, ttl)
		end
		return new
	`)

	result, err := script.Run(ctx, r.rdb, []string{key}, limit, int(ttl.Seconds())).Int()
	if err != nil {
		r.log.Warn().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Redis rate-limit check failed; failing open")
		return nil
	}

	if result == -1 {
		return ErrDailyLimitReached
	}

	return nil
}
