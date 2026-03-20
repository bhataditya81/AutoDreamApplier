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

// ICIMSPlugin implements ats.Plugin for iCIMS Talent Cloud job applications.
//
// iCIMS is one of the largest enterprise ATS platforms, used by over 4,000
// companies.  Application URLs typically follow these patterns:
//
//   - https://<company>.icims.com/jobs/<id>/job
//   - https://careers-<company>.icims.com/jobs/<id>/job
//   - https://careers.icims.com/...
//
// The plugin delegates all Playwright automation to the browser-pool
// microservice; no browser code runs in this process.
type ICIMSPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewICIMSPlugin creates an iCIMS ATS plugin.
func NewICIMSPlugin(browserClient *browser.Client, log zerolog.Logger) *ICIMSPlugin {
	return &ICIMSPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "icims").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *ICIMSPlugin) Name() string { return "icims" }

// Detect reports whether url belongs to an iCIMS job board.
func (p *ICIMSPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, ".icims.com") ||
		strings.Contains(lower, "icims.com/jobs/")
}

// ValidateURL checks if the URL is a valid iCIMS application URL.
func (p *ICIMSPlugin) ValidateURL(url string) bool { return p.Detect(url) }

// Apply fills out and submits an iCIMS application via the browser-pool
// microservice.
func (p *ICIMSPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "icims",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid iCIMS application URL: %s", applyURL)
		return result, fmt.Errorf("icims: %s", result.ErrorMessage)
	}

	appID := ""
	if data.AdditionalFields != nil {
		appID = data.AdditionalFields["application_id"]
	}
	if appID == "" {
		appID = uuid.NewString()
		p.log.Debug().Str("app_id", appID).Msg("no application_id in AdditionalFields; generated new UUID")
	}

	resumeText := ""
	if data.ResumeFilePath != "" {
		raw, err := os.ReadFile(data.ResumeFilePath)
		if err != nil {
			p.log.Warn().Err(err).Str("path", data.ResumeFilePath).
				Msg("failed to read resume file; proceeding without resume text")
		} else {
			resumeText = string(raw)
		}
	}

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
		Msg("delegating iCIMS application to browser pool")

	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "icims",
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
		return result, fmt.Errorf("icims apply: %w", err)
	}

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
			Msg("iCIMS application failed")
		return result, fmt.Errorf("icims apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("iCIMS application submitted successfully")

	return result, nil
}
