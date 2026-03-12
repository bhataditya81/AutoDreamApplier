// Package plugins contains ATS-specific form-filling plugins.
// Each plugin implements ats.Plugin and delegates the actual Playwright
// automation to the browser-pool microservice via browser.Client.
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

// GreenhousePlugin implements ats.Plugin for Greenhouse.io job applications.
// It delegates the actual form-filling to the browser-pool microservice via
// browser.Client; no Playwright code runs in this process.
type GreenhousePlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewGreenhousePlugin creates a Greenhouse ATS plugin.
// browserClient must not be nil; it is used to delegate form-filling to the
// EC2-hosted browser-pool service.
func NewGreenhousePlugin(browserClient *browser.Client, log zerolog.Logger) *GreenhousePlugin {
	return &GreenhousePlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "greenhouse").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *GreenhousePlugin) Name() string {
	return "greenhouse"
}

// Detect reports whether url belongs to a Greenhouse job board.
func (p *GreenhousePlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "boards.greenhouse.io") ||
		strings.Contains(lower, "grnh.se") ||
		strings.Contains(lower, "greenhouse.io/embed/job_app")
}

// ValidateURL checks if the URL is a valid Greenhouse application URL.
func (p *GreenhousePlugin) ValidateURL(url string) bool {
	return p.Detect(url)
}

// Apply fills out and submits a Greenhouse application via the browser-pool
// microservice. The actual Playwright automation (navigation, field filling,
// submission, screenshot) is handled by the browser-pool service; this method
// is responsible for data marshalling and result mapping.
//
// Resume text: if data.ResumeFilePath is set, the file is read from disk and
// forwarded as ResumeText. The AI service writes tailored resumes as plain
// text files; this path should point to such a file rather than a binary PDF.
//
// Application ID: data.AdditionalFields["application_id"] is preferred; if
// absent a new UUID is generated so the browser pool can correlate logs.
func (p *GreenhousePlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "greenhouse",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid Greenhouse application URL: %s", applyURL)
		return result, fmt.Errorf("greenhouse: %s", result.ErrorMessage)
	}

	// ── Application ID ─────────────────────────────────────────────────────────
	// Prefer an ID supplied by the caller (via AdditionalFields) so that
	// screenshots and logs can be correlated to a database row.
	appID := ""
	if data.AdditionalFields != nil {
		appID = data.AdditionalFields["application_id"]
	}
	if appID == "" {
		appID = uuid.NewString()
		p.log.Debug().Str("app_id", appID).Msg("no application_id in AdditionalFields; generated new UUID")
	}

	// ── Resume text ────────────────────────────────────────────────────────────
	// Read the plain-text tailored resume from disk. A failure here is
	// non-fatal: the browser pool will still attempt the application.
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

	// ── Normalise FormAnswers ──────────────────────────────────────────────────
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
		Msg("delegating Greenhouse application to browser pool")

	// ── Delegate to browser pool ───────────────────────────────────────────────
	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "greenhouse",
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
		return result, fmt.Errorf("greenhouse apply: %w", err)
	}

	// ── Map response ───────────────────────────────────────────────────────────
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
			Msg("Greenhouse application failed")
		return result, fmt.Errorf("greenhouse apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("Greenhouse application submitted successfully")

	return result, nil
}
