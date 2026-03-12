// Package auth contains authentication helpers.
// dev_handler.go is ONLY used in non-production environments.
// It provides simple email/password login backed by bcrypt + a self-signed
// HS256 JWT so the dashboard can be developed without AWS Cognito.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/bhata/AutoDreamApplier/internal/user/models"
	"github.com/bhata/AutoDreamApplier/pkg/response"
)

// DevUserStore is the minimal repository surface needed by DevAuthHandler.
type DevUserStore interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindPasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	Create(ctx context.Context, req *models.CreateUserRequest) (*models.User, error)
	SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
}

// DevAuthHandler handles dev-only login and registration.
type DevAuthHandler struct {
	store  DevUserStore
	secret string
	log    zerolog.Logger
}

// NewDevAuthHandler creates a new DevAuthHandler.
// secret is the HMAC key used to sign JWTs (from DEV_JWT_SECRET env var).
func NewDevAuthHandler(store DevUserStore, secret string, log zerolog.Logger) *DevAuthHandler {
	return &DevAuthHandler{store: store, secret: secret, log: log}
}

// devLoginRequest is the payload for POST /api/v1/auth/login.
type devLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// devRegisterRequest is the payload for POST /api/v1/auth/register.
type devRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
}

// devAuthResponse is the response for both login and register.
type devAuthResponse struct {
	Token string      `json:"token"`
	User  devUserInfo `json:"user"`
}

type devUserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
}

// Login handles POST /api/v1/auth/login.
func (h *DevAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req devLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		response.BadRequest(w, "email and password are required")
		return
	}

	user, err := h.store.FindByEmail(r.Context(), req.Email)
	if err != nil {
		h.log.Error().Err(err).Msg("dev login: FindByEmail")
		response.InternalError(w, "login failed")
		return
	}
	if user == nil {
		response.Unauthorized(w, "invalid email or password")
		return
	}

	hash, err := h.store.FindPasswordHash(r.Context(), user.ID)
	if err != nil || hash == "" {
		response.Unauthorized(w, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		response.Unauthorized(w, "invalid email or password")
		return
	}

	token, err := mintDevToken(user.CognitoSub, user.Email, h.secret)
	if err != nil {
		h.log.Error().Err(err).Msg("dev login: mint token")
		response.InternalError(w, "failed to generate token")
		return
	}

	response.JSON(w, http.StatusOK, devAuthResponse{
		Token: token,
		User:  devUserInfo{ID: user.ID.String(), Email: user.Email, FullName: user.FullName},
	})
}

// Register handles POST /api/v1/auth/register.
func (h *DevAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req devRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		response.BadRequest(w, "email, password, and fullName are required")
		return
	}

	// Check if user already exists.
	existing, err := h.store.FindByEmail(r.Context(), req.Email)
	if err != nil {
		h.log.Error().Err(err).Msg("dev register: FindByEmail")
		response.InternalError(w, "registration failed")
		return
	}
	if existing != nil {
		response.BadRequest(w, "email already registered")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error().Err(err).Msg("dev register: bcrypt")
		response.InternalError(w, "registration failed")
		return
	}

	// Use email as the cognito_sub surrogate for dev users.
	user, err := h.store.Create(r.Context(), &models.CreateUserRequest{
		CognitoSub: "dev:" + req.Email,
		Email:      req.Email,
		FullName:   req.FullName,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("dev register: Create user")
		response.InternalError(w, "registration failed")
		return
	}

	if err := h.store.SetPasswordHash(r.Context(), user.ID, string(hashed)); err != nil {
		h.log.Error().Err(err).Msg("dev register: SetPasswordHash")
		// non-fatal: user created, but can't log in without hash — surface error
		response.InternalError(w, "registration failed: could not store credentials")
		return
	}

	token, err := mintDevToken(user.CognitoSub, user.Email, h.secret)
	if err != nil {
		h.log.Error().Err(err).Msg("dev register: mint token")
		response.InternalError(w, "registered, but failed to generate token")
		return
	}

	h.log.Info().Str("email", user.Email).Msg("dev user registered")

	response.JSON(w, http.StatusCreated, devAuthResponse{
		Token: token,
		User:  devUserInfo{ID: user.ID.String(), Email: user.Email, FullName: user.FullName},
	})
}

// mintDevToken issues a signed HS256 JWT valid for 24 hours.
func mintDevToken(userID, email, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":       userID,
		"email":     email,
		"token_use": "id",
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"iss":       "autodream-dev",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}
