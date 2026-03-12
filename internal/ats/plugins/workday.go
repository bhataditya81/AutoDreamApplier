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

// WorkdayPlugin implements ats.Plugin for Workday job applications.
//
// Workday is widely deployed across enterprise companies. Job application URLs
// appear in two common forms:
//
//   - https://<company>.wd1.myworkdayjobs.com/en-US/<site>/<job-path>
//   - https://<company>.wd5.myworkdayjobs.com/...
//   - https://<company>.myworkdayjobs.com/...         (non-versioned CNAME)
//
// All three patterns share the "myworkdayjobs.com" hostname fragment.
// Some companies embed the Workday apply widget on their own career page via an
// iframe; the canonical form still points to a myworkdayjobs.com URL inside the
// iframe, so Detect covers both direct links and iframe-src patterns.
//
// Like the Greenhouse and Lever plugins, all Playwright automation is delegated
// to the browser-pool microservice via browser.Client; no headless browser runs
// inside this process.
type WorkdayPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewWorkdayPlugin creates a Workday ATS plugin.
// browserClient must not be nil.
func NewWorkdayPlugin(browserClient *browser.Client, log zerolog.Logger) *WorkdayPlugin {
	return &WorkdayPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "workday").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *WorkdayPlugin) Name() string {
	return "workday"
}

// Detect reports whether url belongs to a Workday job board.
//
// Covered patterns:
//   - *.myworkdayjobs.com          (all versioned and non-versioned subdomains)
//   - myworkdayjobs.com            (bare domain, uncommon but possible)
//   - workday.com/en-US/find-jobs  (Workday's own public job board)
func (p *WorkdayPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "myworkdayjobs.com") ||
		strings.Contains(lower, "workday.com/en-us/find-jobs")
}

// ValidateURL checks if the URL is a valid Workday application URL.
func (p *WorkdayPlugin) ValidateURL(url string) bool {
	return p.Detect(url)
}

// Apply fills out and submits a Workday application via the browser-pool
// microservice. The apply URL may point to the job overview page or the direct
// apply step; the browser-pool service handles the navigation difference.
//
// Resume text: if data.ResumeFilePath is set, the file is read from disk and
// forwarded as ResumeText. A missing or unreadable file is non-fatal — the
// browser pool will attempt the application without it.
//
// Application ID: data.AdditionalFields["application_id"] is preferred; a new
// UUID is generated when absent so the browser pool can correlate its logs.
func (p *WorkdayPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "workday",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid Workday application URL: %s", applyURL)
		return result, fmt.Errorf("workday: %s", result.ErrorMessage)
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
		Msg("delegating Workday application to browser pool")

	// ── Delegate to browser pool ────────────────────────────────────────────────
	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "workday",
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
		return result, fmt.Errorf("workday apply: %w", err)
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
			Msg("Workday application failed")
		return result, fmt.Errorf("workday apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("Workday application submitted successfully")

	return result, nil
}
