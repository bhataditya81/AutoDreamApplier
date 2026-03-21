package notification_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/notification"
)

func newWebhookService() *notification.WebhookService {
	return notification.NewWebhookService(zerolog.Nop())
}

// ─── Send — happy path ────────────────────────────────────────────────────────

func TestWebhookService_Send_SlackHappyPath(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()
	cfg := notification.WebhookConfig{
		SlackURL: srv.URL,
		Events:   []notification.WebhookEvent{notification.EventApplicationSubmitted},
	}
	s.Send(context.Background(), cfg, notification.EventApplicationSubmitted, map[string]string{
		"title": "Engineer", "company": "ACME", "dashboard_url": "https://app.example.com",
	})

	// Give the goroutine time to deliver.
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) == 0 {
		t.Error("expected at least one POST to the webhook server")
	}
}

// TestWebhookService_Send_EventNotEnabled ensures events not in cfg.Events are not sent.
func TestWebhookService_Send_EventNotEnabled(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()
	cfg := notification.WebhookConfig{
		SlackURL: srv.URL,
		Events:   []notification.WebhookEvent{notification.EventNewMatch}, // only new_match enabled
	}
	// Send application_submitted — should be filtered out
	s.Send(context.Background(), cfg, notification.EventApplicationSubmitted, nil)

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&received) != 0 {
		t.Error("expected no POST when event is not enabled")
	}
}

// TestWebhookService_Send_EmptyURLs ensures no HTTP calls are made for empty URLs.
func TestWebhookService_Send_EmptyURLs(t *testing.T) {
	s := newWebhookService()
	cfg := notification.WebhookConfig{
		SlackURL:   "",
		DiscordURL: "",
		Events:     []notification.WebhookEvent{notification.EventApplicationSubmitted},
	}
	// Should not panic or error
	s.Send(context.Background(), cfg, notification.EventApplicationSubmitted, map[string]string{
		"title": "Engineer", "company": "ACME",
	})
	time.Sleep(50 * time.Millisecond)
	// No assertion needed — just ensure no panic
}

// TestWebhookService_Send_DiscordHappyPath verifies Discord URL also gets posted.
func TestWebhookService_Send_DiscordHappyPath(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()
	cfg := notification.WebhookConfig{
		DiscordURL: srv.URL,
		Events:     []notification.WebhookEvent{notification.EventApplicationFailed},
	}
	s.Send(context.Background(), cfg, notification.EventApplicationFailed, map[string]string{
		"title": "Engineer", "company": "ACME", "error": "timeout",
	})

	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&received) == 0 {
		t.Error("expected at least one POST to discord webhook")
	}
}

// TestWebhookService_Send_BothURLs ensures both Slack and Discord are notified.
func TestWebhookService_Send_BothURLs(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()
	cfg := notification.WebhookConfig{
		SlackURL:   srv.URL,
		DiscordURL: srv.URL,
		Events:     []notification.WebhookEvent{notification.EventNewMatch},
	}
	s.Send(context.Background(), cfg, notification.EventNewMatch, map[string]string{
		"title": "Engineer", "company": "ACME", "score": "85",
	})

	time.Sleep(150 * time.Millisecond)
	if got := atomic.LoadInt32(&received); got < 2 {
		t.Errorf("expected at least 2 POSTs (slack + discord), got %d", got)
	}
}

// TestWebhookService_Send_Non2xx_DoesNotPanic verifies fire-and-forget: non-2xx
// responses do not bubble up errors to the caller.
// We test by having the server return 500 and verifying the caller returns
// immediately (the goroutine handles the retry internally).
func TestWebhookService_Send_Non2xx_DoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newWebhookService()
	cfg := notification.WebhookConfig{
		SlackURL: srv.URL,
		Events:   []notification.WebhookEvent{notification.EventDailyLimitReached},
	}

	done := make(chan struct{})
	go func() {
		// Send must return immediately even if server is non-2xx
		s.Send(context.Background(), cfg, notification.EventDailyLimitReached, map[string]string{
			"limit": "10",
		})
		close(done)
	}()

	select {
	case <-done:
		// Good — returned without blocking
	case <-time.After(500 * time.Millisecond):
		t.Error("Send blocked for too long")
	}
}

// ─── buildPayload event coverage ─────────────────────────────────────────────

// TestWebhookService_Send_AllEvents ensures each event type is dispatched without panic.
func TestWebhookService_Send_AllEvents(t *testing.T) {
	events := []notification.WebhookEvent{
		notification.EventNewMatch,
		notification.EventApplicationSubmitted,
		notification.EventApplicationFailed,
		notification.EventDailyLimitReached,
	}

	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()

	for _, event := range events {
		cfg := notification.WebhookConfig{
			SlackURL: srv.URL,
			Events:   []notification.WebhookEvent{event},
		}
		s.Send(context.Background(), cfg, event, map[string]string{
			"title": "Engineer", "company": "ACME", "score": "90", "error": "none",
			"dashboard_url": "https://example.com", "limit": "5",
		})
	}

	time.Sleep(150 * time.Millisecond)
	if got := atomic.LoadInt32(&received); got < int32(len(events)) {
		t.Errorf("expected at least %d POSTs, got %d", len(events), got)
	}
}

// ─── ValidateURL ──────────────────────────────────────────────────────────────

func TestWebhookService_ValidateURL_Reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newWebhookService()
	if err := s.ValidateURL(context.Background(), srv.URL); err != nil {
		t.Fatalf("expected nil error for reachable URL, got: %v", err)
	}
}

func TestWebhookService_ValidateURL_Unreachable(t *testing.T) {
	s := newWebhookService()
	err := s.ValidateURL(context.Background(), "http://127.0.0.1:19999")
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}
}
