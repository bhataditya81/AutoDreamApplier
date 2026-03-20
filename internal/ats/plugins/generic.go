// Package plugins contains ATS-specific form-filling plugins.
package plugins

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/browser"
)

// GenericPlugin is the fallback ATS plugin used when no specific plugin
// matches the job URL. It instructs the browser-pool to use a heuristic
// form-fill strategy: discover visible inputs, match labels to known field
// types (name, email, phone, resume), and submit the primary button.
//
// Important: register this plugin LAST in the ATS registry so that all
// specific plugins (Greenhouse, Lever, Workday, etc.) take priority via
// their Detect() methods.
type GenericPlugin struct {
	browserClient *browser.Client
	log           zerolog.Logger
}

// NewGenericPlugin creates the generic ATS fallback plugin.
func NewGenericPlugin(browserClient *browser.Client, log zerolog.Logger) *GenericPlugin {
	return &GenericPlugin{
		browserClient: browserClient,
		log:           log.With().Str("ats_plugin", "generic").Logger(),
	}
}

// Name returns the ATS identifier.
func (p *GenericPlugin) Name() string { return "generic" }

// Detect always returns true — this plugin is the catch-all fallback.
// It should be registered last so specific plugins get first pick.
func (p *GenericPlugin) Detect(_ string) bool { return true }

// ValidateURL always returns true — the generic plugin attempts any URL.
func (p *GenericPlugin) ValidateURL(_ string) bool { return true }

// Apply fills out and submits a job application on an unknown ATS by
// delegating to the browser-pool's generic heuristic strategy.
//
// The browser-pool handles:
//  1. Discovering all visible <input> and <textarea> elements
//  2. Matching label text to known field types (name, email, phone, file)
//  3. Filling known fields with applicant data
//  4. Clicking the primary submit button
//  5. Detecting success or failure from post-submit DOM state
func (p *GenericPlugin) Apply(ctx context.Context, applyURL string, data *ats.ApplicationData) (*ats.ApplicationResult, error) {
	result := &ats.ApplicationResult{
		ATSType:        "generic",
		StepsCompleted: []string{},
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
		Msg("delegating unknown ATS application to browser pool (generic strategy)")

	req := &browser.ApplyRequest{
		ApplicationID:   appID,
		ApplyURL:        applyURL,
		ATSType:         "generic",
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
		return result, fmt.Errorf("generic apply: %w", err)
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
			Str("apply_url", applyURL).
			Str("error", resp.ErrorMessage).
			Strs("steps_completed", result.StepsCompleted).
			Msg("generic ATS application failed")
		return result, fmt.Errorf("generic apply unsuccessful: %s", resp.ErrorMessage)
	}

	p.log.Info().
		Str("app_id", appID).
		Str("apply_url", applyURL).
		Str("screenshot_key", resp.ScreenshotKey).
		Int("steps", len(resp.StepsCompleted)).
		Msg("generic ATS application submitted successfully")

	return result, nil
}
