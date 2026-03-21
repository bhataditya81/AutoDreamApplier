package notification_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/notification"
)

// ─── Nil client (no-op) tests ─────────────────────────────────────────────────

// TestNilClient_SendApplicationSubmitted verifies a nil *Client is safe to call.
func TestNilClient_SendApplicationSubmitted(t *testing.T) {
	var c *notification.Client // nil
	err := c.SendApplicationSubmitted(context.Background(), "user@example.com", notification.ApplicationSubmittedData{
		UserName:      "Alice",
		JobTitle:      "Software Engineer",
		Company:       "ACME",
		ApplyURL:      "https://acme.com/job/123",
		ApplicationID: "app-001",
	})
	if err != nil {
		t.Errorf("nil Client.SendApplicationSubmitted should return nil error, got: %v", err)
	}
}

func TestNilClient_SendApplicationFailed(t *testing.T) {
	var c *notification.Client
	err := c.SendApplicationFailed(context.Background(), "user@example.com", notification.ApplicationFailedData{
		UserName:     "Bob",
		JobTitle:     "Backend Engineer",
		Company:      "Corp",
		ErrorSummary: "Timeout during form fill",
	})
	if err != nil {
		t.Errorf("nil Client.SendApplicationFailed should return nil error, got: %v", err)
	}
}

func TestNilClient_SendOutcomeUpdate(t *testing.T) {
	var c *notification.Client
	err := c.SendOutcomeUpdate(context.Background(), "user@example.com", notification.OutcomeData{
		UserName:     "Carol",
		JobTitle:     "SRE",
		Company:      "BigCo",
		Outcome:      "Interview Request",
		OutcomeEmoji: "🎉",
	})
	if err != nil {
		t.Errorf("nil Client.SendOutcomeUpdate should return nil error, got: %v", err)
	}
}

func TestNilClient_SendWeeklyDigest(t *testing.T) {
	var c *notification.Client
	err := c.SendWeeklyDigest(context.Background(), "user@example.com", notification.WeeklyDigestData{
		UserName:   "Dave",
		WeekOf:     "2026-03-15",
		Applied:    10,
		Interviews: 2,
		Offers:     0,
		Rejections: 3,
		Pending:    5,
	})
	if err != nil {
		t.Errorf("nil Client.SendWeeklyDigest should return nil error, got: %v", err)
	}
}

// ─── New() with empty FromEmail returns nil (no-op) ──────────────────────────

func TestNew_EmptyFromEmail_ReturnsNil(t *testing.T) {
	c, err := notification.New(context.Background(), notification.Config{
		Region:    "us-east-1",
		FromEmail: "", // Empty → no-op
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected no error for empty FromEmail, got: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil client when FromEmail is empty, got non-nil")
	}
}

// Calling Send* on the nil client returned by New(..., empty FromEmail) should be safe.
func TestNew_EmptyFromEmail_NilSendsSafe(t *testing.T) {
	c, _ := notification.New(context.Background(), notification.Config{
		FromEmail: "",
	}, zerolog.Nop())
	// c is nil — all Send calls should be no-ops
	if err := c.SendApplicationSubmitted(context.Background(), "x@y.com", notification.ApplicationSubmittedData{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
