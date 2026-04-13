package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/user/models"
	"github.com/bhata/AutoDreamApplier/pkg/config"
)

// ─── Fake in-memory store ─────────────────────────────────────────────────────

type fakeDevStore struct {
	users  map[string]*models.User
	hashes map[uuid.UUID]string
}

func newFakeDevStore() *fakeDevStore {
	return &fakeDevStore{
		users:  make(map[string]*models.User),
		hashes: make(map[uuid.UUID]string),
	}
}

func (s *fakeDevStore) FindByEmail(_ context.Context, email string) (*models.User, error) {
	u, ok := s.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (s *fakeDevStore) FindPasswordHash(_ context.Context, userID uuid.UUID) (string, error) {
	return s.hashes[userID], nil
}

func (s *fakeDevStore) Create(_ context.Context, req *models.CreateUserRequest) (*models.User, error) {
	id := uuid.New()
	u := &models.User{
		ID:         id,
		CognitoSub: req.CognitoSub,
		Email:      req.Email,
		FullName:   req.FullName,
	}
	s.users[req.Email] = u
	return u, nil
}

func (s *fakeDevStore) SetPasswordHash(_ context.Context, userID uuid.UUID, hash string) error {
	s.hashes[userID] = hash
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newDevHandler(store auth.DevUserStore) *auth.DevAuthHandler {
	return auth.NewDevAuthHandler(store, testDevSecret, zerolog.Nop())
}

func doPost(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// registerUser is a test helper that registers a user via the handler and returns the recorder.
func registerUser(t *testing.T, h *auth.DevAuthHandler, email, password, fullName string) {
	t.Helper()
	rr := doPost(t, h.Register, map[string]string{
		"email": email, "password": password, "fullName": fullName,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("registerUser: expected 201, got %d — %s", rr.Code, rr.Body.String())
	}
}

// ─── Registration tests ───────────────────────────────────────────────────────

func TestDevHandler_Register_Success(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	rr := doPost(t, h.Register, map[string]string{
		"email":    "new@example.com",
		"password": "secret123",
		"fullName": "New User",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
			User  struct {
				Email    string `json:"email"`
				FullName string `json:"fullName"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.Data.User.Email != "new@example.com" {
		t.Errorf("expected email new@example.com, got %q", resp.Data.User.Email)
	}
}

func TestDevHandler_Register_MissingFields(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	// Missing password
	rr := doPost(t, h.Register, map[string]string{"email": "x@example.com"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d", rr.Code)
	}
}

func TestDevHandler_Register_DuplicateEmail(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	// Register once
	doPost(t, h.Register, map[string]string{
		"email": "dup@example.com", "password": "pass", "fullName": "Dup",
	})

	// Register again with same email
	rr := doPost(t, h.Register, map[string]string{
		"email": "dup@example.com", "password": "pass", "fullName": "Dup",
	})
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate email, got %d", rr.Code)
	}
}

// ─── Login tests ──────────────────────────────────────────────────────────────

func TestDevHandler_Login_Success(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	registerUser(t, h, "login@example.com", "mypassword", "Login User")

	rr := doPost(t, h.Login, map[string]string{
		"email":    "login@example.com",
		"password": "mypassword",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty JWT token")
	}
}

func TestDevHandler_Login_WrongPassword(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	registerUser(t, h, "wp@example.com", "correct", "WP User")

	rr := doPost(t, h.Login, map[string]string{
		"email":    "wp@example.com",
		"password": "wrong",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", rr.Code)
	}
}

func TestDevHandler_Login_NonExistentEmail(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	rr := doPost(t, h.Login, map[string]string{
		"email":    "ghost@example.com",
		"password": "doesntmatter",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-existent email, got %d", rr.Code)
	}
}

func TestDevHandler_Login_MissingFields(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	rr := doPost(t, h.Login, map[string]string{"email": "x@example.com"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d", rr.Code)
	}
}

// TestDevHandler_Login_TokenIsValidatable verifies the issued JWT can be
// decoded by ValidateToken.
func TestDevHandler_Login_TokenIsValidatable(t *testing.T) {
	store := newFakeDevStore()
	h := newDevHandler(store)

	registerUser(t, h, "jwtcheck@example.com", "pw", "JWT User")

	rr := doPost(t, h.Login, map[string]string{
		"email": "jwtcheck@example.com", "password": "pw",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ca := auth.NewCognitoAuth(
		config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"},
		"us-east-1",
		zerolog.Nop(),
	)
	ca.WithDevSecret(testDevSecret)

	claims, err := ca.ValidateToken(context.Background(), resp.Data.Token)
	if err != nil {
		t.Fatalf("issued token is not valid: %v", err)
	}
	if claims.Email != "jwtcheck@example.com" {
		t.Errorf("email in claims mismatch: %q", claims.Email)
	}
}
