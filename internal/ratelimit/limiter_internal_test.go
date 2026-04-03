// Package ratelimit white-box tests for unexported helpers.
// These run in the same package to access unexported symbols.
package ratelimit

import (
	"testing"
)

// ── extractBoard ──────────────────────────────────────────────────────────────

func TestExtractBoard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key  string
		want string
	}{
		{"ratelimit:user-123:indeed:2026-03-21", "indeed"},
		{"ratelimit:user-abc:glassdoor:2026-01-01", "glassdoor"},
		{"ratelimit:user-xyz:linkedin:2025-12-31", "linkedin"},
		{"ratelimit:user-1:default:2026-03-21", "default"},
		// board name with unusual characters
		{"ratelimit:u1:my-board:2026-03-21", "my-board"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			got := extractBoard(tc.key)
			if got != tc.want {
				t.Errorf("extractBoard(%q) = %q; want %q", tc.key, got, tc.want)
			}
		})
	}
}

// ── getLimit ──────────────────────────────────────────────────────────────────

func TestGetLimit_KnownBoards(t *testing.T) {
	t.Parallel()
	l := &Limiter{}

	cases := []struct {
		board string
		want  int
	}{
		{"indeed", 10},
		{"glassdoor", 8},
		{"linkedin", 3},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.board, func(t *testing.T) {
			t.Parallel()
			got := l.getLimit(tc.board)
			if got != tc.want {
				t.Errorf("getLimit(%q) = %d; want %d", tc.board, got, tc.want)
			}
		})
	}
}

func TestGetLimit_UnknownBoard_ReturnsDefault(t *testing.T) {
	t.Parallel()
	l := &Limiter{}
	got := l.getLimit("unknownboard")
	want := BoardLimits["default"]
	if got != want {
		t.Errorf("getLimit(unknownboard) = %d; want default %d", got, want)
	}
}

// ── key format ────────────────────────────────────────────────────────────────

func TestKey_Format(t *testing.T) {
	t.Parallel()
	l := &Limiter{}

	userID := "user-abc"
	board := "indeed"
	key := l.key(userID, board)

	// Key must start with "ratelimit:"
	prefix := "ratelimit:"
	if len(key) < len(prefix) || key[:len(prefix)] != prefix {
		t.Errorf("key %q does not start with %q", key, prefix)
	}

	// Key must contain userID and board
	if !containsSubstr(key, userID) {
		t.Errorf("key %q does not contain userID %q", key, userID)
	}
	if !containsSubstr(key, board) {
		t.Errorf("key %q does not contain board %q", key, board)
	}

	// Must have exactly 3 colons (4 segments)
	colonCount := 0
	for _, ch := range key {
		if ch == ':' {
			colonCount++
		}
	}
	if colonCount != 3 {
		t.Errorf("key %q has %d colons; want 3", key, colonCount)
	}
}

func TestKey_ExtractRoundTrip(t *testing.T) {
	t.Parallel()
	l := &Limiter{}

	boards := []string{"indeed", "glassdoor", "linkedin", "default"}
	for _, board := range boards {
		board := board
		t.Run(board, func(t *testing.T) {
			t.Parallel()
			key := l.key("test-user", board)
			got := extractBoard(key)
			if got != board {
				t.Errorf("extractBoard(key(%q)) = %q; want %q (key=%q)", board, got, board, key)
			}
		})
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && s != "" &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}()
}
