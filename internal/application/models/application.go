package models

import (
	"time"

	"github.com/google/uuid"
)

// ApplicationStatus represents the current state in the 2-stage pipeline.
type ApplicationStatus string

const (
	// StatusQueued — match approved, waiting for Stage 1 (AI prep)
	StatusQueued      ApplicationStatus = "queued"
	// StatusAIPreparing — Stage 1 Asynq task picked up by a worker
	StatusAIPreparing ApplicationStatus = "ai_preparing"
	// StatusAIReady — AI prep complete, content stored in S3; Stage 2 ready
	StatusAIReady     ApplicationStatus = "ai_ready"
	// StatusApplying — Stage 2 Asynq task picked up by a browser worker
	StatusApplying    ApplicationStatus = "applying"
	// StatusApplied — form submitted successfully
	StatusApplied     ApplicationStatus = "applied"
	// StatusFailed — an unrecoverable error occurred in either stage
	StatusFailed      ApplicationStatus = "failed"
	// StatusWithdrawn — user manually cancelled before submission
	StatusWithdrawn   ApplicationStatus = "withdrawn"
)

// ApplicationOutcome tracks the post-submission funnel.
type ApplicationOutcome string

const (
	OutcomeViewed    ApplicationOutcome = "viewed"
	OutcomeRejected  ApplicationOutcome = "rejected"
	OutcomeInterview ApplicationOutcome = "interview"
	OutcomeOffer     ApplicationOutcome = "offer"
)

// Application is the central entity for tracking a job application attempt.
type Application struct {
	ID                  uuid.UUID          `json:"id" db:"id"`
	UserID              uuid.UUID          `json:"user_id" db:"user_id"`
	JobID               uuid.UUID          `json:"job_id" db:"job_id"`
	MatchID             uuid.UUID          `json:"match_id" db:"match_id"`
	ResumeID            uuid.UUID          `json:"resume_id" db:"resume_id"`
	Status              ApplicationStatus  `json:"status" db:"status"`
	Outcome             *ApplicationOutcome `json:"outcome,omitempty" db:"outcome"`
	TailoredResumeS3    string             `json:"tailored_resume_s3,omitempty" db:"tailored_resume_s3"`
	CoverLetterS3       string             `json:"cover_letter_s3,omitempty" db:"cover_letter_s3"`
	ScreenshotS3        string             `json:"screenshot_s3,omitempty" db:"screenshot_s3"`
	FormAnswersJSON     []byte             `json:"-" db:"form_answers_json"`
	ErrorMessage        string             `json:"error_message,omitempty" db:"error_message"`
	AppliedAt           *time.Time         `json:"applied_at,omitempty" db:"applied_at"`
	OutcomeUpdatedAt    *time.Time         `json:"outcome_updated_at,omitempty" db:"outcome_updated_at"`
	OutcomeNotes        string             `json:"outcome_notes,omitempty" db:"outcome_notes"`
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
}

// ApplicationEvent records a step in the audit trail for an application.
type ApplicationEvent struct {
	ID            uuid.UUID `json:"id" db:"id"`
	ApplicationID uuid.UUID `json:"application_id" db:"application_id"`
	EventType     string    `json:"event_type" db:"event_type"`
	Details       []byte    `json:"details,omitempty" db:"details"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Event type constants — matches the plan's application_events.event_type enum.
const (
	EventCreated        = "created"
	EventAIStarted      = "ai_started"
	EventAICompleted    = "ai_completed"
	EventBrowserStarted = "browser_started"
	EventFormFilled     = "form_filled"
	EventSubmitted      = "submitted"
	EventFailed         = "failed"
	EventCaptchaHit     = "captcha_hit"
	EventWithdrawn      = "withdrawn"
)

// SubmitRequest is the HTTP body for enqueuing a new application.
type SubmitRequest struct {
	UserID  string `json:"user_id"`
	JobID   string `json:"job_id"`
	MatchID string `json:"match_id"`
}

// UpdateOutcomeRequest is the HTTP body for recording an application outcome.
type UpdateOutcomeRequest struct {
	Outcome string `json:"outcome"`
	Notes   string `json:"notes,omitempty"` // optional free-text notes (stored in outcome_notes)
}
