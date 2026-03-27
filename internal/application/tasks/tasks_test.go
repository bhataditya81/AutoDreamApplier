package tasks_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
)

// ── Kind constant tests ───────────────────────────────────────────────────────

func TestAIPrepArgs_Kind(t *testing.T) {
	t.Parallel()
	a := tasks.AIPrepArgs{}
	if a.Kind() == "" {
		t.Error("AIPrepArgs.Kind() must not be empty")
	}
}

func TestBrowserApplyArgs_Kind(t *testing.T) {
	t.Parallel()
	a := tasks.BrowserApplyArgs{}
	if a.Kind() == "" {
		t.Error("BrowserApplyArgs.Kind() must not be empty")
	}
}

func TestKinds_Distinct(t *testing.T) {
	t.Parallel()
	ai := tasks.AIPrepArgs{}.Kind()
	br := tasks.BrowserApplyArgs{}.Kind()
	if ai == br {
		t.Errorf("AIPrepArgs and BrowserApplyArgs must have distinct Kind values; both are %q", ai)
	}
}

// ── Queue constant tests ──────────────────────────────────────────────────────

func TestQueueConstants_NonEmptyAndDistinct(t *testing.T) {
	t.Parallel()

	queues := map[string]string{
		"QueueAIPrep":       tasks.QueueAIPrep,
		"QueueBrowserApply": tasks.QueueBrowserApply,
	}

	for name, q := range queues {
		if q == "" {
			t.Errorf("%s must not be empty", name)
		}
	}

	seen := make(map[string]string)
	for name, q := range queues {
		if prev, ok := seen[q]; ok {
			t.Errorf("queue values must be distinct: %s and %s both equal %q", prev, name, q)
		}
		seen[q] = name
	}
}

// ── InsertOpts tests ──────────────────────────────────────────────────────────

func TestAIPrepArgs_InsertOpts_Queue(t *testing.T) {
	t.Parallel()
	opts := tasks.AIPrepArgs{}.InsertOpts()
	if opts.Queue != tasks.QueueAIPrep {
		t.Errorf("AIPrepArgs InsertOpts queue = %q; want %q", opts.Queue, tasks.QueueAIPrep)
	}
	if opts.Priority < 1 {
		t.Errorf("AIPrepArgs InsertOpts priority = %d; want >= 1", opts.Priority)
	}
}

func TestBrowserApplyArgs_InsertOpts_Queue(t *testing.T) {
	t.Parallel()
	opts := tasks.BrowserApplyArgs{}.InsertOpts()
	if opts.Queue != tasks.QueueBrowserApply {
		t.Errorf("BrowserApplyArgs InsertOpts queue = %q; want %q", opts.Queue, tasks.QueueBrowserApply)
	}
	if opts.Priority < 1 {
		t.Errorf("BrowserApplyArgs InsertOpts priority = %d; want >= 1", opts.Priority)
	}
}

// ── JobArgs interface compliance ──────────────────────────────────────────────

func TestAIPrepArgs_ImplementsJobArgs(t *testing.T) {
	t.Parallel()
	var _ river.JobArgs = tasks.AIPrepArgs{}
}

func TestBrowserApplyArgs_ImplementsJobArgs(t *testing.T) {
	t.Parallel()
	var _ river.JobArgs = tasks.BrowserApplyArgs{}
}

// ── Field round-trip tests ────────────────────────────────────────────────────

func TestAIPrepArgs_Fields(t *testing.T) {
	t.Parallel()
	appID := uuid.New()
	userID := uuid.New()
	jobID := uuid.New()
	resumeID := uuid.New()

	a := tasks.AIPrepArgs{
		ApplicationID: appID,
		UserID:        userID,
		JobID:         jobID,
		ResumeID:      resumeID,
	}

	if a.ApplicationID != appID {
		t.Errorf("ApplicationID mismatch")
	}
	if a.UserID != userID {
		t.Errorf("UserID mismatch")
	}
	if a.JobID != jobID {
		t.Errorf("JobID mismatch")
	}
	if a.ResumeID != resumeID {
		t.Errorf("ResumeID mismatch")
	}
}

func TestBrowserApplyArgs_Fields(t *testing.T) {
	t.Parallel()
	appID := uuid.New()
	userID := uuid.New()
	jobID := uuid.New()

	a := tasks.BrowserApplyArgs{
		ApplicationID: appID,
		UserID:        userID,
		JobID:         jobID,
	}

	if a.ApplicationID != appID {
		t.Errorf("ApplicationID mismatch")
	}
	if a.UserID != userID {
		t.Errorf("UserID mismatch")
	}
	if a.JobID != jobID {
		t.Errorf("JobID mismatch")
	}
}
