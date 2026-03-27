package tasks

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// Queue name constants — map to River queue configs in apply-engine/main.go.
const (
	QueueAIPrep       = "ai_prep"
	QueueBrowserApply = "browser_apply"
)

// ── Job args ──────────────────────────────────────────────────────────────────
// Each struct implements river.JobArgs: Kind() identifies the worker,
// InsertOpts() sets queue + priority so callers don't have to repeat them.

// AIPrepArgs is the River job args for Stage 1 (AI tailoring + cover letter).
type AIPrepArgs struct {
	ApplicationID uuid.UUID `json:"application_id"`
	UserID        uuid.UUID `json:"user_id"`
	JobID         uuid.UUID `json:"job_id"`
	ResumeID      uuid.UUID `json:"resume_id"`
}

func (AIPrepArgs) Kind() string { return "ai_prep" }
func (AIPrepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueAIPrep, Priority: 1}
}

// BrowserApplyArgs is the River job args for Stage 2 (browser form-filling).
type BrowserApplyArgs struct {
	ApplicationID uuid.UUID `json:"application_id"`
	UserID        uuid.UUID `json:"user_id"`
	JobID         uuid.UUID `json:"job_id"`
}

func (BrowserApplyArgs) Kind() string { return "browser_apply" }
func (BrowserApplyArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueBrowserApply, Priority: 2}
}
