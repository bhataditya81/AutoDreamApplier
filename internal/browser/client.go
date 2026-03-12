// Package browser provides an HTTP client for the Browser Pool microservice.
// The Apply Engine does NOT run Playwright directly; it delegates to the
// browser-pool service running on EC2 Spot instances.
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const applyTimeout = 180 * time.Second

// Client is an HTTP client for the browser-pool microservice.
type Client struct {
	base   string
	http   *http.Client
	log    zerolog.Logger
}

// New creates a browser pool Client pointing at baseURL.
func New(baseURL string, log zerolog.Logger) *Client {
	return &Client{
		base: baseURL,
		http: &http.Client{Timeout: applyTimeout},
		log:  log,
	}
}

// ── Request / Response types ──────────────────────────────────────────────────

// ApplyRequest is sent to the browser pool to fill and submit an ATS form.
type ApplyRequest struct {
	ApplicationID   string            `json:"application_id"`
	ApplyURL        string            `json:"apply_url"`
	ATSType         string            `json:"ats_type"`
	FullName        string            `json:"full_name"`
	Email           string            `json:"email"`
	Phone           string            `json:"phone"`
	LinkedIn        string            `json:"linkedin"`
	Website         string            `json:"website"`
	ResumeText      string            `json:"resume_text"`
	CoverLetterText string            `json:"cover_letter_text"`
	FormAnswers     map[string]string `json:"form_answers"`
}

// ApplyResponse is returned by the browser pool after an apply attempt.
type ApplyResponse struct {
	Success        bool     `json:"success"`
	ScreenshotKey  string   `json:"screenshot_key"`
	ErrorMessage   string   `json:"error_message"`
	StepsCompleted []string `json:"steps_completed"`
}

// EmergencyStopRequest halts all active browser sessions for a user.
type EmergencyStopRequest struct {
	UserID string `json:"user_id"`
}

// ── Methods ───────────────────────────────────────────────────────────────────

// Apply sends an apply request to the browser pool and returns the result.
// The call blocks up to 180 s while Playwright fills and submits the form.
func (c *Client) Apply(ctx context.Context, req *ApplyRequest) (*ApplyResponse, error) {
	var resp ApplyResponse
	if err := c.post(ctx, "/api/v1/browser/apply", req, &resp); err != nil {
		return nil, fmt.Errorf("browser apply: %w", err)
	}
	return &resp, nil
}

// EmergencyStop kills all active browser sessions for the given user.
func (c *Client) EmergencyStop(ctx context.Context, userID string) error {
	return c.post(ctx, "/api/v1/browser/emergency-stop", &EmergencyStopRequest{UserID: userID}, nil)
}

// Health checks the browser pool liveness endpoint.
func (c *Client) Health(ctx context.Context) error {
	url := c.base + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("browser pool unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.base + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("browser pool %s returned %d: %s", path, resp.StatusCode, string(raw))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response %s: %w", path, err)
		}
	}
	return nil
}
