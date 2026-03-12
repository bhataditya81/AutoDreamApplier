// Package plugins contains ATS-specific form-filling plugins.
package plugins

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/browser"
)

// LeverPlugin implements ats.Plugin for Lever job applications.
// Lever is used by many mid-size tech companies; their apply URLs are either
// hosted at jobs.lever.co/<company>/<job-id> or embedded on company career
// pages via a canonical lever.co iframe.
//
// Like GreenhousePlugin, all Playwright automation is delegated to the
// browser-pool microservice via browser.Client; no headless browser runs here.
type LeverPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewLeverPlugin creates a Lever ATS plugin.
// browserClient must not be nil.
func NewLeverPlugin(browserClient *browser.Client, log zerolog.Logger) *LeverPlugin {
	return &LeverPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "lever").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *LeverPlugin) Name() string {
	return "lever"
}

// Detect reports whether url belongs to a Lever job board.
// Lever job URLs take one of two forms:
//   - https://jobs.lever.co/<company>/<uuid>          (canonical)
//   - https://jobs.lever.co/<company>/<uuid>/apply    (direct apply page)
func (p *LeverPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "jobs.lever.co") ||
		strings.Contains(lower, "lever.co/")
}

// ValidateURL checks if the URL is a valid Lever application URL.
func (p *LeverPlugin) ValidateURL(url string) bool {
	return p.Detect(url)
}

// Apply fills out and submits a Lever application via the browser-pool
// microservice. The apply URL may point to either the job overview page
// (jobs.lever.co/<co>/<id>) or the direct apply page (…/<id>/apply); the
// browser-pool service handles the navigation difference.
//
// Resume text: if data.ResumeFilePath is set, the file is read from disk and
// forwarded as ResumeText (plain text, AI-tailored). A missing or unreadable
// file is non-fatal — the browser pool will attempt the application without it.
//
// Application ID: data.AdditionalFields["application_id"] is preferred; a new
// UUID is generated when absent so the browser pool can correlate its logs.
func (p *LeverPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "lever",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid Lever application URL: %s", applyURL)
		return result, fmt.Errorf("lever: %s", result.ErrorMessage)
	}

	// ── Application ID ──────────────────────────────────────────────────────────
	appID := ""
	if data.AdditionalFields != nil {
		appID = data.AdditionalFields["application_id"]
	}
	if appID == "" {
		appID = uuid.NewString()
		p.log.Debug().Str("app_id", appID).Msg("no application_id in AdditionalFields; generated new UUID")
	}

	// ── Resume text ─────────────────────────────────────────────────────────────
	resumeText := ""
	if data.ResumeFilePath != "" {
		raw, err := os.ReadFile(data.ResumeFilePath)
		if err != nil {
			p.log.Warn().
				Err(err).
				Str("path", data.ResumeFilePath).
				Msg("failed to read resume file; proceeding without resume text")
		} else {
			resumeText = string(raw)
		}
	}

	// ── Normalise FormAnswers ───────────────────────────────────────────────────
	formAnswers := data.FormAnswers
	if formAnswers == nil {
		formAnswers = make(map[string]string)
	}

	p.log.Info().
		Str("apply_url", applyURL).
		Str("app_id", appID).
		Str("email", data.Email).
		Bool("has_resume", resumeText != "").
		Bool("has_cover_letter", data.CoverLetterText != "").
		Int("form_answers", len(formAnswers)).
		Msg("delegating Lever application to browser pool")

	// ── Delegate to browser pool ────────────────────────────────────────────────
	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "lever",
		FullName:        data.FullName,
		Email:           data.Email,
		Phone:           data.Phone,
		LinkedIn:        data.LinkedIn,
		Website:         data.Website,
		ResumeText:      resumeText,
		CoverLetterText: data.CoverLetterText,
		FormAnswers:     formAnswers,
	}

	resp, err := p.browserClient.Apply(ctx, req)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("browser pool call failed: %v", err)
		return result, fmt.Errorf("lever apply: %w", err)
	}

	// ── Map response ────────────────────────────────────────────────────────────
	result.Success = resp.Success
	result.ScreenshotURL = resp.ScreenshotKey
	result.ErrorMessage = resp.ErrorMessage
	if len(resp.StepsCompleted) > 0 {
		result.StepsCompleted = resp.StepsCompleted
	}

	if !resp.Success {
		p.log.Warn().
			Str("app_id", appID).
			Str("error", resp.ErrorMessage).
			Strs("steps_completed", result.StepsCompleted).
			Msg("Lever application failed")
		return result, fmt.Errorf("lever apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("Lever application submitted successfully")

	return result, nil
}
