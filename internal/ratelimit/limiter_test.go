package ratelimit_test

import (
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/ratelimit"
)

// ── BoardLimits ───────────────────────────────────────────────────────────────

func TestBoardLimits_KnownBoards(t *testing.T) {
	t.Parallel()

	cases := []struct {
		board string
		want  int
	}{
		{"indeed", 10},
		{"glassdoor", 8},
		{"linkedin", 3},
		{"default", 5},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.board, func(t *testing.T) {
			t.Parallel()
			got, ok := ratelimit.BoardLimits[tc.board]
			if !ok {
				t.Fatalf("BoardLimits[%q] missing", tc.board)
			}
			if got != tc.want {
				t.Errorf("BoardLimits[%q] = %d; want %d", tc.board, got, tc.want)
			}
		})
	}
}

func TestBoardLimits_AllLimitsPositive(t *testing.T) {
	t.Parallel()
	for board, limit := range ratelimit.BoardLimits {
		if limit <= 0 {
			t.Errorf("BoardLimits[%q] = %d; must be positive", board, limit)
		}
	}
}

func TestBoardLimits_DefaultExists(t *testing.T) {
	t.Parallel()
	if _, ok := ratelimit.BoardLimits["default"]; !ok {
		t.Error("BoardLimits must contain a 'default' entry")
	}
}

func TestBoardLimits_UnknownBoard_FallsBackToDefault(t *testing.T) {
	t.Parallel()
	// Simulate the getLimit logic: unknown boards should use the default.
	// We can't call getLimit directly (unexported), but we can verify
	// that no board named "unknownboard" is in the map (so callers fall back to default).
	defaultLimit := ratelimit.BoardLimits["default"]
	if defaultLimit <= 0 {
		t.Fatalf("default limit must be positive, got %d", defaultLimit)
	}

	if _, exists := ratelimit.BoardLimits["unknownboard"]; exists {
		t.Error("'unknownboard' should not be a registered board")
	}
}

// ── NewLimiter ────────────────────────────────────────────────────────────────

func TestNewLimiter_NotNil(t *testing.T) {
	t.Parallel()
	// We can't create a real redis.Client without Redis, but we can verify
	// that NewLimiter accepts a nil client without panicking (the struct is
	// created lazily — actual calls will panic on nil rdb, but construction won't).
	// Instead, verify the exported constructor signature is callable.
	//
	// Since redis.Client is a concrete struct, we pass nil to verify no panic on construction.
	l := ratelimit.NewLimiter(nil)
	if l == nil {
		t.Fatal("NewLimiter returned nil")
	}
}

// ── BoardLimits coverage ──────────────────────────────────────────────────────

func TestBoardLimits_IndeedHigherThanLinkedIn(t *testing.T) {
	t.Parallel()
	indeed := ratelimit.BoardLimits["indeed"]
	linkedin := ratelimit.BoardLimits["linkedin"]
	if indeed <= linkedin {
		t.Errorf("indeed limit (%d) should be higher than linkedin limit (%d)", indeed, linkedin)
	}
}

func TestBoardLimits_GlassdoorHigherThanLinkedIn(t *testing.T) {
	t.Parallel()
	glassdoor := ratelimit.BoardLimits["glassdoor"]
	linkedin := ratelimit.BoardLimits["linkedin"]
	if glassdoor <= linkedin {
		t.Errorf("glassdoor limit (%d) should be higher than linkedin limit (%d)", glassdoor, linkedin)
	}
}
