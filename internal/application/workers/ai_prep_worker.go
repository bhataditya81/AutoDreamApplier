// Package workers contains Asynq task handlers for the 2-stage apply pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	aimodels "github.com/bhata/AutoDreamApplier/internal/ai"
	"github.com/bhata/AutoDreamApplier/internal/application/models"
	"github.com/bhata/AutoDreamApplier/internal/application/repository"
	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

// S3Buckets holds the bucket names needed by workers so they do not have to
// import the full config package.
type S3Buckets struct {
	Resumes     string
	Screenshots string
}

// AIPrepWorker is the Stage 1 Asynq handler.
// It calls the AI service to tailor the resume + generate a cover letter +
// pre-answer common form questions, stores the results in S3, then enqueues
// the Stage 2 browser-apply task.
type AIPrepWorker struct {
	appRepo     *repository.ApplicationRepository
	aiClient    aimodels.Provider
	s3Client    *pkgs3.Client
	asynqClient *asynq.Client
	buckets     S3Buckets
	webhookSvc  *notification.WebhookService // nil-safe; no-ops when not configured
	log         zerolog.Logger
}

// NewAIPrepWorker constructs an AIPrepWorker.
func NewAIPrepWorker(
	appRepo *repository.ApplicationRepository,
	aiClient aimodels.Provider,
	s3Client *pkgs3.Client,
	asynqClient *asynq.Client,
	buckets S3Buckets,
	log zerolog.Logger,
) *AIPrepWorker {
	return &AIPrepWorker{
		appRepo:     appRepo,
		aiClient:    aiClient,
		s3Client:    s3Client,
		asynqClient: asynqClient,
		buckets:     buckets,
		log:         log,
	}
}

// WithWebhookService attaches a WebhookService so AI-prep failures can be
// reported to the user's Slack/Discord. webhookSvc may be nil — no-ops.
func (w *AIPrepWorker) WithWebhookService(ws *notification.WebhookService) *AIPrepWorker {
	w.webhookSvc = ws
	return w
}

// ProcessTask implements asynq.HandlerFunc for TypeAIPrep tasks.
func (w *AIPrepWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.AIPrepPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal ai_prep payload: %w", err)
	}

	log := w.log.With().
		Str("app_id", p.ApplicationID.String()).
		Str("user_id", p.UserID.String()).
		Str("job_id", p.JobID.String()).
		Logger()

	log.Info().Msg("Stage 1: AI prep starting")

	// ── Transition to ai_preparing ────────────────────────────────────────────
	if err := w.appRepo.UpdateStatus(ctx, p.ApplicationID, models.StatusAIPreparing); err != nil {
		return fmt.Errorf("update status ai_preparing: %w", err)
	}
	if err := w.appRepo.RecordEvent(ctx, p.ApplicationID, models.EventAIStarted, nil); err != nil {
		log.Warn().Err(err).Msg("failed to record ai_started event")
	}

	// ── Load resume & job ─────────────────────────────────────────────────────
	resume, err := w.appRepo.GetResume(ctx, p.ResumeID, p.UserID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get resume: %v", err))
	}

	job, err := w.appRepo.GetJob(ctx, p.JobID)
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("get job: %v", err))
	}

	// ── Check user AI tailoring preference ───────────────────────────────────
	prefs, err := w.appRepo.GetPreferences(ctx, p.UserID)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load user preferences; defaulting to AI tailor enabled")
	}
	aiTailorEnabled := prefs == nil || prefs.AiTailorEnabled

	// ── Tailor resume + generate cover letter + pre-answer questions ──────────
	var tailoredText, coverLetter string
	formAnswers := make(map[string]string)

	if aiTailorEnabled {
		tailorResp, err := w.aiClient.TailorResume(ctx, &aimodels.ResumeTailorRequest{
			ResumeText:     resume.RawText,
			JobTitle:       job.Title,
			JobDescription: job.Description,
			CompanyName:    job.Company,
			Mode:           "keyword_inject",
		})
		if err != nil {
			errMsg := fmt.Sprintf("tailor resume: %v", err)
			w.sendFailureWebhook(ctx, p.UserID, job.Title, job.Company, errMsg)
			return w.failApp(ctx, p.ApplicationID, errMsg)
		}
		tailoredText = tailorResp.TailoredText

		coverResp, err := w.aiClient.GenerateCoverLetter(ctx, &aimodels.CoverLetterRequest{
			ResumeText:     resume.RawText,
			JobTitle:       job.Title,
			JobDescription: job.Description,
			CompanyName:    job.Company,
			Tone:           "professional",
		})
		if err != nil {
			errMsg := fmt.Sprintf("generate cover letter: %v", err)
			w.sendFailureWebhook(ctx, p.UserID, job.Title, job.Company, errMsg)
			return w.failApp(ctx, p.ApplicationID, errMsg)
		}
		coverLetter = coverResp.CoverLetter

		commonQuestions := []string{
			"Why do you want to work at " + job.Company + "?",
			"Describe your experience relevant to this role.",
			"Are you authorized to work in the United States?",
		}
		for _, q := range commonQuestions {
			qaResp, err := w.aiClient.AnswerFormQuestion(ctx, &aimodels.FormQARequest{
				Question:     q,
				ResumeText:   resume.RawText,
				JobTitle:     job.Title,
				QuestionType: "text",
			})
			if err != nil {
				log.Warn().Err(err).Str("question", q).Msg("failed to pre-answer form question; skipping")
				continue
			}
			formAnswers[q] = qaResp.Answer
		}

		log.Info().Msg("AI tailoring complete")
	} else {
		// User disabled AI tailoring: use the raw resume as-is, no cover letter.
		tailoredText = resume.RawText
		log.Info().Msg("AI tailoring disabled by user; using raw resume")
	}

	// ── Upload to S3 ──────────────────────────────────────────────────────────
	appIDStr := p.ApplicationID.String()
	resumeKey := pkgs3.ResumeKey(appIDStr)
	coverKey := pkgs3.CoverLetterKey(appIDStr)

	if err := w.s3Client.PutText(ctx, w.buckets.Resumes, resumeKey, tailoredText); err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("upload tailored resume: %v", err))
	}
	if err := w.s3Client.PutText(ctx, w.buckets.Resumes, coverKey, coverLetter); err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("upload cover letter: %v", err))
	}

	log.Info().
		Str("resume_key", resumeKey).
		Str("cover_key", coverKey).
		Msg("AI output uploaded to S3")

	// ── Persist AI output & transition to ai_ready ────────────────────────────
	if err := w.appRepo.UpdateAIOutput(ctx, p.ApplicationID, resumeKey, coverKey, formAnswers); err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("update ai output: %v", err))
	}
	if err := w.appRepo.RecordEvent(ctx, p.ApplicationID, models.EventAICompleted, map[string]any{
		"resume_key":    resumeKey,
		"cover_key":     coverKey,
		"answers_count": len(formAnswers),
	}); err != nil {
		log.Warn().Err(err).Msg("failed to record ai_completed event")
	}

	// ── Enqueue Stage 2 ───────────────────────────────────────────────────────
	browserTask, err := tasks.NewBrowserApply(tasks.BrowserApplyPayload{
		ApplicationID: p.ApplicationID,
		UserID:        p.UserID,
		JobID:         p.JobID,
	})
	if err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("build browser_apply task: %v", err))
	}

	if _, err := w.asynqClient.EnqueueContext(ctx, browserTask); err != nil {
		return w.failApp(ctx, p.ApplicationID, fmt.Sprintf("enqueue browser_apply: %v", err))
	}

	log.Info().Msg("Stage 1: AI prep complete; Stage 2 enqueued")
	return nil
}

// sendFailureWebhook fires an EventApplicationFailed webhook for the user.
// Errors are silently ignored — the prep failure itself is already logged.
func (w *AIPrepWorker) sendFailureWebhook(
	ctx context.Context,
	userID uuid.UUID,
	jobTitle, company, errMsg string,
) {
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

// failApp records an error on the application and returns the wrapped error.
func (w *AIPrepWorker) failApp(ctx context.Context, appID uuid.UUID, msg string) error {
	if err := w.appRepo.SetError(ctx, appID, msg); err != nil {
		w.log.Error().Err(err).Str("app_id", appID.String()).Msg("failed to record application error")
	}
	if err := w.appRepo.RecordEvent(ctx, appID, models.EventFailed, map[string]any{"error": msg}); err != nil {
		w.log.Warn().Err(err).Str("app_id", appID.String()).Msg("failed to record failed event")
	}
	return fmt.Errorf("%s", msg)
}
