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

// SmartRecruitersPlugin implements ats.Plugin for SmartRecruiters job
// applications.
//
// SmartRecruiters is a popular cloud-based ATS used by mid-market and
// enterprise companies.  Application URLs typically follow these patterns:
//
//   - https://jobs.smartrecruiters.com/<Company>/<id>-<job-title>
//   - https://careers.smartrecruiters.com/<Company>/...
//   - https://<company>.smartrecruiters.com/...
//
// The plugin delegates all Playwright automation to the browser-pool
// microservice; no browser code runs in this process.
type SmartRecruitersPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewSmartRecruitersPlugin creates a SmartRecruiters ATS plugin.
func NewSmartRecruitersPlugin(browserClient *browser.Client, log zerolog.Logger) *SmartRecruitersPlugin {
	return &SmartRecruitersPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "smartrecruiters").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *SmartRecruitersPlugin) Name() string { return "smartrecruiters" }

// Detect reports whether url belongs to a SmartRecruiters job board.
func (p *SmartRecruitersPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "smartrecruiters.com") ||
		strings.Contains(lower, "jobs.smartrecruiters.com") ||
		strings.Contains(lower, "careers.smartrecruiters.com")
}

// ValidateURL checks if the URL is a valid SmartRecruiters application URL.
func (p *SmartRecruitersPlugin) ValidateURL(url string) bool { return p.Detect(url) }

// Apply fills out and submits a SmartRecruiters application via the
// browser-pool microservice.
func (p *SmartRecruitersPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "smartrecruiters",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid SmartRecruiters application URL: %s", applyURL)
		return result, fmt.Errorf("smartrecruiters: %s", result.ErrorMessage)
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
		Msg("delegating SmartRecruiters application to browser pool")

	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "smartrecruiters",
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
		return result, fmt.Errorf("smartrecruiters apply: %w", err)
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
			Msg("SmartRecruiters application failed")
		return result, fmt.Errorf("smartrecruiters apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("SmartRecruiters application submitted successfully")

	return result, nil
}
