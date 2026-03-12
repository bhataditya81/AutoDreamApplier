// Package workers contains Asynq task handlers for the 2-stage apply pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/browser"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

// BrowserApplyWorker is the Stage 2 Asynq handler.
// It loads AI-prepared content from S3, fetches job/user details, then
// delegates form filling + submission to the browser pool microservice.
// S3Buckets is declared in ai_prep_worker.go (same package); do not redefine.
type BrowserApplyWorker struct {
	appRepo       *repository.ApplicationRepository
	browserClient *browser.Client
	s3Client      *pkgs3.Client
	buckets       S3Buckets
	atsRegistry   *ats.Registry        // guards against unsupported ATS types before browser call
	notifier      *notification.Client // nil-safe; no-ops when SES is unconfigured
	log           zerolog.Logger
}

// NewBrowserApplyWorker constructs a BrowserApplyWorker.
// notifier may be nil — notification calls become no-ops.
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

// ProcessTask implements asynq.HandlerFunc for TypeBrowserApply tasks.
func (w *BrowserApplyWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.BrowserApplyPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal browser_apply payload: %w", err)
	}

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

	// ── Load application (S3 keys + pre-answered questions) ───────────────────
	app, err := w.appRepo.GetByID(ctx, p.ApplicationID, p.UserID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get application: %v", err))
	}

	if app.TailoredResumeS3 == "" || app.CoverLetterS3 == "" {
		return w.failApp(ctx, p.ApplicationID,
			"AI prep output missing: tailored resume or cover letter S3 key is empty")
	}

	// ── Load job & user ────────────────────────────────────────────────────────
	job, err := w.appRepo.GetJob(ctx, p.JobID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get job: %v", err))
	}

	// ── Resolve ATS plugin ─────────────────────────────────────────────────────
	// Prefer the pre-classified ats_type stored on the job. When the discovery
	// service set it to "unknown" (or it is missing), fall back to URL-pattern
	// detection so jobs discovered before a plugin was registered can still be
	// applied to.
	resolvedATSType := string(job.ATSType)
	if _, pluginErr := w.atsRegistry.Get(resolvedATSType); pluginErr != nil {
		if detected, ok := w.atsRegistry.DetectATS(job.ApplyURL); ok {
			resolvedATSType = detected.Name()
			log.Info().
				Str("original_ats_type", string(job.ATSType)).
				Str("detected_ats_type", resolvedATSType).
				Msg("ATS type resolved via URL auto-detection")
		} else {
			return w.failApp(ctx, p.ApplicationID,
				fmt.Sprintf("unsupported ATS type %q and URL auto-detection found no matching plugin for %s",
					job.ATSType, job.ApplyURL))
		}
	}

	user, err := w.appRepo.GetUser(ctx, p.UserID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get user: %v", err))
	}

	// ── Fetch AI-prepared content from S3 (both keys live in Resumes bucket) ──
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
			formAnswers = make(map[string]string)
		}
	}

	// ── Delegate to browser pool ───────────────────────────────────────────────
	log.Info().
		Str("apply_url", job.ApplyURL).
		Str("ats_type", resolvedATSType).
		Msg("delegating to browser pool")

	applyResp, err := w.browserClient.Apply(ctx, &browser.ApplyRequest{
		ApplicationID:   p.ApplicationID.String(),
		ApplyURL:        job.ApplyURL,
		ATSType:         resolvedATSType,
		FullName:        user.FullName,
		Email:           user.Email,
		Phone:           "", // not stored in users table; provided by browser pool profile if needed
		LinkedIn:        "",
		Website:         "",
		ResumeText:      resumeText,
		CoverLetterText: coverText,
		FormAnswers:     formAnswers,
	})
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("browser apply HTTP call: %v", err))
	}

	if !applyResp.Success {
		return w.failApp(ctx, p.ApplicationID,
			fmt.Sprintf("browser apply failed: %s", applyResp.ErrorMessage))
	}

	// ── Persist screenshot + mark applied ─────────────────────────────────────
	if applyResp.ScreenshotKey != "" {
		if err := w.appRepo.UpdateScreenshot(ctx, p.ApplicationID, applyResp.ScreenshotKey); err != nil {
			log.Warn().Err(err).
				Str("screenshot_key", applyResp.ScreenshotKey).
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

	// ── Notify user of successful application (non-fatal) ─────────────────────
	w.sendSubmittedNotification(ctx, p.ApplicationID, user.Email, user.FullName, job.Title, job.Company)

	return nil
}

// sendSubmittedNotification emails the user that their application was submitted.
// Errors are logged but never propagated — the apply already succeeded.
func (w *BrowserApplyWorker) sendSubmittedNotification(
	ctx context.Context,
	appID uuid.UUID,
	email, fullName, jobTitle, company string,
) {
	if err := w.notifier.SendApplicationSubmitted(ctx, email, notification.ApplicationSubmittedData{
		UserName:      fullName,
		JobTitle:      jobTitle,
		Company:       company,
		ApplicationID: appID.String(),
	}); err != nil {
		w.log.Warn().Err(err).
			Str("app_id", appID.String()).
			Msg("submitted notify: SES send failed (non-fatal)")
	}
}

// failApp records an error on the application and returns the wrapped error.
func (w *BrowserApplyWorker) failApp(ctx context.Context, appID uuid.UUID, msg string) error {
	if err := w.appRepo.SetError(ctx, appID, msg); err != nil {
		w.log.Error().Err(err).Str("app_id", appID.String()).Msg("failed to record application error")
	}
	if err := w.appRepo.RecordEvent(ctx, appID, models.EventFailed, map[string]any{"error": msg}); err != nil {
		w.log.Warn().Err(err).Str("app_id", appID.String()).Msg("failed to record failed event")
	}
	return fmt.Errorf("%s", msg)
}
