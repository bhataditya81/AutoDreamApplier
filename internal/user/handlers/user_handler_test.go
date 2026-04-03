package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/testhelper"
	"github.com/bhata/AutoDreamApplier/internal/user/handlers"
	"github.com/bhata/AutoDreamApplier/internal/user/models"
	"github.com/bhata/AutoDreamApplier/internal/user/repository"
	pkgconfig "github.com/bhata/AutoDreamApplier/pkg/config"
	pkgs3 "github.com/bhata/AutoDreamApplier/pkg/s3"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

// newS3Stub creates a minimal S3-compatible stub server (PUT → 200, GET → 200).
func newS3Stub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "stub-pdf-content")
			return
		}
		w.Header().Set("ETag", `"stub-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newS3Client(t *testing.T, stubURL string) *pkgs3.Client {
	t.Helper()
	client, err := pkgs3.New(context.Background(), pkgconfig.S3Config{
		Endpoint:          stubURL,
		BucketResumes:     "test-resumes",
		BucketScreenshots: "test-screenshots",
	}, pkgconfig.AWSConfig{
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}, testhelper.NopLogger())
	if err != nil {
		t.Fatalf("newS3Client: %v", err)
	}
	return client
}

// ── Test router / auth injection ──────────────────────────────────────────────

// withClaims injects a fake auth.Claims into a request context (bypasses JWT middleware).
func withClaims(r *http.Request, sub, email string) *http.Request {
	claims := &auth.Claims{}
	claims.Sub = sub
	claims.Email = email
	ctx := context.WithValue(r.Context(), auth.UserContextKey, claims)
	return r.WithContext(ctx)
}

// newHandlerRouter builds a fully wired chi router for the user handler,
// pointing at the test DB and an optional S3 stub.
func newHandlerRouter(t *testing.T, s3URL string, encKey string) (http.Handler, *repository.UserRepository) {
	t.Helper()
	pool := testhelper.NewTestPool(t)
	repo := repository.NewUserRepository(pool, testhelper.NopLogger())

	var s3 *pkgs3.Client
	if s3URL != "" {
		s3 = newS3Client(t, s3URL)
	}

	h := handlers.NewUserHandler(repo, s3, "test-resumes", testhelper.NopLogger())
	if encKey != "" {
		h = h.WithEncryptionKey(encKey)
	}

	r := chi.NewRouter()
	r.Mount("/users", h.Routes())
	return r, repo
}

// doWithAuth builds a request, injects claims, fires it, and returns the recorder.
func doWithAuth(t *testing.T, router http.Handler, method, target string, sub, email string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("doWithAuth: marshal: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sub != "" {
		req = withClaims(req, sub, email)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ── seedUser helper ───────────────────────────────────────────────────────────

func seedTestUser(t *testing.T, ctx context.Context, repo *repository.UserRepository) (*models.User, string) {
	t.Helper()
	sub := fmt.Sprintf("dev:handler-%s@example.com", uuid.New())
	email := fmt.Sprintf("handler-%s@example.com", uuid.New())
	user, err := repo.Create(ctx, &models.CreateUserRequest{
		CognitoSub: sub,
		Email:      email,
		FullName:   "Handler Test User",
	})
	if err != nil {
		t.Fatalf("seedTestUser: %v", err)
	}
	t.Cleanup(func() {
		pool := testhelper.NewTestPool(t)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID) //nolint:errcheck
	})
	return user, sub
}

// ── GET /users/me ─────────────────────────────────────────────────────────────

func TestUserHandler_GetMe_ExistingUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodGet, "/users/me", sub, user.Email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /users/me: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("GET /users/me: missing 'data' envelope")
	}
	if data["email"] != user.Email {
		t.Errorf("GET /users/me: email = %v; want %s", data["email"], user.Email)
	}
}

func TestUserHandler_GetMe_AutoCreatesOnFirstVisit(t *testing.T) {
	t.Parallel()
	router, _ := newHandlerRouter(t, "", "")

	sub := fmt.Sprintf("dev:autocreate-%s@example.com", uuid.New())
	email := fmt.Sprintf("autocreate-%s@example.com", uuid.New())

	rr := doWithAuth(t, router, http.MethodGet, "/users/me", sub, email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /users/me (auto-create): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("GET /users/me (auto-create): missing 'data'")
	}
	if data["email"] != email {
		t.Errorf("GET /users/me (auto-create): email = %v; want %s", data["email"], email)
	}
}

func TestUserHandler_GetMe_Unauthenticated(t *testing.T) {
	t.Parallel()
	router, _ := newHandlerRouter(t, "", "")

	req := httptest.NewRequest(http.MethodGet, "/users/me", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /users/me (no auth): got %d; want %d", rr.Code, http.StatusUnauthorized)
	}
}

// ── PUT /users/me ─────────────────────────────────────────────────────────────

func TestUserHandler_UpdateProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]string{"full_name": "New Name"}
	rr := doWithAuth(t, router, http.MethodPut, "/users/me", sub, user.Email, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /users/me: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("PUT /users/me: missing 'data'")
	}
	if data["full_name"] != "New Name" {
		t.Errorf("PUT /users/me: full_name = %v; want 'New Name'", data["full_name"])
	}
}

// ── GET /users/me/preferences ─────────────────────────────────────────────────

func TestUserHandler_GetPreferences_Defaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodGet, "/users/me/preferences", sub, user.Email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /preferences: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

// ── PUT /users/me/preferences ─────────────────────────────────────────────────

func TestUserHandler_UpdatePreferences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]any{
		"target_titles": []string{"Software Engineer"},
		"locations":     []string{"New York"},
		"remote_pref":   "remote",
		"exclusions":    []string{},
	}
	rr := doWithAuth(t, router, http.MethodPut, "/users/me/preferences", sub, user.Email, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /preferences: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("PUT /preferences: missing 'data'")
	}
	titles, _ := data["target_titles"].([]any)
	if len(titles) != 1 {
		t.Errorf("PUT /preferences: target_titles len = %d; want 1", len(titles))
	}
}

func TestUserHandler_UpdatePreferences_NoTitles_BadRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]any{
		"target_titles": []string{},
		"remote_pref":   "remote",
	}
	rr := doWithAuth(t, router, http.MethodPut, "/users/me/preferences", sub, user.Email, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("PUT /preferences (no titles): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── GET /users/me/resumes ─────────────────────────────────────────────────────

func TestUserHandler_ListResumes_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodGet, "/users/me/resumes", sub, user.Email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /resumes (empty): got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].([]any)
	if data == nil {
		// data may be an empty array — check that the response was successful
		success, _ := resp["success"].(bool)
		if !success {
			t.Error("GET /resumes (empty): success = false")
		}
	}
}

// ── POST /users/me/resumes ────────────────────────────────────────────────────

func TestUserHandler_UploadResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s3Srv := newS3Stub(t)
	router, repo := newHandlerRouter(t, s3Srv.URL, "")

	user, sub := seedTestUser(t, ctx, repo)

	// Build a multipart form with a minimal PDF stub
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "test-resume.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	// Minimal valid-looking PDF header
	fmt.Fprint(fw, "%PDF-1.4 stub content for testing")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/users/me/resumes", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withClaims(req, sub, user.Email)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /resumes: got %d; want %d — body: %s", rr.Code, http.StatusCreated, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("POST /resumes: missing 'data'")
	}
	if data["file_name"] != "test-resume.pdf" {
		t.Errorf("POST /resumes: file_name = %v; want test-resume.pdf", data["file_name"])
	}
}

func TestUserHandler_UploadResume_InvalidType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s3Srv := newS3Stub(t)
	router, repo := newHandlerRouter(t, s3Srv.URL, "")

	user, sub := seedTestUser(t, ctx, repo)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("file", "resume.txt")
	fmt.Fprint(fw, "plain text")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/users/me/resumes", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = withClaims(req, sub, user.Email)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /resumes (bad type): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── DELETE /users/me/resumes/{resumeID} ───────────────────────────────────────

func TestUserHandler_DeleteResume_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodDelete,
		fmt.Sprintf("/users/me/resumes/%s", uuid.New()),
		sub, user.Email, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("DELETE /resumes/{nonexistent}: got %d; want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUserHandler_DeleteResume_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodDelete,
		"/users/me/resumes/not-a-uuid",
		sub, user.Email, nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("DELETE /resumes/bad-id: got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

// ── PUT /users/me/resumes/{resumeID}/primary ──────────────────────────────────

func TestUserHandler_SetPrimaryResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s3Srv := newS3Stub(t)
	router, repo := newHandlerRouter(t, s3Srv.URL, "")

	user, sub := seedTestUser(t, ctx, repo)

	// Insert a resume directly
	resume := &models.Resume{
		UserID:    user.ID,
		FileName:  "primary-test.pdf",
		S3Key:     fmt.Sprintf("resumes/%s/primary-test.pdf", user.ID),
		IsPrimary: false,
	}
	if err := repo.CreateResume(ctx, resume); err != nil {
		t.Fatalf("CreateResume: %v", err)
	}

	rr := doWithAuth(t, router, http.MethodPut,
		fmt.Sprintf("/users/me/resumes/%s/primary", resume.ID),
		sub, user.Email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /resumes/{id}/primary: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

// ── GET /users/me/dashboard ───────────────────────────────────────────────────

func TestUserHandler_GetDashboard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "")

	user, sub := seedTestUser(t, ctx, repo)

	rr := doWithAuth(t, router, http.MethodGet, "/users/me/dashboard", sub, user.Email, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /dashboard: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("GET /dashboard: missing 'data'")
	}
	if _, ok := data["user"]; !ok {
		t.Error("GET /dashboard: missing 'user' field")
	}
	if _, ok := data["resumes"]; !ok {
		t.Error("GET /dashboard: missing 'resumes' field")
	}
	if _, ok := data["stats"]; !ok {
		t.Error("GET /dashboard: missing 'stats' field")
	}
}

// ── POST /users/me/credentials ─────────────────────────────────────────────────

func TestUserHandler_SaveCredentials_OK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Valid 32-byte hex key
	encKey := strings.Repeat("ab", 32)
	router, repo := newHandlerRouter(t, "", encKey)

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]string{
		"boardName": "greenhouse",
		"email":     "jobseeker@example.com",
		"password":  "secret123",
	}
	rr := doWithAuth(t, router, http.MethodPost, "/users/me/credentials", sub, user.Email, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /credentials: got %d; want %d — body: %s", rr.Code, http.StatusOK, rr.Body)
	}
}

func TestUserHandler_SaveCredentials_MissingFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	encKey := strings.Repeat("ab", 32)
	router, repo := newHandlerRouter(t, "", encKey)

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]string{
		"boardName": "greenhouse",
		// missing email and password
	}
	rr := doWithAuth(t, router, http.MethodPost, "/users/me/credentials", sub, user.Email, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("POST /credentials (missing fields): got %d; want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_SaveCredentials_NoEncryptionKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	router, repo := newHandlerRouter(t, "", "") // no enc key

	user, sub := seedTestUser(t, ctx, repo)

	body := map[string]string{
		"boardName": "greenhouse",
		"email":     "user@example.com",
		"password":  "secret",
	}
	rr := doWithAuth(t, router, http.MethodPost, "/users/me/credentials", sub, user.Email, body)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("POST /credentials (no key): got %d; want %d", rr.Code, http.StatusInternalServerError)
	}
}
