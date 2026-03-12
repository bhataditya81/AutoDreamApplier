package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Task type constants — used by both enqueuers and handlers.
const (
	TypeAIPrep        = "application:ai_prep"
	TypeBrowserApply  = "application:browser_apply"
)

// Queue name constants — map to Asynq queue priorities in main.go.
const (
	QueueAIPrep       = "ai_prep"
	QueueBrowserApply = "browser_apply"
	QueueDefault      = "default"
)

// ── Payload types ─────────────────────────────────────────────────────────────

// AIPrepPayload is the task payload for Stage 1 (AI tailoring).
type AIPrepPayload struct {
	ApplicationID uuid.UUID `json:"application_id"`
	UserID        uuid.UUID `json:"user_id"`
	JobID         uuid.UUID `json:"job_id"`
	ResumeID      uuid.UUID `json:"resume_id"`
}

// BrowserApplyPayload is the task payload for Stage 2 (form-filling).
type BrowserApplyPayload struct {
	ApplicationID uuid.UUID `json:"application_id"`
	UserID        uuid.UUID `json:"user_id"`
	JobID         uuid.UUID `json:"job_id"`
}

// ── Constructors ──────────────────────────────────────────────────────────────

// NewAIPrep creates a new Stage 1 Asynq task routed to the ai_prep queue.
func NewAIPrep(p AIPrepPayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal ai_prep payload: %w", err)
	}
	return asynq.NewTask(TypeAIPrep, payload, asynq.Queue(QueueAIPrep)), nil
}

// NewBrowserApply creates a new Stage 2 Asynq task routed to the browser_apply queue.
func NewBrowserApply(p BrowserApplyPayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal browser_apply payload: %w", err)
	}
	return asynq.NewTask(TypeBrowserApply, payload, asynq.Queue(QueueBrowserApply)), nil
}
