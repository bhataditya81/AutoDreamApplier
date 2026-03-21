package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/testhelper"
	"github.com/bhata/AutoDreamApplier/internal/user/models"
	"github.com/bhata/AutoDreamApplier/internal/user/repository"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newRepo(t *testing.T) *repository.UserRepository {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	return repository.NewUserRepository(pool, testhelper.NopLogger())
}

// seedUser inserts a minimal user and registers cleanup.
func seedUser(t *testing.T, ctx context.Context, repo *repository.UserRepository, sub, email string) *models.User {
	t.Helper()
	user, err := repo.Create(ctx, &models.CreateUserRequest{
		CognitoSub: sub,
		Email:      email,
		FullName:   "Test User",
	})
	if err != nil {
		t.Fatalf("seedUser: %v", err)
	}
	t.Cleanup(func() {
		pool := testhelper.NewTestPool(t)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID) //nolint:errcheck
	})
	return user
}

// ── Create / FindByCognitoSub / FindByID ──────────────────────────────────────

func TestUserRepo_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:create-%s@example.com", uuid.New())
	email := fmt.Sprintf("create-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	if user.ID == uuid.Nil {
		t.Error("Create: expected non-nil ID")
	}
	if user.Email != email {
		t.Errorf("Create: email = %q; want %q", user.Email, email)
	}
	if user.CognitoSub != sub {
		t.Errorf("Create: cognito_sub = %q; want %q", user.CognitoSub, sub)
	}
	if !user.IsActive {
		t.Error("Create: expected is_active = true by default")
	}
}

func TestUserRepo_FindByCognitoSub_Found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:find-sub-%s@example.com", uuid.New())
	email := fmt.Sprintf("find-sub-%s@example.com", uuid.New())
	created := seedUser(t, ctx, repo, sub, email)

	found, err := repo.FindByCognitoSub(ctx, sub)
	if err != nil {
		t.Fatalf("FindByCognitoSub: %v", err)
	}
	if found == nil {
		t.Fatal("FindByCognitoSub: got nil; want user")
	}
	if found.ID != created.ID {
		t.Errorf("FindByCognitoSub: id = %s; want %s", found.ID, created.ID)
	}
}

func TestUserRepo_FindByCognitoSub_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	found, err := repo.FindByCognitoSub(ctx, "nonexistent-sub-xyz")
	if err != nil {
		t.Fatalf("FindByCognitoSub (not found): %v", err)
	}
	if found != nil {
		t.Errorf("FindByCognitoSub (not found): got user; want nil")
	}
}

func TestUserRepo_FindByID_Found(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:findid-%s@example.com", uuid.New())
	email := fmt.Sprintf("findid-%s@example.com", uuid.New())
	created := seedUser(t, ctx, repo, sub, email)

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found == nil {
		t.Fatal("FindByID: got nil; want user")
	}
	if found.Email != email {
		t.Errorf("FindByID: email = %q; want %q", found.Email, email)
	}
}

func TestUserRepo_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	found, err := repo.FindByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("FindByID (not found): %v", err)
	}
	if found != nil {
		t.Error("FindByID (not found): got user; want nil")
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUserRepo_Update_FullName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:update-%s@example.com", uuid.New())
	email := fmt.Sprintf("update-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	newName := "Updated Name"
	updated, err := repo.Update(ctx, user.ID, &models.UpdateUserRequest{FullName: &newName})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.FullName != newName {
		t.Errorf("Update: full_name = %q; want %q", updated.FullName, newName)
	}
}

func TestUserRepo_Update_ApplyMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:applymode-%s@example.com", uuid.New())
	email := fmt.Sprintf("applymode-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	mode := "auto"
	updated, err := repo.Update(ctx, user.ID, &models.UpdateUserRequest{ApplyMode: &mode})
	if err != nil {
		t.Fatalf("Update (apply_mode): %v", err)
	}
	if updated.ApplyMode != mode {
		t.Errorf("Update: apply_mode = %q; want %q", updated.ApplyMode, mode)
	}
}

func TestUserRepo_Update_NoFields_ReturnsUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:noupdate-%s@example.com", uuid.New())
	email := fmt.Sprintf("noupdate-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	updated, err := repo.Update(ctx, user.ID, &models.UpdateUserRequest{})
	if err != nil {
		t.Fatalf("Update (no fields): %v", err)
	}
	if updated.ID != user.ID {
		t.Errorf("Update (no fields): id = %s; want %s", updated.ID, user.ID)
	}
}

// ── GetPreferences / UpsertPreferences ───────────────────────────────────────

func TestUserRepo_GetPreferences_NotSet_ReturnsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:prefs-empty-%s@example.com", uuid.New())
	email := fmt.Sprintf("prefs-empty-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	prefs, err := repo.GetPreferences(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetPreferences (none): %v", err)
	}
	if prefs != nil {
		t.Errorf("GetPreferences (none): want nil; got %+v", prefs)
	}
}

func TestUserRepo_UpsertPreferences_Creates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:prefs-create-%s@example.com", uuid.New())
	email := fmt.Sprintf("prefs-create-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	req := &models.UpdatePreferencesRequest{
		TargetTitles: []string{"Software Engineer", "Backend Developer"},
		Locations:    []string{"New York", "Remote"},
		RemotePref:   "remote",
		Exclusions:   []string{"Sales"},
	}

	prefs, err := repo.UpsertPreferences(ctx, user.ID, req)
	if err != nil {
		t.Fatalf("UpsertPreferences (create): %v", err)
	}
	if prefs.UserID != user.ID {
		t.Errorf("UpsertPreferences: user_id = %s; want %s", prefs.UserID, user.ID)
	}
	if len(prefs.TargetTitles) != 2 {
		t.Errorf("UpsertPreferences: target_titles len = %d; want 2", len(prefs.TargetTitles))
	}
	if prefs.SalaryCurrency != "USD" {
		t.Errorf("UpsertPreferences: currency = %q; want USD (default)", prefs.SalaryCurrency)
	}
}

func TestUserRepo_UpsertPreferences_Updates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:prefs-update-%s@example.com", uuid.New())
	email := fmt.Sprintf("prefs-update-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	// First upsert
	req1 := &models.UpdatePreferencesRequest{
		TargetTitles: []string{"Engineer"},
		RemotePref:   "remote",
	}
	if _, err := repo.UpsertPreferences(ctx, user.ID, req1); err != nil {
		t.Fatalf("UpsertPreferences (first): %v", err)
	}

	// Second upsert — update titles
	req2 := &models.UpdatePreferencesRequest{
		TargetTitles: []string{"Senior Engineer", "Staff Engineer"},
		RemotePref:   "hybrid",
	}
	prefs, err := repo.UpsertPreferences(ctx, user.ID, req2)
	if err != nil {
		t.Fatalf("UpsertPreferences (update): %v", err)
	}
	if len(prefs.TargetTitles) != 2 {
		t.Errorf("UpsertPreferences (update): title count = %d; want 2", len(prefs.TargetTitles))
	}
	if prefs.RemotePref != "hybrid" {
		t.Errorf("UpsertPreferences (update): remote_pref = %q; want hybrid", prefs.RemotePref)
	}
}

// ── GetResumes / CreateResume / SetPrimaryResume / DeleteResume ───────────────

func TestUserRepo_GetResumes_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:resume-empty-%s@example.com", uuid.New())
	email := fmt.Sprintf("resume-empty-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	resumes, err := repo.GetResumes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetResumes (empty): %v", err)
	}
	if len(resumes) != 0 {
		t.Errorf("GetResumes (empty): got %d; want 0", len(resumes))
	}
}

func TestUserRepo_CreateResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:resume-create-%s@example.com", uuid.New())
	email := fmt.Sprintf("resume-create-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	resume := &models.Resume{
		UserID:    user.ID,
		FileName:  "my-cv.pdf",
		S3Key:     fmt.Sprintf("resumes/%s/my-cv.pdf", user.ID),
		IsPrimary: true,
		RawText:   "Go developer 5 years",
	}

	if err := repo.CreateResume(ctx, resume); err != nil {
		t.Fatalf("CreateResume: %v", err)
	}
	if resume.ID == uuid.Nil {
		t.Error("CreateResume: expected non-nil ID after insert")
	}

	// Verify it's retrievable
	resumes, err := repo.GetResumes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetResumes after create: %v", err)
	}
	if len(resumes) != 1 {
		t.Fatalf("GetResumes after create: got %d; want 1", len(resumes))
	}
	if resumes[0].FileName != "my-cv.pdf" {
		t.Errorf("GetResumes: file_name = %q; want my-cv.pdf", resumes[0].FileName)
	}
}

func TestUserRepo_SetPrimaryResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:setprimary-%s@example.com", uuid.New())
	email := fmt.Sprintf("setprimary-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	r1 := &models.Resume{UserID: user.ID, FileName: "cv1.pdf", S3Key: "s3/cv1.pdf", IsPrimary: true}
	r2 := &models.Resume{UserID: user.ID, FileName: "cv2.pdf", S3Key: "s3/cv2.pdf", IsPrimary: false}
	if err := repo.CreateResume(ctx, r1); err != nil {
		t.Fatalf("create r1: %v", err)
	}
	if err := repo.CreateResume(ctx, r2); err != nil {
		t.Fatalf("create r2: %v", err)
	}

	if err := repo.SetPrimaryResume(ctx, user.ID, r2.ID); err != nil {
		t.Fatalf("SetPrimaryResume: %v", err)
	}

	resumes, err := repo.GetResumes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetResumes: %v", err)
	}
	primaryCount := 0
	for _, res := range resumes {
		if res.IsPrimary {
			primaryCount++
			if res.ID != r2.ID {
				t.Errorf("SetPrimaryResume: primary is %s; want %s", res.ID, r2.ID)
			}
		}
	}
	if primaryCount != 1 {
		t.Errorf("SetPrimaryResume: %d primaries; want exactly 1", primaryCount)
	}
}

func TestUserRepo_DeleteResume_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:delresume-%s@example.com", uuid.New())
	email := fmt.Sprintf("delresume-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	resume := &models.Resume{UserID: user.ID, FileName: "del.pdf", S3Key: "s3/del.pdf", IsPrimary: false}
	if err := repo.CreateResume(ctx, resume); err != nil {
		t.Fatalf("CreateResume: %v", err)
	}

	if err := repo.DeleteResume(ctx, user.ID, resume.ID); err != nil {
		t.Fatalf("DeleteResume: %v", err)
	}

	resumes, err := repo.GetResumes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetResumes after delete: %v", err)
	}
	if len(resumes) != 0 {
		t.Errorf("DeleteResume: %d resumes remain; want 0", len(resumes))
	}
}

func TestUserRepo_DeleteResume_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:delnf-%s@example.com", uuid.New())
	email := fmt.Sprintf("delnf-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	err := repo.DeleteResume(ctx, user.ID, uuid.New())
	if err == nil {
		t.Error("DeleteResume (not found): expected error; got nil")
	}
	if err != nil && err.Error() != "resume not found" {
		t.Errorf("DeleteResume (not found): error = %q; want %q", err.Error(), "resume not found")
	}
}

// ── GetStats ──────────────────────────────────────────────────────────────────

func TestUserRepo_GetStats_ZeroForNewUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:stats-%s@example.com", uuid.New())
	email := fmt.Sprintf("stats-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	stats, err := repo.GetStats(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalApplications != 0 {
		t.Errorf("GetStats: total_applications = %d; want 0", stats.TotalApplications)
	}
	if stats.PendingMatches != 0 {
		t.Errorf("GetStats: pending_matches = %d; want 0", stats.PendingMatches)
	}
}

// ── SaveBoardCredential ───────────────────────────────────────────────────────

func TestUserRepo_SaveBoardCredential(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepo(t)

	sub := fmt.Sprintf("dev:creds-%s@example.com", uuid.New())
	email := fmt.Sprintf("creds-%s@example.com", uuid.New())
	user := seedUser(t, ctx, repo, sub, email)

	ciphertext := []byte("encrypted-payload")
	iv := []byte("iv-bytes-here")

	if err := repo.SaveBoardCredential(ctx, user.ID, "greenhouse", ciphertext, iv); err != nil {
		t.Fatalf("SaveBoardCredential: %v", err)
	}

	// Upsert same board — should not error
	if err := repo.SaveBoardCredential(ctx, user.ID, "greenhouse", []byte("new-payload"), iv); err != nil {
		t.Fatalf("SaveBoardCredential (upsert): %v", err)
	}
}
