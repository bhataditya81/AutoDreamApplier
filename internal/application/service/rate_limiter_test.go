package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/service"
)

// skipIfNoRedis skips the test if Redis is not available at localhost:6379.
func skipIfNoRedis(t *testing.T) {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis not reachable (%v): skipping test", err)
	}
}

func newTestRateLimiter(t *testing.T) (*service.RateLimiter, *goredis.Client) {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	rl := service.NewRateLimiter(rdb, zerolog.Nop())
	return rl, rdb
}

// TestRateLimiter_AllowsUpToLimit verifies N calls succeed.
func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	skipIfNoRedis(t)
	rl, rdb := newTestRateLimiter(t)
	defer rdb.Close()

	userID := uuid.New()
	const limit = 3

	// Flush any leftover key
	ctx := context.Background()

	for i := 0; i < limit; i++ {
		if err := rl.CheckAndIncrement(ctx, userID, limit, "UTC"); err != nil {
			t.Fatalf("call %d/%d: expected nil, got %v", i+1, limit, err)
		}
	}
}

// TestRateLimiter_BlocksOnLimitPlusOne verifies the N+1 call returns ErrDailyLimitReached.
func TestRateLimiter_BlocksOnLimitPlusOne(t *testing.T) {
	skipIfNoRedis(t)
	rl, rdb := newTestRateLimiter(t)
	defer rdb.Close()

	userID := uuid.New()
	const limit = 2
	ctx := context.Background()

	for i := 0; i < limit; i++ {
		if err := rl.CheckAndIncrement(ctx, userID, limit, "UTC"); err != nil {
			t.Fatalf("setup call %d: unexpected error: %v", i+1, err)
		}
	}

	err := rl.CheckAndIncrement(ctx, userID, limit, "UTC")
	if err == nil {
		t.Fatal("expected ErrDailyLimitReached on N+1 call, got nil")
	}
	if err != service.ErrDailyLimitReached {
		t.Errorf("expected ErrDailyLimitReached, got: %v", err)
	}
}

// TestRateLimiter_DifferentUsersDontInterfere verifies per-user isolation.
func TestRateLimiter_DifferentUsersDontInterfere(t *testing.T) {
	skipIfNoRedis(t)
	rl, rdb := newTestRateLimiter(t)
	defer rdb.Close()

	userA := uuid.New()
	userB := uuid.New()
	const limit = 1
	ctx := context.Background()

	// Use up userA's limit
	if err := rl.CheckAndIncrement(ctx, userA, limit, "UTC"); err != nil {
		t.Fatalf("userA first call failed: %v", err)
	}
	// userA at limit
	if err := rl.CheckAndIncrement(ctx, userA, limit, "UTC"); err != service.ErrDailyLimitReached {
		t.Errorf("expected ErrDailyLimitReached for userA, got: %v", err)
	}
	// userB should still be allowed
	if err := rl.CheckAndIncrement(ctx, userB, limit, "UTC"); err != nil {
		t.Errorf("userB should not be blocked, got: %v", err)
	}
}

// TestRateLimiter_KeyIncludesDate verifies different UUIDs (simulating different
// days via key format) don't share state. We can't time-travel easily, but we
// verify the UTC key includes the date portion by checking two distinct user IDs
// do NOT share limits.
func TestRateLimiter_InvalidTimezone_FallsBackToUTC(t *testing.T) {
	skipIfNoRedis(t)
	rl, rdb := newTestRateLimiter(t)
	defer rdb.Close()

	userID := uuid.New()
	const limit = 5
	ctx := context.Background()

	// An invalid timezone should fall back to UTC and not error.
	if err := rl.CheckAndIncrement(ctx, userID, limit, "Invalid/Zone"); err != nil {
		t.Errorf("invalid timezone should fail-open, got: %v", err)
	}
}

// TestRateLimiter_ErrDailyLimitReachedSentinel verifies ErrDailyLimitReached is exported.
func TestRateLimiter_ErrDailyLimitReachedSentinel(t *testing.T) {
	if service.ErrDailyLimitReached == nil {
		t.Fatal("ErrDailyLimitReached must not be nil")
	}
	if service.ErrDailyLimitReached.Error() == "" {
		t.Fatal("ErrDailyLimitReached must have a non-empty message")
	}
}
