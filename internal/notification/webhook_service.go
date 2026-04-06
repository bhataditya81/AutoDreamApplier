package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// WebhookEvent identifies which application lifecycle events trigger webhooks.
type WebhookEvent string

const (
	EventNewMatch             WebhookEvent = "new_match"
	EventApplicationSubmitted WebhookEvent = "application_submitted"
	EventApplicationFailed    WebhookEvent = "application_failed"
	EventDailyLimitReached    WebhookEvent = "daily_limit_reached"
)

// WebhookConfig holds webhook URLs and enabled events for one user.
type WebhookConfig struct {
	SlackURL   string
	DiscordURL string
	Events     []WebhookEvent // which events to send
}

// WebhookPayload is the Slack Block Kit compatible payload.
// Discord supports Slack-compatible webhooks.
type WebhookPayload struct {
	Text   string        `json:"text"`
	Blocks []interface{} `json:"blocks,omitempty"`
}

// WebhookService sends Slack/Discord webhook notifications.
type WebhookService struct {
	httpClient *http.Client
	log        zerolog.Logger
}

// NewWebhookService creates a WebhookService.
func NewWebhookService(log zerolog.Logger) *WebhookService {
	return &WebhookService{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        log.With().Str("component", "webhook_service").Logger(),
	}
}

// Send sends a webhook notification for the given event.
// Non-blocking: retries up to 3× with 5s backoff in a goroutine.
// Never blocks the caller even on failure.
func (s *WebhookService) Send(ctx context.Context, cfg WebhookConfig, event WebhookEvent, data map[string]string) {
	// Check if the event is enabled for this user.
	if !s.isEventEnabled(cfg, event) {
		return
	}

	payload := s.buildPayload(event, data)

	if cfg.SlackURL != "" {
		go func() {
			// Detached context so retries survive after the worker's context is cancelled.
			detached, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.sendWithRetry(detached, cfg.SlackURL, payload); err != nil {
				s.log.Error().
					Err(err).
					Str("event", string(event)).
					Msg("slack webhook delivery failed after retries")
			}
		}()
	}

	if cfg.DiscordURL != "" {
		go func() {
			detached, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.sendWithRetry(detached, cfg.DiscordURL, payload); err != nil {
				s.log.Error().
					Err(err).
					Str("event", string(event)).
					Msg("discord webhook delivery failed after retries")
			}
		}()
	}
}

// isEventEnabled returns true if the given event is in cfg.Events.
func (s *WebhookService) isEventEnabled(cfg WebhookConfig, event WebhookEvent) bool {
	for _, e := range cfg.Events {
		if e == event {
			return true
		}
	}
	return false
}

// buildPayload constructs a Slack Block Kit message for the event.
func (s *WebhookService) buildPayload(event WebhookEvent, data map[string]string) WebhookPayload {
	get := func(key string) string {
		if v, ok := data[key]; ok {
			return v
		}
		return ""
	}

	switch event {
	case EventApplicationSubmitted:
		title := get("title")
		company := get("company")
		dashURL := get("dashboard_url")
		return WebhookPayload{
			Text: fmt.Sprintf("✅ Applied to %s at %s", title, company),
			Blocks: []interface{}{
				map[string]interface{}{
					"type": "section",
					"text": map[string]interface{}{
						"type": "mrkdwn",
						"text": fmt.Sprintf("✅ *Application Submitted*\n*Role:* %s\n*Company:* %s\n*Status:* Applied", title, company),
					},
				},
				map[string]interface{}{
					"type": "actions",
					"elements": []interface{}{
						map[string]interface{}{
							"type": "button",
							"text": map[string]interface{}{
								"type": "plain_text",
								"text": "View Application",
							},
							"url": dashURL,
						},
					},
				},
			},
		}

	case EventApplicationFailed:
		return WebhookPayload{
			Text: fmt.Sprintf("❌ Application failed: %s at %s\nReason: %s",
				get("title"), get("company"), get("error")),
		}

	case EventNewMatch:
		return WebhookPayload{
			Text: fmt.Sprintf("🎯 New job match: %s at %s (%s%% match)",
				get("title"), get("company"), get("score")),
		}

	case EventDailyLimitReached:
		return WebhookPayload{
			Text: fmt.Sprintf("⏸️ Daily application limit reached (%s applications). Resumes tomorrow.",
				get("limit")),
		}

	default:
		return WebhookPayload{
			Text: fmt.Sprintf("[%s] %v", event, data),
		}
	}
}

// sendWithRetry POSTs to a webhook URL with up to 3 retries.
func (s *WebhookService) sendWithRetry(ctx context.Context, webhookURL string, payload WebhookPayload) error {
	const maxAttempts = 3
	const backoff = 5 * time.Second

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt, err)
			s.log.Warn().
				Err(lastErr).
				Int("attempt", attempt).
				Msg("webhook POST failed, will retry")
			if attempt < maxAttempts {
				time.Sleep(backoff)
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.log.Debug().
				Str("url", webhookURL).
				Int("status", resp.StatusCode).
				Int("attempt", attempt).
				Msg("webhook delivered")
			return nil
		}

		lastErr = fmt.Errorf("attempt %d: unexpected status %d", attempt, resp.StatusCode)
		s.log.Warn().
			Int("status", resp.StatusCode).
			Int("attempt", attempt).
			Msg("webhook returned non-2xx, will retry")
		if attempt < maxAttempts {
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("webhook delivery failed after %d attempts: %w", maxAttempts, lastErr)
}

// ValidateURL checks if a webhook URL is reachable (GET with 3s timeout).
// Returns nil if reachable, error if not.
func (s *WebhookService) ValidateURL(ctx context.Context, url string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook URL unreachable: %w", err)
	}
	resp.Body.Close()

	return nil
}
