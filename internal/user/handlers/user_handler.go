package handlers

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/internal/crypto"
	"github.com/bhata/AutoDreamApplier/internal/notification"
	resumeparser "github.com/bhata/AutoDreamApplier/internal/resume"
	"github.com/bhata/AutoDreamApplier/internal/user/models"
	"github.com/bhata/AutoDreamApplier/internal/user/repository"
	"github.com/bhata/AutoDreamApplier/pkg/response"
	s3client "github.com/bhata/AutoDreamApplier/pkg/s3"
)

const maxResumeSize = 10 << 20 // 10MB

// UserHandler handles user-related HTTP requests.
type UserHandler struct {
	repo           *repository.UserRepository
	s3             *s3client.Client
	bucketResumes  string
	encryptionKey  string // hex-encoded 32-byte key for AES-256-GCM
	webhookService *notification.WebhookService
	log            zerolog.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(repo *repository.UserRepository, s3 *s3client.Client, bucketResumes string, log zerolog.Logger) *UserHandler {
	return &UserHandler{repo: repo, s3: s3, bucketResumes: bucketResumes, log: log}
}

// WithWebhookService sets the WebhookService used to validate webhook URLs on save.
func (h *UserHandler) WithWebhookService(ws *notification.WebhookService) *UserHandler {
	h.webhookService = ws
	return h
}

// WithEncryptionKey sets the AES-256-GCM encryption key (hex-encoded, 32 bytes).
func (h *UserHandler) WithEncryptionKey(key string) *UserHandler {
	h.encryptionKey = key
	return h
}

// Routes returns the user routes.
func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/me", h.GetCurrentUser)
	r.Put("/me", h.UpdateProfile)
	r.Get("/me/preferences", h.GetPreferences)
	r.Put("/me/preferences", h.UpdatePreferences)
	r.Get("/me/resumes", h.ListResumes)
	r.Post("/me/resumes", h.UploadResume)
	r.Put("/me/resumes/{resumeID}/primary", h.SetPrimaryResume)
	r.Put("/me/resumes/{resumeID}/ab-test", h.UpdateResumeAB)
	r.Delete("/me/resumes/{resumeID}", h.DeleteResume)
	r.Get("/me/dashboard", h.GetDashboard)
	r.Post("/me/credentials", h.SaveBoardCredentials)

	return r
}

// GetCurrentUser returns the current authenticated user.
func (h *UserHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "not authenticated")
		return
	}

	user, err := h.repo.FindByCognitoSub(r.Context(), claims.Sub)
	if err != nil {
		h.log.Error().Err(err).Msg("error finding user")
		response.InternalError(w, "failed to retrieve user")
		return
	}

	if user == nil {
		// First-time login: auto-create user from Cognito claims
		user, err = h.repo.Create(r.Context(), &models.CreateUserRequest{
			CognitoSub: claims.Sub,
			Email:      claims.Email,
		})
		if err != nil {
			h.log.Error().Err(err).Msg("error creating user")
			response.InternalError(w, "failed to create user")
			return
		}
	}

	response.JSON(w, http.StatusOK, user)
}

// UpdateProfile updates the current user's profile.
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "not authenticated")
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	user, err := h.repo.FindByCognitoSub(r.Context(), claims.Sub)
	if err != nil || user == nil {
		response.NotFound(w, "user not found")
		return
	}

	updated, err := h.repo.Update(r.Context(), user.ID, &req)
	if err != nil {
		h.log.Error().Err(err).Msg("error updating user")
		response.InternalError(w, "failed to update user")
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

// GetPreferences returns the current user's job preferences.
func (h *UserHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	prefs, err := h.repo.GetPreferences(r.Context(), user.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("error getting preferences")
		response.InternalError(w, "failed to retrieve preferences")
		return
	}

	if prefs == nil {
		prefs = &models.UserPreferences{UserID: user.ID}
	}

	response.JSON(w, http.StatusOK, prefs)
}

// UpdatePreferences updates the current user's job preferences.
func (h *UserHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	var req models.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if len(req.TargetTitles) == 0 {
		response.BadRequest(w, "at least one target title is required")
		return
	}

	prefs, err := h.repo.UpsertPreferences(r.Context(), user.ID, &req)
	if err != nil {
		h.log.Error().Err(err).Msg("error updating preferences")
		response.InternalError(w, "failed to update preferences")
		return
	}

	response.JSON(w, http.StatusOK, prefs)
}

// ListResumes returns all resumes for the current user.
func (h *UserHandler) ListResumes(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	resumes, err := h.repo.GetResumes(r.Context(), user.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("error getting resumes")
		response.InternalError(w, "failed to retrieve resumes")
		return
	}

	if resumes == nil {
		resumes = []models.Resume{}
	}

	response.JSON(w, http.StatusOK, resumes)
}

// UploadResume handles resume file upload.
func (h *UserHandler) UploadResume(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	// Limit file size
	r.Body = http.MaxBytesReader(w, r.Body, maxResumeSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, "resume file is required (max 10MB)")
		return
	}
	defer file.Close()

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" && ext != ".docx" {
		response.BadRequest(w, "only PDF and DOCX files are supported")
		return
	}

	contentType := "application/pdf"
	if ext == ".docx" {
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	// Read file
	data, err := io.ReadAll(file)
	if err != nil {
		response.BadRequest(w, "failed to read file")
		return
	}

	// Generate S3 key
	resumeID := uuid.New()
	s3Key := fmt.Sprintf("resumes/%s/%s%s", user.ID.String(), resumeID.String(), ext)

	// Upload to S3
	if err := h.s3.PutBytes(r.Context(), h.bucketResumes, s3Key, data, contentType); err != nil {
		h.log.Error().Err(err).Msg("error uploading resume to S3")
		response.InternalError(w, "failed to upload resume")
		return
	}

	// Check if this is the first resume (auto-set as primary)
	existingResumes, _ := h.repo.GetResumes(r.Context(), user.ID)
	isPrimary := len(existingResumes) == 0

	// Extract plain text from the file so the AI service can tailor it later.
	// Parsing failures are non-fatal: the file is already safely stored in S3.
	rawText, parseErr := resumeparser.ParseText(data, ext)
	if parseErr != nil {
		h.log.Warn().
			Err(parseErr).
			Str("file_name", header.Filename).
			Msg("resume text extraction failed; continuing without raw_text")
	}

	// Save to DB
	resume := &models.Resume{
		UserID:    user.ID,
		FileName:  header.Filename,
		S3Key:     s3Key,
		IsPrimary: isPrimary,
		RawText:   rawText,
		// ParsedJSON is populated later by the AI prep worker (Stage 1 pipeline).
	}

	if err := h.repo.CreateResume(r.Context(), resume); err != nil {
		h.log.Error().Err(err).Msg("error saving resume record")
		response.InternalError(w, "failed to save resume")
		return
	}

	h.log.Info().
		Str("user_id", user.ID.String()).
		Str("resume_id", resume.ID.String()).
		Str("file_name", header.Filename).
		Msg("resume uploaded")

	response.JSON(w, http.StatusCreated, &models.UploadResumeResponse{
		ID:        resume.ID,
		FileName:  resume.FileName,
		IsPrimary: resume.IsPrimary,
		CreatedAt: resume.CreatedAt,
	})
}

// SetPrimaryResume marks a specific resume as the primary one.
func (h *UserHandler) SetPrimaryResume(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	resumeIDStr := chi.URLParam(r, "resumeID")
	resumeID, err := uuid.Parse(resumeIDStr)
	if err != nil {
		response.BadRequest(w, "invalid resume ID")
		return
	}

	if err := h.repo.SetPrimaryResume(r.Context(), user.ID, resumeID); err != nil {
		if errors.Is(err, repository.ErrResumeNotFound) {
			response.NotFound(w, "resume not found")
			return
		}
		h.log.Error().Err(err).Msg("error setting primary resume")
		response.InternalError(w, "failed to set primary resume")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "primary resume updated"})
}

// GetDashboard returns the full dashboard data for the current user.
func (h *UserHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	prefs, _ := h.repo.GetPreferences(r.Context(), user.ID)
	resumes, _ := h.repo.GetResumes(r.Context(), user.ID)
	stats, _ := h.repo.GetStats(r.Context(), user.ID)

	if resumes == nil {
		resumes = []models.Resume{}
	}

	dashboard := &models.UserDashboard{
		User:        user,
		Preferences: prefs,
		Resumes:     resumes,
		Stats:       stats,
	}

	response.JSON(w, http.StatusOK, dashboard)
}

// getCurrentUser is a helper to get the current user from context.
func (h *UserHandler) getCurrentUser(r *http.Request) (*models.User, error) {
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		return nil, fmt.Errorf("not authenticated")
	}

	user, err := h.repo.FindByCognitoSub(r.Context(), claims.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// DeleteResume deletes a resume owned by the current user.
func (h *UserHandler) DeleteResume(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	resumeIDStr := chi.URLParam(r, "resumeID")
	resumeID, err := uuid.Parse(resumeIDStr)
	if err != nil {
		response.BadRequest(w, "invalid resume ID")
		return
	}

	if err := h.repo.DeleteResume(r.Context(), user.ID, resumeID); err != nil {
		if err.Error() == "resume not found" {
			response.NotFound(w, "resume not found")
			return
		}
		h.log.Error().Err(err).Msg("error deleting resume")
		response.InternalError(w, "failed to delete resume")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateResumeABRequest is the request body for PUT /me/resumes/{resumeID}/ab-test.
type updateResumeABRequest struct {
	AbEnabled bool `json:"abEnabled"`
	AbWeight  int  `json:"abWeight"`
}

// UpdateResumeAB enables or disables A/B testing for a specific resume and sets its weight.
func (h *UserHandler) UpdateResumeAB(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	resumeIDStr := chi.URLParam(r, "resumeID")
	resumeID, err := uuid.Parse(resumeIDStr)
	if err != nil {
		response.BadRequest(w, "invalid resume ID")
		return
	}

	var req updateResumeABRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.AbWeight < 1 || req.AbWeight > 10 {
		response.BadRequest(w, "abWeight must be between 1 and 10")
		return
	}

	if err := h.repo.UpdateResumeAB(r.Context(), resumeID, user.ID, req.AbEnabled, req.AbWeight); err != nil {
		if err.Error() == "resume not found" {
			response.NotFound(w, "resume not found")
			return
		}
		h.log.Error().Err(err).Msg("error updating resume A/B settings")
		response.InternalError(w, "failed to update A/B settings")
		return
	}

	// Return updated resume list so the client can refresh
	resumes, err := h.repo.GetResumes(r.Context(), user.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("error fetching resumes after AB update")
		response.InternalError(w, "failed to retrieve updated resumes")
		return
	}

	// Find and return the updated resume
	for i := range resumes {
		if resumes[i].ID == resumeID {
			response.JSON(w, http.StatusOK, resumes[i])
			return
		}
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "A/B settings updated"})
}

// saveBoardCredentialsRequest is the request body for POST /me/credentials.
type saveBoardCredentialsRequest struct {
	BoardName string `json:"boardName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// SaveBoardCredentials encrypts and stores ATS board credentials.
func (h *UserHandler) SaveBoardCredentials(w http.ResponseWriter, r *http.Request) {
	user, err := h.getCurrentUser(r)
	if err != nil {
		response.Unauthorized(w, "not authenticated")
		return
	}

	var req saveBoardCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if req.BoardName == "" || req.Email == "" || req.Password == "" {
		response.BadRequest(w, "boardName, email, and password are required")
		return
	}

	// Encrypt the credentials.
	if h.encryptionKey == "" {
		h.log.Error().Msg("ENCRYPTION_KEY not set; cannot store credentials")
		response.InternalError(w, "server misconfiguration: credentials cannot be stored")
		return
	}
	keyBytes, err := hex.DecodeString(h.encryptionKey)
	if err != nil || len(keyBytes) != 32 {
		h.log.Error().Msg("ENCRYPTION_KEY must be 32 hex-encoded bytes")
		response.InternalError(w, "server misconfiguration: invalid encryption key")
		return
	}
	enc, err := crypto.NewEncryptor(keyBytes)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create encryptor")
		response.InternalError(w, "failed to encrypt credentials")
		return
	}

	// Combine email+password as the plaintext blob.
	plaintext := []byte(req.Email + ":" + req.Password)
	ciphertext, iv, err := enc.Encrypt(plaintext)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to encrypt credentials")
		response.InternalError(w, "failed to encrypt credentials")
		return
	}

	if err := h.repo.SaveBoardCredential(r.Context(), user.ID, req.BoardName, ciphertext, iv); err != nil {
		h.log.Error().Err(err).Msg("error saving board credential")
		response.InternalError(w, "failed to save credentials")
		return
	}

	h.log.Info().
		Str("user_id", user.ID.String()).
		Str("board", req.BoardName).
		Msg("board credentials saved")

	response.JSON(w, http.StatusOK, map[string]string{"message": "credentials saved"})
}
