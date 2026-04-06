package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// BoardLimits defines per-board daily application limits.
var BoardLimits = map[string]int{
	"indeed":    10,
	"glassdoor": 8,
	"linkedin":  3, // Phase 2: conservative
	"default":   5,
}

// Limiter enforces per-user, per-board rate limits using Redis.
type Limiter struct {
	rdb *redis.Client
}

// NewLimiter creates a new rate limiter backed by Redis.
func NewLimiter(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// key generates the Redis key for a user+board rate limit.
func (l *Limiter) key(userID, board string) string {
	// Key format: ratelimit:{userID}:{board}:{date}
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("ratelimit:%s:%s:%s", userID, board, date)
}

// checkAndIncrScript atomically checks the limit, increments, and sets expiry.
// Returns the new count (post-increment). If the count already meets the limit,
// the key is NOT incremented and the script returns -1.
var checkAndIncrScript = redis.NewScript(`
local key   = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl   = tonumber(ARGV[2])

local current = tonumber(redis.call("GET", key) or "0")
if current >= limit then
	return -1
end

local newCount = redis.call("INCR", key)
if newCount == 1 then
	redis.call("EXPIRE", key, ttl)
end
return newCount
`)

// Check returns whether the user can apply on this board today.
// Returns (allowed, currentCount, dailyLimit, error).
func (l *Limiter) Check(ctx context.Context, userID, board string) (bool, int, int, error) {
	limit := l.getLimit(board)

	count, err := l.rdb.Get(ctx, l.key(userID, board)).Int()
	if err == redis.Nil {
		return true, 0, limit, nil
	}
	if err != nil {
		return false, 0, limit, fmt.Errorf("check rate limit: %w", err)
	}

	return count < limit, count, limit, nil
}

// Increment atomically checks the limit and records an application.
// Returns the new count. Returns an error if the limit has been reached.
func (l *Limiter) Increment(ctx context.Context, userID, board string) (int, error) {
	key := l.key(userID, board)
	limit := l.getLimit(board)

	// TTL = seconds until end of day UTC + 1 hour buffer
	now := time.Now().UTC()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	ttlSec := int(time.Until(endOfDay)/time.Second) + 3600

	result, err := checkAndIncrScript.Run(ctx, l.rdb, []string{key}, limit, ttlSec).Int()
	if err != nil {
		return 0, fmt.Errorf("increment rate limit: %w", err)
	}
	if result == -1 {
		return limit, fmt.Errorf("rate limit reached (%d/%d)", limit, limit)
	}

	return result, nil
}

// GetDailyUsage returns the user's application count across all boards for today.
func (l *Limiter) GetDailyUsage(ctx context.Context, userID string) (map[string]int, error) {
	usage := make(map[string]int)
	date := time.Now().UTC().Format("2006-01-02")
	pattern := fmt.Sprintf("ratelimit:%s:*:%s", userID, date)

	iter := l.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		count, err := l.rdb.Get(ctx, key).Int()
		if err != nil {
			continue
		}
		// Extract board name from key
		// key format: ratelimit:{userID}:{board}:{date}
		board := extractBoard(key)
		usage[board] = count
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan rate limits: %w", err)
	}

	return usage, nil
}

// getLimit returns the daily limit for a board.
func (l *Limiter) getLimit(board string) int {
	if limit, ok := BoardLimits[board]; ok {
		return limit
	}
	return BoardLimits["default"]
}

// extractBoard extracts the board name from a Redis key.
func extractBoard(key string) string {
	// key format: ratelimit:{userID}:{board}:{date}
	// Find second and third colon positions
	colonCount := 0
	start := 0
	end := len(key)

	for i, ch := range key {
		if ch == ':' {
			colonCount++
			if colonCount == 2 {
				start = i + 1
			}
			if colonCount == 3 {
				end = i
				break
			}
		}
	}

	return key[start:end]
}
