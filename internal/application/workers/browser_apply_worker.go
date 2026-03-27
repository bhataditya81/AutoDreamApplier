// Package workers contains River job workers for the 2-stage apply pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

// BrowserApplyWorker is the Stage 2 River worker.
// It loads AI-prepared content from S3, fetches job/user details, then
// delegates form filling + submission to the browser pool microservice.
// S3Buckets is declared in ai_prep_worker.go (same package); do not redefine.
type BrowserApplyWorker struct {
	river.WorkerDefaults[tasks.BrowserApplyArgs]

	appRepo      *repository.ApplicationRepository
	browserClient *browser.Client
	s3Client     *pkgs3.Client
	buckets      S3Buckets
	atsRegistry  *ats.Registry
	notifier     *notification.Client
	webhookSvc   *notification.WebhookService
	dashboardURL string
	log          zerolog.Logger
}

// NewBrowserApplyWorker constructs a BrowserApplyWorker.
func NewBrowserApplyWorker(
	appRepo *repository.ApplicationRepository,
	browserClient *browser.Client,
	s3Client *pkgs3.Client,
	buckets S3Buckets,
	atsRegistry *ats.Registry,
	notifier *notification.Client,
	log zerolog.Logger,
) *BrowserApplyWorker {
	return &BrowserApplyWorker{
		appRepo:       appRepo,
		browserClient: browserClient,
		s3Client:      s3Client,
		buckets:       buckets,
		atsRegistry:   atsRegistry,
		notifier:      notifier,
		log:           log,
	}
}

// WithWebhookService attaches a WebhookService and dashboard base URL.
func (w *BrowserApplyWorker) WithWebhookService(ws *notification.WebhookService, dashboardURL string) *BrowserApplyWorker {
	w.webhookSvc = ws
	w.dashboardURL = dashboardURL
	return w
}

// toWebhookEvents converts raw event strings to typed WebhookEvent values.
func toWebhookEvents(events []string) []notification.WebhookEvent {
	out := make([]notification.WebhookEvent, 0, len(events))
	for _, e := range events {
		out = append(out, notification.WebhookEvent(e))
	}
	return out
}

// Work implements river.Worker for BrowserApplyArgs (Stage 2).
func (w *BrowserApplyWorker) Work(ctx context.Context, job *river.Job[tasks.BrowserApplyArgs]) error {
	p := job.Args

	log := w.log.With().
		Str("app_id", p.ApplicationID.String()).
		Str("user_id", p.UserID.String()).
		Str("job_id", p.JobID.String()).
		Logger()

	log.Info().Msg("Stage 2: browser apply starting")

	// ── Transition to applying ─────────────────────────────────────────────────
	if err := w.appRepo.UpdateStatus(ctx, p.ApplicationID, models.StatusApplying); err != nil {
		return fmt.Errorf("update status applying: %w", err)
	}
	if err := w.appRepo.RecordEvent(ctx, p.ApplicationID, models.EventBrowserStarted, nil); err != nil {
		log.Warn().Err(err).Msg("failed to record browser_started event")
	}

	// ── Load application ───────────────────────────────────────────────────────
	app, err := w.appRepo.GetByID(ctx, p.ApplicationID, p.UserID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get application: %v", err))
	}
	if app.TailoredResumeS3 == "" || app.CoverLetterS3 == "" {
		return w.failApp(ctx, p.ApplicationID,
			"AI prep output missing: tailored resume or cover letter S3 key is empty")
	}

	// ── Load job & user ────────────────────────────────────────────────────────
	job2, err := w.appRepo.GetJob(ctx, p.JobID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get job: %v", err))
	}

	// ── Resolve ATS plugin ─────────────────────────────────────────────────────
	resolvedATSType := string(job2.ATSType)
	if _, pluginErr := w.atsRegistry.Get(resolvedATSType); pluginErr != nil {
		if detected, ok := w.atsRegistry.DetectATS(job2.ApplyURL); ok {
			resolvedATSType = detected.Name()
			log.Info().
				Str("original_ats_type", string(job2.ATSType)).
				Str("detected_ats_type", resolvedATSType).
				Msg("ATS type resolved via URL auto-detection")
		} else {
			return w.failApp(ctx, p.ApplicationID,
				fmt.Sprintf("unsupported ATS type %q and URL auto-detection found no matching plugin for %s",
					job2.ATSType, job2.ApplyURL))
		}
	}

	user, err := w.appRepo.GetUser(ctx, p.UserID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get user: %v", err))
	}

	// ── Fetch AI-prepared content from S3 ─────────────────────────────────────
	resumeText, err := w.s3Client.GetText(ctx, w.buckets.Resumes, app.TailoredResumeS3)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("fetch tailored resume from S3: %v", err))
	}
	coverText, err := w.s3Client.GetText(ctx, w.buckets.Resumes, app.CoverLetterS3)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("fetch cover letter from S3: %v", err))
	}

	// ── Deserialize pre-answered form questions ────────────────────────────────
	formAnswers := make(map[string]string)
	if len(app.FormAnswersJSON) > 0 {
		if err := json.Unmarshal(app.FormAnswersJSON, &formAnswers); err != nil {
			log.Warn().Err(err).Msg("failed to deserialize form answers; proceeding with empty map")
		}
	}

	// ── Delegate to browser pool ───────────────────────────────────────────────
	log.Info().
		Str("apply_url", job2.ApplyURL).
		Str("ats_type", resolvedATSType).
		Msg("delegating to browser pool")

	applyResp, err := w.browserClient.Apply(ctx, &browser.ApplyRequest{
		ApplicationID:   p.ApplicationID.String(),
		ApplyURL:        job2.ApplyURL,
		ATSType:         resolvedATSType,
		FullName:        user.FullName,
		Email:           user.Email,
		Phone:           "",
		LinkedIn:        "",
		Website:         "",
		ResumeText:      resumeText,
		CoverLetterText: coverText,
		FormAnswers:     formAnswers,
	})
	if err != nil {
		errMsg := fmt.Sprintf("browser apply HTTP call: %v", err)
		w.sendFailureWebhook(ctx, p.UserID, job2.Title, job2.Company, errMsg)
		return w.failApp(ctx, p.ApplicationID, errMsg)
	}
	if !applyResp.Success {
		errMsg := fmt.Sprintf("browser apply failed: %s", applyResp.ErrorMessage)
		w.sendFailureWebhook(ctx, p.UserID, job2.Title, job2.Company, errMsg)
		return w.failApp(ctx, p.ApplicationID, errMsg)
	}

	// ── Persist screenshot + mark applied ─────────────────────────────────────
	if applyResp.ScreenshotKey != "" {
		if err := w.appRepo.UpdateScreenshot(ctx, p.ApplicationID, applyResp.ScreenshotKey); err != nil {
			log.Warn().Err(err).Str("screenshot_key", applyResp.ScreenshotKey).
				Msg("failed to persist screenshot key")
		}
	}
	if err := w.appRepo.SetAppliedAt(ctx, p.ApplicationID, time.Now().UTC()); err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("set applied_at: %v", err))
	}
	if err := w.appRepo.RecordEvent(ctx, p.ApplicationID, models.EventSubmitted, map[string]any{
		"screenshot_key":  applyResp.ScreenshotKey,
		"steps_completed": applyResp.StepsCompleted,
	}); err != nil {
		log.Warn().Err(err).Msg("failed to record submitted event")
	}

	log.Info().
		Str("screenshot_key", applyResp.ScreenshotKey).
		Int("steps", len(applyResp.StepsCompleted)).
		Msg("Stage 2: browser apply complete")

	w.sendSubmittedNotification(ctx, p.ApplicationID, user.Email, user.FullName, job2.Title, job2.Company)

	if w.webhookSvc != nil {
		prefs, err := w.appRepo.GetPreferences(ctx, p.UserID)
		if err == nil && prefs != nil {
			cfg := notification.WebhookConfig{
				SlackURL:   prefs.SlackWebhookURL,
				DiscordURL: prefs.DiscordWebhookURL,
				Events:     toWebhookEvents(prefs.WebhookEvents),
			}
			w.webhookSvc.Send(ctx, cfg, notification.EventApplicationSubmitted, map[string]string{
				"title":         job2.Title,
				"company":       job2.Company,
				"dashboard_url": w.dashboardURL + "/dashboard/applications/" + p.ApplicationID.String(),
			})
		}
	}

	return nil
}

func (w *BrowserApplyWorker) sendSubmittedNotification(ctx context.Context, appID uuid.UUID, email, fullName, jobTitle, company string) {
	if err := w.notifier.SendApplicationSubmitted(ctx, email, notification.ApplicationSubmittedData{
		UserName:      fullName,
		JobTitle:      jobTitle,
		Company:       company,
		ApplicationID: appID.String(),
	}); err != nil {
		w.log.Warn().Err(err).Str("app_id", appID.String()).Msg("submitted notify: SES send failed (non-fatal)")
	}
}

func (w *BrowserApplyWorker) sendFailureWebhook(ctx context.Context, userID uuid.UUID, jobTitle, company, errMsg string) {
	if w.webhookSvc == nil {
		return
	}
	prefs, err := w.appRepo.GetPreferences(ctx, userID)
	if err != nil || prefs == nil {
		return
	}
	cfg := notification.WebhookConfig{
		SlackURL:   prefs.SlackWebhookURL,
		DiscordURL: prefs.DiscordWebhookURL,
		Events:     toWebhookEvents(prefs.WebhookEvents),
	}
	w.webhookSvc.Send(ctx, cfg, notification.EventApplicationFailed, map[string]string{
		"title":   jobTitle,
		"company": company,
		"error":   errMsg,
	})
}

func (w *BrowserApplyWorker) failApp(ctx context.Context, appID uuid.UUID, msg string) error {
	if err := w.appRepo.SetError(ctx, appID, msg); err != nil {
		w.log.Error().Err(err).Str("app_id", appID.String()).Msg("failed to record application error")
	}
	if err := w.appRepo.RecordEvent(ctx, appID, models.EventFailed, map[string]any{"error": msg}); err != nil {
		w.log.Warn().Err(err).Str("app_id", appID.String()).Msg("failed to record failed event")
	}
	return fmt.Errorf("%s", msg)
}
