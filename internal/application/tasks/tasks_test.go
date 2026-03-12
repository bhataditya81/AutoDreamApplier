package tasks_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/application/tasks"
)

// ── Type constant tests ───────────────────────────────────────────────────────

func TestTypeConstants_NonEmpty(t *testing.T) {
	t.Parallel()

	if tasks.TypeAIPrep == "" {
		t.Error("TypeAIPrep must not be empty")
	}
	if tasks.TypeBrowserApply == "" {
		t.Error("TypeBrowserApply must not be empty")
	}
}

func TestTypeConstants_Distinct(t *testing.T) {
	t.Parallel()

	if tasks.TypeAIPrep == tasks.TypeBrowserApply {
		t.Errorf("TypeAIPrep and TypeBrowserApply must be distinct; both are %q", tasks.TypeAIPrep)
	}
}

// ── Queue constant tests ──────────────────────────────────────────────────────

func TestQueueConstants_NonEmptyAndDistinct(t *testing.T) {
	t.Parallel()

	queues := map[string]string{
		"QueueAIPrep":       tasks.QueueAIPrep,
		"QueueBrowserApply": tasks.QueueBrowserApply,
		"QueueDefault":      tasks.QueueDefault,
	}

	for name, q := range queues {
		if q == "" {
			t.Errorf("%s must not be empty", name)
		}
	}

	// All three must be distinct.
	seen := make(map[string]string) // value → name
	for name, q := range queues {
		if prev, ok := seen[q]; ok {
			t.Errorf("queue values must be distinct: %s and %s both equal %q", prev, name, q)
		}
		seen[q] = name
	}
}

// ── NewAIPrep tests ───────────────────────────────────────────────────────────

func TestNewAIPrep_SetsCorrectType(t *testing.T) {
	t.Parallel()

	p := tasks.AIPrepPayload{
		ApplicationID: uuid.New(),
		UserID:        uuid.New(),
		JobID:         uuid.New(),
		ResumeID:      uuid.New(),
	}

	task, err := tasks.NewAIPrep(p)
	if err != nil {
		t.Fatalf("NewAIPrep: unexpected error: %v", err)
	}
	if task.Type() != tasks.TypeAIPrep {
		t.Errorf("task.Type() = %q; want %q", task.Type(), tasks.TypeAIPrep)
	}
}

func TestNewAIPrep_PayloadRoundTrip(t *testing.T) {
	t.Parallel()

	want := tasks.AIPrepPayload{
		ApplicationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		JobID:         uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ResumeID:      uuid.MustParse("44444444-4444-4444-4444-444444444444"),
	}

	task, err := tasks.NewAIPrep(want)
	if err != nil {
		t.Fatalf("NewAIPrep: unexpected error: %v", err)
	}

	var got tasks.AIPrepPayload
	if err := json.Unmarshal(task.Payload(), &got); err != nil {
		t.Fatalf("json.Unmarshal payload: %v", err)
	}

	if got.ApplicationID != want.ApplicationID {
		t.Errorf("ApplicationID: got %s; want %s", got.ApplicationID, want.ApplicationID)
	}
	if got.UserID != want.UserID {
		t.Errorf("UserID: got %s; want %s", got.UserID, want.UserID)
	}
	if got.JobID != want.JobID {
		t.Errorf("JobID: got %s; want %s", got.JobID, want.JobID)
	}
	if got.ResumeID != want.ResumeID {
		t.Errorf("ResumeID: got %s; want %s", got.ResumeID, want.ResumeID)
	}
}

// ── NewBrowserApply tests ─────────────────────────────────────────────────────

func TestNewBrowserApply_SetsCorrectType(t *testing.T) {
	t.Parallel()

	p := tasks.BrowserApplyPayload{
		ApplicationID: uuid.New(),
		UserID:        uuid.New(),
		JobID:         uuid.New(),
	}

	task, err := tasks.NewBrowserApply(p)
	if err != nil {
		t.Fatalf("NewBrowserApply: unexpected error: %v", err)
	}
	if task.Type() != tasks.TypeBrowserApply {
		t.Errorf("task.Type() = %q; want %q", task.Type(), tasks.TypeBrowserApply)
	}
}

func TestNewBrowserApply_PayloadRoundTrip(t *testing.T) {
	t.Parallel()

	want := tasks.BrowserApplyPayload{
		ApplicationID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UserID:        uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		JobID:         uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
	}

	task, err := tasks.NewBrowserApply(want)
	if err != nil {
		t.Fatalf("NewBrowserApply: unexpected error: %v", err)
	}

	var got tasks.BrowserApplyPayload
	if err := json.Unmarshal(task.Payload(), &got); err != nil {
		t.Fatalf("json.Unmarshal payload: %v", err)
	}

	if got.ApplicationID != want.ApplicationID {
		t.Errorf("ApplicationID: got %s; want %s", got.ApplicationID, want.ApplicationID)
	}
	if got.UserID != want.UserID {
		t.Errorf("UserID: got %s; want %s", got.UserID, want.UserID)
	}
	if got.JobID != want.JobID {
		t.Errorf("JobID: got %s; want %s", got.JobID, want.JobID)
	}
}
