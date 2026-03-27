package models

import (
	"time"

	"github.com/google/uuid"
)

// JobSource represents a supported job board.
type JobSource string

const (
	SourceIndeed       JobSource = "indeed"
	SourceGlassdoor    JobSource = "glassdoor"
	SourceZipRecruiter JobSource = "ziprecruiter"
	SourceLinkedIn     JobSource = "linkedin" // Phase 2
)

// ATSType represents the applicant tracking system behind the apply button.
type ATSType string

const (
	ATSGreenhouse    ATSType = "greenhouse"
	ATSLever         ATSType = "lever"
	ATSWorkday       ATSType = "workday"
	ATSICIMS         ATSType = "icims"
	ATSTaleo         ATSType = "taleo"
	ATSSuccessFactors ATSType = "successfactors"
	ATSSmartRecruiters ATSType = "smartrecruiters"
	ATSBambooHR      ATSType = "bamboohr"
	ATSUnknown       ATSType = "unknown"
)

// Job represents a discovered job posting.
type Job struct {
	ID             uuid.UUID  `db:"id"`
	ExternalID     string     `db:"external_id"`
	SourceBoard    JobSource  `db:"source_board"`
	URL            string     `db:"url"`
	Title          string     `db:"title"`
	Company        string     `db:"company"`
	Location       string     `db:"location"`
	IsRemote       bool       `db:"is_remote"`
	SalaryMin      *int       `db:"salary_min"`
	SalaryMax      *int       `db:"salary_max"`
	SalaryCurrency string     `db:"salary_currency"`
	Description    string     `db:"description"`
	ATSType        ATSType    `db:"ats_type"`
	ApplyURL       string     `db:"apply_url"`
	IsActive       bool       `db:"is_active"`
	PostedAt       *time.Time `db:"posted_at"`
	DiscoveredAt   time.Time  `db:"discovered_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// ScrapedJob is the raw data returned from a scraper before saving.
// ATSType may be pre-populated by the scraper or discovery service when
// the ATS is known (e.g. from a board-specific API response). If left
// empty (or ATSUnknown), the repository will infer it from the ApplyURL.
type ScrapedJob struct {
	ExternalID     string
	Source         JobSource
	URL            string
	Title          string
	Company        string
	Location       string
	IsRemote       bool
	SalaryMin      *int
	SalaryMax      *int
	SalaryCurrency string
	Description    string
	ATSType        ATSType // optional: set by scraper/service, detected from URL if empty
	ApplyURL       string
	PostedAt       *time.Time
	// Scam detection fields — populated by the discovery service before DB insert.
	IsScam    bool
	ScamScore float64
}

// DiscoveryRun tracks a single scraping run for observability.
type DiscoveryRun struct {
	ID          uuid.UUID `db:"id"`
	Source      JobSource `db:"source"`
	StartedAt   time.Time `db:"started_at"`
	FinishedAt  *time.Time `db:"finished_at"`
	JobsFound   int        `db:"jobs_found"`
	JobsNew     int        `db:"jobs_new"`
	JobsDupe    int        `db:"jobs_dupe"`
	ErrorMsg    string     `db:"error_msg"`
	Success     bool       `db:"success"`
}
