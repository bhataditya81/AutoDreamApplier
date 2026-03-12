package models

import (
	"time"

	"github.com/google/uuid"
)

// MatchStatus represents the state of a job match.
type MatchStatus string

const (
	MatchStatusPending   MatchStatus = "pending"   // Awaiting user review (review mode)
	MatchStatusApproved  MatchStatus = "approved"   // Approved for application
	MatchStatusRejected  MatchStatus = "rejected"   // User skipped / below auto-threshold
	MatchStatusApplied   MatchStatus = "applied"    // Application submitted
	MatchStatusExpired   MatchStatus = "expired"    // Application expired/failed
)

// Match represents a user-job pairing with a score.
type Match struct {
	ID            uuid.UUID              `db:"id"`
	UserID        uuid.UUID              `db:"user_id"`
	JobID         uuid.UUID              `db:"job_id"`
	MatchScore    float64                `db:"match_score"`
	MatchBreakdown map[string]float64    `db:"-"` // JSON in DB
	Status        MatchStatus            `db:"status"`
	CreatedAt     time.Time              `db:"created_at"`
	UpdatedAt     time.Time             `db:"updated_at"`
}

// ScoreBreakdown holds the per-dimension match scores (all 0-1).
type ScoreBreakdown struct {
	TitleScore    float64 `json:"title"`
	LocationScore float64 `json:"location"`
	SalaryScore   float64 `json:"salary"`
	SkillsScore   float64 `json:"skills"`
	SeniorityScore float64 `json:"seniority"`
}

// Weighted computes the final weighted score.
// Weights: title 30%, location 20%, salary 20%, skills 25%, seniority 5%.
func (s ScoreBreakdown) Weighted() float64 {
	return s.TitleScore*0.30 +
		s.LocationScore*0.20 +
		s.SalaryScore*0.20 +
		s.SkillsScore*0.25 +
		s.SeniorityScore*0.05
}
