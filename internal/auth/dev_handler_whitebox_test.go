// White-box tests for dev_handler.go that require error injection.
// Uses package auth for internal type access (if needed), but all tested via public API.
// Actually all test through public handlers so this is package auth_test, just with
// a different file for error-path tests using a richer fake store.
package auth_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/user/models"
)

// errorStore is a store implementation that returns errors for testing error paths.
type errorStore struct {
	findByEmailErr      error
	findByEmailResult   *models.User
	findPasswordHashErr error
	findPasswordHash    string
	createErr           error
	createResult        *models.User
	setPasswordHashErr  error
}

func (s *errorStore) FindByEmail(_ context.Context, _ string) (*models.User, error) {
	return s.findByEmailResult, s.findByEmailErr
}

func (s *errorStore) FindPasswordHash(_ context.Context, _ uuid.UUID) (string, error) {
	return s.findPasswordHash, s.findPasswordHashErr
}

func (s *errorStore) Create(_ context.Context, _ *models.CreateUserRequest) (*models.User, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createResult != nil {
		return s.createResult, nil
	}
	id := uuid.New()
	return &models.User{ID: id, CognitoSub: "dev:x@x.com", Email: "x@x.com"}, nil
}

func (s *errorStore) SetPasswordHash(_ context.Context, _ uuid.UUID, _ string) error {
	return s.setPasswordHashErr
}

func doPostRaw(t *testing.T, handler http.HandlerFunc, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// ─── Login error paths ────────────────────────────────────────────────────────

func TestDevHandler_Login_FindByEmailError(t *testing.T) {
	store := &errorStore{
		findByEmailErr: errors.New("db connection error"),
	}
	h := auth.NewDevAuthHandler(store, "secret", zerolog.Nop())

	rr := doPostRaw(t, h.Login, []byte(`{"email":"x@x.com","password":"pw"}`))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for FindByEmail error, got %d", rr.Code)
	}
}

func TestDevHandler_Login_FindPasswordHashError(t *testing.T) {
	id := uuid.New()
	store := &errorStore{
		findByEmailResult: &models.User{
			ID:         id,
			CognitoSub: "dev:x@x.com",
			Email:      "x@x.com",
		},
		findPasswordHashErr: errors.New("db error"),
	}
	h := auth.NewDevAuthHandler(store, "secret", zerolog.Nop())

	rr := doPostRaw(t, h.Login, []byte(`{"email":"x@x.com","password":"pw"}`))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when password hash fetch fails, got %d", rr.Code)
	}
}

// ─── Register error paths ─────────────────────────────────────────────────────

func TestDevHandler_Register_FindByEmailError(t *testing.T) {
	store := &errorStore{
		findByEmailErr: errors.New("db connection error"),
	}
	h := auth.NewDevAuthHandler(store, "secret", zerolog.Nop())

	rr := doPostRaw(t, h.Register, []byte(`{"email":"y@y.com","password":"pw","fullName":"Y"}`))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for FindByEmail error in Register, got %d", rr.Code)
	}
}

func TestDevHandler_Register_CreateUserError(t *testing.T) {
	store := &errorStore{
		createErr: errors.New("db insert error"),
	}
	h := auth.NewDevAuthHandler(store, "secret", zerolog.Nop())

	rr := doPostRaw(t, h.Register, []byte(`{"email":"z@z.com","password":"pw","fullName":"Z"}`))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for Create error, got %d", rr.Code)
	}
}

func TestDevHandler_Register_SetPasswordHashError(t *testing.T) {
	id := uuid.New()
	store := &errorStore{
		createResult: &models.User{
			ID:         id,
			CognitoSub: "dev:a@a.com",
			Email:      "a@a.com",
		},
		setPasswordHashErr: errors.New("hash store error"),
	}
	h := auth.NewDevAuthHandler(store, "secret", zerolog.Nop())

	rr := doPostRaw(t, h.Register, []byte(`{"email":"a@a.com","password":"pw","fullName":"A"}`))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for SetPasswordHash error, got %d", rr.Code)
	}
}
