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

// TaleoPlugin implements ats.Plugin for Oracle Taleo job applications.
//
// Oracle Taleo Enterprise Edition is widely used by large enterprises.
// Application URLs typically follow these patterns:
//
//   - https://<company>.taleo.net/careersection/...
//   - https://tbe.taleo.net/...
//   - https://<company>.ats.hrsmart.com/... (Taleo Business Edition / HRSmart)
//
// The plugin delegates all Playwright automation to the browser-pool
// microservice; no browser code runs in this process.
type TaleoPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewTaleoPlugin creates a Taleo ATS plugin.
func NewTaleoPlugin(browserClient *browser.Client, log zerolog.Logger) *TaleoPlugin {
	return &TaleoPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "taleo").Logger(),
	}
}

// Name returns the ATS identifier used throughout the system.
func (p *TaleoPlugin) Name() string { return "taleo" }

// Detect reports whether url belongs to a Taleo job board.
func (p *TaleoPlugin) Detect(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, ".taleo.net") ||
		strings.Contains(lower, "tbe.taleo.net") ||
		strings.Contains(lower, "taleo.net/careersection")
}

// ValidateURL checks if the URL is a valid Taleo application URL.
func (p *TaleoPlugin) ValidateURL(url string) bool { return p.Detect(url) }

// Apply fills out and submits a Taleo application via the browser-pool
// microservice.
func (p *TaleoPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "taleo",
		StepsCompleted: []string{},
	}

	if !p.ValidateURL(applyURL) {
		result.ErrorMessage = fmt.Sprintf("not a valid Taleo application URL: %s", applyURL)
		return result, fmt.Errorf("taleo: %s", result.ErrorMessage)
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
		Msg("delegating Taleo application to browser pool")

	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "taleo",
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
		return result, fmt.Errorf("taleo apply: %w", err)
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
			Msg("Taleo application failed")
		return result, fmt.Errorf("taleo apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("Taleo application submitted successfully")

	return result, nil
}
