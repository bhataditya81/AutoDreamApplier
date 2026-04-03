package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
type User struct {
	ID            uuid.UUID `json:"id" db:"id"`
	CognitoSub    string    `json:"cognito_sub" db:"cognito_sub"`
	Email         string    `json:"email" db:"email"`
	FullName      string    `json:"full_name" db:"full_name"`
	Tier          string    `json:"tier" db:"tier"`
	ApplyMode     string    `json:"apply_mode" db:"apply_mode"`
	AutoThreshold float64   `json:"auto_threshold" db:"auto_threshold"`
	DailyLimit    int       `json:"daily_limit" db:"daily_limit"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// UserPreferences represents user's job search preferences.
type UserPreferences struct {
	ID                    uuid.UUID `json:"id" db:"id"`
	UserID                uuid.UUID `json:"user_id" db:"user_id"`
	TargetTitles          []string  `json:"targetTitles" db:"target_titles"`
	Locations             []string  `json:"locations" db:"locations"`
	RemotePref            string    `json:"remotePref" db:"remote_pref"`
	SalaryMin             *int      `json:"salaryMin" db:"salary_min"`
	SalaryMax             *int      `json:"salaryMax" db:"salary_max"`
	SalaryCurrency        string    `json:"salaryCurrency" db:"salary_currency"`
	Exclusions            []string  `json:"exclusions" db:"exclusions"`
	AiTailorEnabled       bool      `json:"ai_tailor_enabled" db:"ai_tailor_enabled"`
	AutoApplyEnabled      bool      `json:"autoApplyEnabled" db:"auto_apply_enabled"`
	DailyApplicationLimit int       `json:"dailyApplicationLimit" db:"daily_application_limit"`
	ApplyStartHour        int       `json:"applyStartHour" db:"apply_start_hour"`
	ApplyEndHour          int       `json:"applyEndHour" db:"apply_end_hour"`
	ApplyTimezone         string    `json:"applyTimezone" db:"apply_timezone"`
	SlackWebhookURL       string    `json:"slackWebhookUrl,omitempty" db:"slack_webhook_url"`
	DiscordWebhookURL     string    `json:"discordWebhookUrl,omitempty" db:"discord_webhook_url"`
	WebhookEvents         []string  `json:"webhookEvents,omitempty" db:"webhook_events"`
	EmailDigestEnabled    bool      `json:"emailDigestEnabled" db:"email_digest_enabled"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// Resume represents an uploaded resume.
type Resume struct {
	ID                uuid.UUID `json:"id" db:"id"`
	UserID            uuid.UUID `json:"user_id" db:"user_id"`
	FileName          string    `json:"file_name" db:"file_name"`
	S3Key             string    `json:"s3_key" db:"s3_key"`
	ParsedJSON        []byte    `json:"parsed_json,omitempty" db:"parsed_json"`
	RawText           string    `json:"raw_text,omitempty" db:"raw_text"`
	IsPrimary         bool      `json:"is_primary" db:"is_primary"`
	InterviewCount    int       `json:"interview_count" db:"interview_count"`
	AbEnabled         bool      `json:"ab_enabled" db:"ab_enabled"`
	AbWeight          int       `json:"ab_weight" db:"ab_weight"`
	TotalApplications int       `json:"total_applications" db:"total_applications"`
	TotalViews        int       `json:"total_views" db:"total_views"`
	TotalInterviews   int       `json:"total_interviews" db:"total_interviews"`
	TotalOffers       int       `json:"total_offers" db:"total_offers"`
	// InterviewRate is a computed field (not stored): TotalInterviews / TotalApplications * 100
	InterviewRate float64   `json:"interview_rate" db:"-"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// CreateUserRequest is the payload for creating/syncing a user from Cognito.
type CreateUserRequest struct {
	CognitoSub string `json:"cognito_sub" validate:"required"`
	Email      string `json:"email" validate:"required,email"`
	FullName   string `json:"full_name"`
}

// UpdateUserRequest is the payload for updating a user's profile.
type UpdateUserRequest struct {
	FullName      *string  `json:"full_name,omitempty"`
	ApplyMode     *string  `json:"apply_mode,omitempty" validate:"omitempty,oneof=review auto"`
	AutoThreshold *float64 `json:"auto_threshold,omitempty" validate:"omitempty,min=0,max=1"`
}

// UpdatePreferencesRequest is the payload for updating job preferences.
type UpdatePreferencesRequest struct {
	TargetTitles          []string `json:"targetTitles" validate:"required,min=1"`
	Locations             []string `json:"locations"`
	RemotePref            string   `json:"remotePref" validate:"required,oneof=remote hybrid onsite any"`
	SalaryMin             *int     `json:"salaryMin" validate:"omitempty,min=0"`
	SalaryMax             *int     `json:"salaryMax" validate:"omitempty,min=0"`
	SalaryCurrency        string   `json:"salaryCurrency" validate:"omitempty,len=3"`
	Exclusions            []string `json:"exclusions"`
	AiTailorEnabled       *bool    `json:"ai_tailor_enabled,omitempty"`
	AutoApplyEnabled      *bool    `json:"autoApplyEnabled,omitempty"`
	DailyApplicationLimit *int     `json:"dailyApplicationLimit,omitempty"`
	ApplyStartHour        *int     `json:"applyStartHour,omitempty"`
	ApplyEndHour          *int     `json:"applyEndHour,omitempty"`
	ApplyTimezone         string   `json:"applyTimezone,omitempty"`
	SlackWebhookURL       string   `json:"slackWebhookUrl,omitempty"`
	DiscordWebhookURL     string   `json:"discordWebhookUrl,omitempty"`
	WebhookEvents         []string `json:"webhookEvents,omitempty"`
	EmailDigestEnabled    *bool    `json:"emailDigestEnabled"`
}

// UploadResumeResponse is returned after resume upload.
type UploadResumeResponse struct {
	ID        uuid.UUID `json:"id"`
	FileName  string    `json:"file_name"`
	IsPrimary bool      `json:"is_primary"`
	CreatedAt time.Time `json:"created_at"`
}

// UserDashboard combines user data for the dashboard view.
type UserDashboard struct {
	User        *User            `json:"user"`
	Preferences *UserPreferences `json:"preferences"`
	Resumes     []Resume         `json:"resumes"`
	Stats       *UserStats       `json:"stats"`
}

// UserStats holds summary statistics for a user.
type UserStats struct {
	TotalApplications  int `json:"total_applications"`
	AppliedToday       int `json:"applied_today"`
	PendingMatches     int `json:"pending_matches"`
	InterviewsReceived int `json:"interviews_received"`
}
