package notification

// White-box tests for FollowUpScheduler — we are in the same package so we
// can construct internal structs directly without exporting them.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ── Mock implementations ──────────────────────────────────────────────────────

// mockFollowUpUserRepo satisfies FollowUpUserRepo.
type mockFollowUpUserRepo struct {
	users              []FollowUpUser
	getUsersErr        error
	markFollowUpSentFn func(ctx context.Context, applicationID uuid.UUID) error
	markCalls          int32 // atomic counter
}

func (m *mockFollowUpUserRepo) GetAllActiveUsersForFollowUp(ctx context.Context) ([]FollowUpUser, error) {
	return m.users, m.getUsersErr
}

func (m *mockFollowUpUserRepo) MarkFollowUpSent(ctx context.Context, applicationID uuid.UUID) error {
	atomic.AddInt32(&m.markCalls, 1)
	if m.markFollowUpSentFn != nil {
		return m.markFollowUpSentFn(ctx, applicationID)
	}
	return nil
}

// mockFollowUpRepo satisfies FollowUpRepository (used by FollowUpService).
type mockFollowUpRepo struct {
	followUps    []FollowUp
	followUpsErr error
	dismissErr   error
}

func (m *mockFollowUpRepo) GetApplicationsNeedingFollowUp(ctx context.Context, userID uuid.UUID, followUpDays int) ([]FollowUp, error) {
	return m.followUps, m.followUpsErr
}

func (m *mockFollowUpRepo) DismissFollowUp(ctx context.Context, applicationID uuid.UUID, userID uuid.UUID) error {
	return m.dismissErr
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newTestScheduler(userRepo FollowUpUserRepo, fuRepo FollowUpRepository) *FollowUpScheduler {
	log := zerolog.Nop()
	svc := NewFollowUpService(fuRepo, log)
	// Pass a nil *Client — sendFollowUpReminder is nil-safe, so it becomes a no-op.
	return NewFollowUpScheduler(svc, userRepo, nil, log)
}

func sampleUser(email string) FollowUpUser {
	return FollowUpUser{
		ID:       uuid.New(),
		Email:    email,
		FullName: "Test User",
	}
}

func sampleFollowUp(company, title string) FollowUp {
	return FollowUp{
		ApplicationID: uuid.New(),
		JobTitle:      title,
		Company:       company,
		AppliedAt:     time.Now().Add(-8 * 24 * time.Hour),
		DaysElapsed:   8,
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestRun_CancelledContext verifies Run() exits cleanly when ctx is cancelled
// before any tick fires.
func TestRun_CancelledContext(t *testing.T) {
	t.Parallel()

	userRepo := &mockFollowUpUserRepo{}
	fuRepo := &mockFollowUpRepo{}
	s := newTestScheduler(userRepo, fuRepo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good — Run returned.
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit within 2s after context cancellation")
	}
}

// TestTick_NoActiveUsers verifies that when the repo returns an empty user list,
// no follow-up logic runs.
func TestTick_NoActiveUsers(t *testing.T) {
	t.Parallel()

	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{}} // empty
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{sampleFollowUp("ACME", "Engineer")},
	}
	s := newTestScheduler(userRepo, fuRepo)

	s.tick(context.Background())

	// MarkFollowUpSent must not have been called — no users means no emails.
	if got := atomic.LoadInt32(&userRepo.markCalls); got != 0 {
		t.Errorf("MarkFollowUpSent called %d times; want 0 (no users)", got)
	}
}

// TestTick_TwoUsers_OneFollowUpEach verifies that with 2 users each having
// 1 pending follow-up, MarkFollowUpSent is called exactly twice.
func TestTick_TwoUsers_OneFollowUpEach(t *testing.T) {
	t.Parallel()

	user1 := sampleUser("alice@example.com")
	user2 := sampleUser("bob@example.com")

	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{user1, user2}}
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{sampleFollowUp("ACME", "Software Engineer")},
	}
	s := newTestScheduler(userRepo, fuRepo)

	s.tick(context.Background())

	// Each user has 1 follow-up → MarkFollowUpSent called once per user = 2 total.
	if got := atomic.LoadInt32(&userRepo.markCalls); got != 2 {
		t.Errorf("MarkFollowUpSent called %d times; want 2 (1 per user)", got)
	}
}

// TestTick_TwoUsers_TwoFollowUpsEach verifies MarkFollowUpSent is called 4
// times when 2 users each have 2 pending follow-ups.
func TestTick_TwoUsers_TwoFollowUpsEach(t *testing.T) {
	t.Parallel()

	user1 := sampleUser("carol@example.com")
	user2 := sampleUser("dave@example.com")

	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{user1, user2}}
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{
			sampleFollowUp("Corp A", "Backend Engineer"),
			sampleFollowUp("Corp B", "Frontend Engineer"),
		},
	}
	s := newTestScheduler(userRepo, fuRepo)

	s.tick(context.Background())

	if got := atomic.LoadInt32(&userRepo.markCalls); got != 4 {
		t.Errorf("MarkFollowUpSent called %d times; want 4 (2 per user × 2 users)", got)
	}
}

// TestProcessUser_RepoError verifies that when GetApplicationsNeedingFollowUp
// returns an error, processUser logs it and continues — returning (0, 1) for
// (sent, failed).
func TestProcessUser_RepoError(t *testing.T) {
	t.Parallel()

	user := sampleUser("eve@example.com")
	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{user}}
	fuRepo := &mockFollowUpRepo{
		followUpsErr: errors.New("database connection lost"),
	}
	s := newTestScheduler(userRepo, fuRepo)

	sent, failed := s.processUser(context.Background(), user)

	if sent != 0 {
		t.Errorf("sent = %d; want 0 on repo error", sent)
	}
	if failed != 1 {
		t.Errorf("failed = %d; want 1 on repo error", failed)
	}

	// MarkFollowUpSent must not have been called.
	if got := atomic.LoadInt32(&userRepo.markCalls); got != 0 {
		t.Errorf("MarkFollowUpSent called %d times on error; want 0", got)
	}
}

// TestProcessUser_MarkSentError verifies that a MarkFollowUpSent failure is
// tolerated (logged as warning) and does not stop subsequent follow-ups.
func TestProcessUser_MarkSentError(t *testing.T) {
	t.Parallel()

	user := sampleUser("frank@example.com")
	userRepo := &mockFollowUpUserRepo{
		users: []FollowUpUser{user},
		markFollowUpSentFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("db write failed")
		},
	}
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{
			sampleFollowUp("StartupX", "DevOps Engineer"),
			sampleFollowUp("StartupY", "SRE"),
		},
	}
	s := newTestScheduler(userRepo, fuRepo)

	sent, failed := s.processUser(context.Background(), user)

	// Emails are sent (nil client is a no-op, not an error), so sent = 2.
	if sent != 2 {
		t.Errorf("sent = %d; want 2 (mark errors do not affect sent count)", sent)
	}
	if failed != 0 {
		t.Errorf("failed = %d; want 0 (mark errors are warnings, not failures)", failed)
	}
}

// TestProcessUser_FallbackName verifies the scheduler uses Email as display
// name when FullName is empty.
func TestProcessUser_FallbackName(t *testing.T) {
	t.Parallel()

	user := FollowUpUser{
		ID:       uuid.New(),
		Email:    "grace@example.com",
		FullName: "", // empty — should fall back to email
	}
	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{user}}
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{sampleFollowUp("MegaCorp", "QA Engineer")},
	}
	s := newTestScheduler(userRepo, fuRepo)

	// Should not panic — nil client sendFollowUpReminder is a no-op.
	sent, failed := s.processUser(context.Background(), user)
	if sent != 1 {
		t.Errorf("sent = %d; want 1", sent)
	}
	if failed != 0 {
		t.Errorf("failed = %d; want 0", failed)
	}
}

// TestTick_GetUsersError verifies that a repo error on GetAllActiveUsersForFollowUp
// causes tick() to bail out without calling MarkFollowUpSent.
func TestTick_GetUsersError(t *testing.T) {
	t.Parallel()

	userRepo := &mockFollowUpUserRepo{
		getUsersErr: errors.New("replica lag"),
	}
	fuRepo := &mockFollowUpRepo{}
	s := newTestScheduler(userRepo, fuRepo)

	s.tick(context.Background())

	if got := atomic.LoadInt32(&userRepo.markCalls); got != 0 {
		t.Errorf("MarkFollowUpSent called %d times after user fetch error; want 0", got)
	}
}

// TestRun_ExitsAfterFirstTick verifies that Run performs an immediate tick and
// then exits when the context is cancelled before the next hour tick.
func TestRun_ExitsAfterFirstTick(t *testing.T) {
	t.Parallel()

	user := sampleUser("henry@example.com")
	userRepo := &mockFollowUpUserRepo{users: []FollowUpUser{user}}
	fuRepo := &mockFollowUpRepo{
		followUps: []FollowUp{sampleFollowUp("BigFirm", "Analyst")},
	}
	s := newTestScheduler(userRepo, fuRepo)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Give enough time for the immediate tick to execute.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Good.
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit within 2s after context cancellation")
	}

	// The immediate tick must have run and called MarkFollowUpSent once.
	if got := atomic.LoadInt32(&userRepo.markCalls); got != 1 {
		t.Errorf("MarkFollowUpSent called %d times; want 1 from initial tick", got)
	}
}
