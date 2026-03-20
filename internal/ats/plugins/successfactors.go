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

// SuccessFactorsPlugin implements ats.Plugin for SAP SuccessFactors job
// applications.
//
// SAP SuccessFactors Recruiting is used by large enterprises globally.
// Application URLs typically follow these patterns:
//
//   - https://<company>.successfactors.com/career?...
//   - https://<company>.successfactors.eu/career?...
//   - https://performancemanager.successfactors.com/...
//   - https://jobs.sap.com/... (SAP's own career site, also SuccessFactors)
//
// The plugin delegates all Playwright automation to the browser-pool
// microservice; no browser code runs in this process.
type SuccessFactorsPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewSuccessFactorsPlugin creates a SuccessFactors ATS plugin.
func NewSuccessFactorsPlugin(browserClient *browser.Client, log zerolog.Logger) *SuccessFactorsPlugin {
	return &SuccessFactorsPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "successfactors").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *SuccessFactorsPlugin) Name() string { return "successfactors" }

// Detect reports whether url belongs to a SuccessFactors job board.
func (p *SuccessFactorsPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "successfactors.com") ||
		strings.Contains(lower, "successfactors.eu") ||
		strings.Contains(lower, "successfactors.cn") ||
		strings.Contains(lower, "performancemanager.successfactors")
}

// ValidateURL checks if the URL is a valid SuccessFactors application URL.
func (p *SuccessFactorsPlugin) ValidateURL(url string) bool { return p.Detect(url) }

// Apply fills out and submits a SuccessFactors application via the browser-pool
// microservice.
func (p *SuccessFactorsPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "successfactors",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid SuccessFactors application URL: %s", applyURL)
		return result, fmt.Errorf("successfactors: %s", result.ErrorMessage)
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
		Msg("delegating SuccessFactors application to browser pool")

	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "successfactors",
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
		return result, fmt.Errorf("successfactors apply: %w", err)
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
			Msg("SuccessFactors application failed")
		return result, fmt.Errorf("successfactors apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("SuccessFactors application submitted successfully")

	return result, nil
}
