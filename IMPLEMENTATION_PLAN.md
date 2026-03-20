# AutoDreamApplier — Implementation Plan

> Cloud-native, server-side job application automation SaaS.
> Go backend (Chi + Asynq) · Next.js 14 frontend · PostgreSQL + Redis · AWS Cognito auth.

## Status Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Done — implemented & tested |
| ⬜ | Todo — not yet started |
| 🔒 | Blocked — waiting on another task |

---

## ✅ MVP-A — COMPLETE

All MVP-A components are implemented, tested, and passing in local dev mode.

### Completed Components

| Component | Key Files | Notes |
|-----------|-----------|-------|
| Project scaffolding | `go.mod`, `cmd/`, `internal/`, `docker-compose.yml` | Go 1.22+ module layout |
| Database schema | `migrations/` | Users, resumes, jobs, matches, applications, board_credentials, application_events |
| Config system | `pkg/config/config.go` | Env-based, safe defaults for local dev |
| Logger | `pkg/logger/` | zerolog; production JSON / dev pretty-print |
| S3 client | `pkg/s3/` | Upload, download, delete, pre-signed URLs; MinIO in dev |
| Auth middleware | `internal/auth/cognito.go` | Cognito RSA JWT + dev HS256 JWT via `peekAlg()` + `WithDevSecret()` |
| Dev auth handler | `internal/auth/dev_handler.go` | bcrypt email/password → HS256 JWT; non-prod only |
| User handler | `internal/user/handlers/user_handler.go` | Profile CRUD, resume upload/list/delete/set-primary, credentials, dashboard stats, preferences |
| User repository | `internal/user/repository/` | All DB queries: users, resumes, credentials, preferences |
| Application service | `internal/application/service/` | approve, reject, submit, emergency stop |
| Application handler | `internal/application/handler/` | HTTP routes for all application actions |
| Application repository | `internal/application/repository/` | DB queries + Asynq task enqueue |
| 2-stage async pipeline | `internal/application/workers/` | `TypeAIPrep` → `TypeBrowserApply`; decoupled AI prep from browser time |
| AI prep worker | `internal/application/workers/ai_prep_worker.go` | Calls AI service, stores result to S3, advances status → `ai_ready`, enqueues browser task |
| Browser apply worker | `internal/application/workers/browser_apply_worker.go` | Acquires browser, fills ATS form, stores screenshot, status → `applied` or `failed` |
| ATS plugin system | `internal/ats/registry.go` | `Plugin` interface + type-keyed registry; Greenhouse plugin implemented |
| Greenhouse ATS plugin | `internal/ats/plugins/greenhouse.go` | Full reference implementation |
| Browser pool client | `internal/browser/` | HTTP client to EC2 browser pool |
| Job discovery service | `cmd/job-discovery/main.go` | Indeed scraper (real HTML); Glassdoor intentionally stubbed |
| Job matcher | `internal/jobmatcher/` | 5-dimension keyword scorer, exclusion filter, batch insert, pagination |
| Match repository | `internal/jobmatcher/repository/match_repository.go` | approve / reject / feedback writes |
| Notification client | `internal/notification/` | AWS SES email; nil-safe when unconfigured |
| API Gateway | `cmd/api-gateway/main.go` | All routes wired, graceful shutdown |
| Docker Compose | `docker-compose.yml` | Full dev stack with healthchecks, MinIO bucket auto-create, db-migrator pre-start |
| Frontend — auth | `frontend/src/app/(auth)/` | Register / login pages |
| Frontend — dashboard layout | `frontend/src/app/dashboard/layout.tsx` | Sidebar nav, auth guard |
| Frontend — match queue | `frontend/src/components/matches/match-queue.tsx` | Paginated, status tabs, approve/reject wired |
| Frontend — onboarding | `frontend/src/app/dashboard/onboarding/page.tsx` | 3-step wizard: resume → preferences → complete |
| Frontend — settings | `frontend/src/app/dashboard/settings/page.tsx` | ProfileForm, PreferencesForm, CredentialsForm |
| Frontend — preferences form | `frontend/src/components/settings/preferences-form.tsx` | Tag/chip UI, API-wired |
| Frontend — API client | `frontend/src/lib/api.ts` | All API calls |
| Frontend — TS types | `frontend/src/lib/types.ts` | camelCase types matching backend snake_case |
| Worker integration tests | `internal/application/workers/*_test.go` | Happy path, error path, bad payload |

### Bug Fixes Applied

| Bug | File | Fix |
|-----|------|-----|
| Dev HS256 JWT rejected by Cognito RSA middleware | `internal/auth/cognito.go` | `peekAlg()` routes HS256 → `validateDevToken`, RS256 → Cognito JWKS |
| Dev token `sub` was UUID; DB lookup uses `cognito_sub = "dev:email"` | `internal/auth/dev_handler.go` | `mintDevToken(user.CognitoSub, …)` — not `user.ID.String()` |
| Dev secret never wired into CognitoAuth | `cmd/api-gateway/main.go` | `cognitoAuth.WithDevSecret(cfg.App.DevJWTSecret)` when non-prod |
| Resume upload: handler expected `"resume"`, frontend sent `"file"` | `internal/user/handlers/user_handler.go:210` | `r.FormFile("file")` |
| `handleSetPrimary` used snake_case key `is_primary` not camelCase | `frontend/src/components/resumes/resume-list.tsx:70` | `isPrimary:` |

### Architecture Decisions (Locked)

1. **2-stage async pipeline** — AI prep (3–8s) decoupled from browser; browsers only allocated after content is ready. Cuts apply time ~70s → ~40s with zero idle browser waste.
2. **Dev HS256 JWT alongside Cognito RSA** — `peekAlg()` inspects JWT header; HS256 → dev path, RS256 → Cognito. Never active when `APP_ENV=production`.
3. **ATS plugin interface** — `internal/ats/registry.go` provides `Plugin` interface. New ATS = new file + register in `init()`. Zero changes to apply worker.
4. **Credential encryption** — AES-256-GCM via `internal/crypto/`. Empty `ENCRYPTION_KEY` → HTTP 500 with clear error; never silently stores plaintext.
5. **Conservative rate limits** — Indeed 10/day/user, Glassdoor 8/day/user, LinkedIn 3/day/user. Auto-pause on CAPTCHA frequency >3 per 10 minutes.

---

## 🚀 MVP-B — Active Development (Weeks 11–16)

### Developer Branch Map

All branches are **independent** and can be developed simultaneously. Integration points are documented in each task. Merge order: infra → job-boards → ats-plugins → auto-apply → ai-service → notifications → frontend-v2.

| Branch | Focus | Effort | Integrates With |
|--------|-------|--------|-----------------|
| `feat/infra-staging` | AWS staging + CI/CD | 5 days | All (deploy target) |
| `feat/job-boards` | ZipRecruiter + LinkedIn scrapers | 6 days | `feat/auto-apply` (match feed) |
| `feat/ats-plugins` | Lever + Workday + ATS auto-detect | 7 days | `feat/ai-service` (apply worker) |
| `feat/auto-apply` | Auto-approve engine + scheduling | 5 days | `feat/job-boards`, `feat/notifications` |
| `feat/ai-service` | Python FastAPI AI service | 8 days | `feat/ats-plugins` (worker calls it) |
| `feat/notifications` | Weekly digest + Slack/Discord | 4 days | `feat/auto-apply` (events) |
| `feat/frontend-v2` | Dashboard improvements | 7 days | All backend branches |

---

### BRANCH: `feat/infra-staging`

**Owner:** DevOps / Infrastructure developer
**Prerequisite:** None — start immediately
**Merge before:** Every other branch (provides deploy target)

---

#### INFRA-1: GitHub Actions CI Pipeline

**Files to Create:**
- `.github/workflows/ci.yml`
- `.github/workflows/deploy-staging.yml`

**`.github/workflows/ci.yml` spec:**
```yaml
# Triggers: push to any branch, PRs to main
# Jobs (run in parallel):
#   test-backend:
#     - go test ./... with TEST_DATABASE_URL pointing to service postgres
#     - go vet ./...
#     - golangci-lint run
#   test-frontend:
#     - npm ci && npm run build && npm run lint
#   docker-build:
#     - docker build each service (no push on PR)
```

**Services needed in CI:**
```yaml
services:
  postgres:
    image: postgres:16
    env: { POSTGRES_DB: testdb, POSTGRES_USER: test, POSTGRES_PASSWORD: test }
    options: --health-cmd "pg_isready" --health-interval 5s
  redis:
    image: redis:7-alpine
    options: --health-cmd "redis-cli ping" --health-interval 5s
```

**Environment variables for CI:**
```
TEST_DATABASE_URL=postgres://test:test@localhost:5432/testdb?sslmode=disable
APP_ENV=test
DEV_JWT_SECRET=ci-test-secret-32byteslong12345
ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
```

**Files to Modify:**
- `Makefile` — add `make ci` target that mirrors CI steps locally

**Acceptance Criteria:**
- [ ] `go test ./...` passes in CI with real Postgres
- [ ] Frontend `npm run build` passes in CI
- [ ] PRs blocked from merging if CI fails
- [ ] Build artifacts (Docker images) tagged with git SHA on merge to `main`

---

#### INFRA-2: Terraform Staging Environment

**Files to Create:**
- `terraform/staging/main.tf`
- `terraform/staging/variables.tf`
- `terraform/staging/outputs.tf`
- `terraform/staging/terraform.tfvars.example`
- `terraform/modules/rds/` — RDS Postgres module
- `terraform/modules/elasticache/` — Redis module
- `terraform/modules/ecs/` — ECS Fargate service module
- `terraform/modules/s3/` — S3 buckets module

**Resources to provision:**
```hcl
# Core infrastructure
- VPC with public/private subnets (2 AZs)
- RDS Postgres 16 db.t3.micro (single-AZ for staging)
- ElastiCache Redis 7 cache.t3.micro
- S3: ada-staging-resumes, ada-staging-screenshots
- Cognito User Pool + App Client
- ECR repositories: api-gateway, ai-service, job-discovery, job-matcher, apply-engine
- ECS Cluster (Fargate)
- ECS Services: api-gateway (1 task), job-discovery (1 task), apply-engine (1 task)
- Application Load Balancer → api-gateway
- Route53 record: staging-api.autodreamapplier.com → ALB
- ACM certificate
- SSM Parameter Store: all secrets (DATABASE_URL, REDIS_HOST, ENCRYPTION_KEY, etc.)
```

**`.github/workflows/deploy-staging.yml` spec:**
```yaml
# Trigger: push to main
# Steps:
#   1. Build Docker images, tag with SHA, push to ECR
#   2. Update ECS task definitions with new image tags
#   3. Force new deployment on each ECS service
#   4. Wait for services to stabilize (aws ecs wait services-stable)
#   5. Run smoke tests against staging URL
```

**Acceptance Criteria:**
- [ ] `terraform apply` creates all resources from scratch
- [ ] `terraform destroy` cleans up completely (no orphaned resources)
- [ ] `deploy-staging.yml` deploys on merge to `main` within 10 minutes
- [ ] All secrets stored in SSM Parameter Store (no plaintext in Terraform state)
- [ ] Staging URL responds to `GET /health` within 30s of deploy

---

#### INFRA-3: Health Check & Monitoring Setup

**Files to Create:**
- `terraform/staging/monitoring.tf` — CloudWatch dashboards, alarms

**CloudWatch Alarms to configure:**
- `api-gateway` 5xx rate > 5% for 5 min → SNS → email
- `apply-engine` dead-letter queue depth > 10 → SNS → email
- RDS CPU > 80% for 10 min
- Redis memory > 80%
- ECS task count < desired (service disruption)

**Acceptance Criteria:**
- [ ] Grafana + Prometheus running in Docker Compose (already done — wire to staging)
- [ ] CloudWatch dashboard showing key metrics
- [ ] PagerDuty or email alert on 5xx spike

---

### BRANCH: `feat/job-boards`

**Owner:** Backend developer (Go)
**Prerequisite:** None — existing `cmd/job-discovery/` + `internal/job/` are the base
**Reference:** Study `internal/job/scrapers/indeed.go` before implementing new scrapers

---

#### JOB-1: ZipRecruiter Scraper

**File to Create:** `internal/job/scrapers/ziprecruiter.go`

**Interface to implement** (matches `internal/job/scrapers/indeed.go`):
```go
type Scraper interface {
    Name() string
    Scrape(ctx context.Context, query ScraperQuery) (<-chan Job, <-chan error)
}

type ScraperQuery struct {
    Keywords string
    Location string
    Remote   bool
    MaxPages int
}
```

**ZipRecruiter API approach:**
ZipRecruiter has a partner API but also a scrapeable search page.
- Base URL: `https://www.ziprecruiter.com/candidate/search?search={keywords}&location={location}`
- Pagination: `&page=N` (1-based)
- Rate limit: 8 requests/minute, 50/hour — use `pkg/ratelimit/` token bucket
- Deduplication key: `external_id = ziprecruiter_{job_id_from_url}`

**HTML selectors** (as of 2024 — may need updating):
```go
// Job cards
".job_content" // each job card wrapper
"[data-testid='job-title']" // title
"[data-testid='job-company']" // company
"[data-testid='job-location']" // location
"[data-testid='job-url']" // href → job URL
// Salary: "p[aria-label='Estimated Salary']" — optional, may be absent
```

**Files to Create:**
- `internal/job/scrapers/ziprecruiter.go`
- `internal/job/scrapers/ziprecruiter_test.go` — test with fixture HTML in `testdata/`

**Files to Modify:**
- `cmd/job-discovery/main.go` — register `&ZipRecruiterScraper{}` alongside Indeed

**Job struct to populate:**
```go
// internal/job/model.go (existing)
type Job struct {
    ExternalID  string    // "ziprecruiter_" + id from URL
    SourceBoard string    // "ziprecruiter"
    Title       string
    Company     string
    Location    string
    Remote      bool
    SalaryMin   *int
    SalaryMax   *int
    Description string
    URL         string
    PostedAt    time.Time
}
```

**Acceptance Criteria:**
- [ ] `Scrape()` returns ≥1 job for keyword "software engineer" in "New York"
- [ ] Deduplication: second call with same query does not create duplicate DB rows (upsert on `external_id`)
- [ ] Rate limiter prevents >8 requests/minute
- [ ] Graceful shutdown: `ctx.Done()` closes channels without panic
- [ ] Fixture-based unit test passes without network calls

---

#### JOB-2: LinkedIn Easy Apply Scraper (Conservative Mode)

**File to Create:** `internal/job/scrapers/linkedin.go`

**⚠️ Hard constraints (non-negotiable):**
- Max **3 applications/day/user** — enforced at scraper + apply-engine level
- Only "Easy Apply" jobs (LinkedIn's own application flow, not redirect to ATS)
- Use residential proxy per request (configured via `PROXY_URL` env var)
- If 429 response → back off 30 minutes, log alert, do NOT retry same session

**LinkedIn scraper approach:**
LinkedIn blocks headless browsers heavily. Use their job search RSS feed + API where possible.
- Job search URL: `https://www.linkedin.com/jobs/search/?keywords={kw}&location={loc}&f_LF=f_AL` (`f_AL` = Easy Apply filter)
- Parse with Playwright (from browser pool) — NOT direct HTTP (LinkedIn detects)
- Session cookies managed per-user (stored encrypted in `board_credentials`)

**Files to Create:**
- `internal/job/scrapers/linkedin.go`
- `internal/job/scrapers/linkedin_test.go`
- `internal/job/scrapers/linkedin_session.go` — session cookie management

**Files to Modify:**
- `cmd/job-discovery/main.go` — register LinkedIn scraper with rate-limit wrapper
- `migrations/` — add migration: `ALTER TABLE users ADD COLUMN linkedin_daily_count INT DEFAULT 0`, `linkedin_daily_reset TIMESTAMP`

**Rate-limit enforcement location:**
```go
// internal/job/scrapers/linkedin.go
func (s *LinkedInScraper) checkDailyLimit(ctx context.Context, userID uuid.UUID) error {
    // Query: SELECT linkedin_daily_count FROM users WHERE id = $1
    // If count >= 3, return ErrDailyLimitReached
    // Reset count if linkedin_daily_reset < today
}
```

**Acceptance Criteria:**
- [ ] Only jobs with Easy Apply badge are returned
- [ ] `checkDailyLimit` blocks 4th application per user per day
- [ ] Proxy configured via `LINKEDIN_PROXY_URL` env var; error if unset
- [ ] No more than 1 request/10s per user (enforced via per-user token bucket)
- [ ] `external_id = "linkedin_" + job_id`

---

#### JOB-3: Glassdoor Real Implementation

**Current state:** `internal/job/scrapers/glassdoor.go` returns closed empty channels (intentional stub)

**File to Modify:** `internal/job/scrapers/glassdoor.go` — replace stub with real implementation

**Glassdoor approach:**
- Glassdoor requires login for full job listings; use job board API with partner credentials OR scrape public search
- Public URL: `https://www.glassdoor.com/Job/jobs.htm?sc.keyword={kw}&locT=C&locId={loc_id}`
- Rate limit: 5 requests/minute (very conservative; Glassdoor blocks aggressively)
- Must send realistic `User-Agent` + `Referer` headers

**Acceptance Criteria:**
- [ ] `Scrape()` returns real jobs (not empty)
- [ ] `external_id = "glassdoor_" + job_id`
- [ ] Respects 5 req/min rate limit
- [ ] Returns `ErrSourceBlocked` (not panic) if Glassdoor returns 403/429

---

### BRANCH: `feat/ats-plugins`

**Owner:** Backend developer (Go)
**Prerequisite:** None — ATS plugin system already complete
**Reference:** Study `internal/ats/plugins/greenhouse.go` — it's the canonical implementation

---

#### ✅ ATS-1: Lever ATS Plugin

**File Created:** `internal/ats/plugins/lever.go`

**Plugin interface** (defined in `internal/ats/registry.go`):
```go
type Plugin interface {
    Name() string
    Detect(jobURL string) bool
    Fill(ctx context.Context, page playwright.Page, app ApplicationData) error
}

type ApplicationData struct {
    FirstName   string
    LastName    string
    Email       string
    Phone       string
    ResumeURL   string   // pre-signed S3 URL
    CoverLetter string
    LinkedInURL string
    FormAnswers map[string]string  // question text → answer
}
```

**Lever-specific details:**
- URL pattern: `jobs.lever.co/{company}/{job-id}` OR `{company}.lever.co/jobs/{job-id}`
- Application form URL: `jobs.lever.co/{company}/{job-id}/apply`
- Form fields:
  ```
  input[name="name"]          → FirstName + " " + LastName
  input[name="email"]         → Email
  input[name="phone"]         → Phone
  input[type="file"]          → resume upload (download from S3 pre-signed URL to temp file)
  input[name="urls[LinkedIn]"] → LinkedInURL
  textarea.custom-question     → iterate FormAnswers by matching label text
  button[type="submit"]       → click to submit
  ```
- Success indicator: `.success-message` div OR redirect to `/apply/thanks`
- CAPTCHA indicator: `iframe[src*="recaptcha"]` → return `ats.ErrCaptchaRequired`

**Files to Create:**
- `internal/ats/plugins/lever.go`
- `internal/ats/plugins/lever_test.go`

**Files to Modify:**
- `internal/ats/registry.go` — add `Register("lever", &LeverPlugin{})` in init

**Acceptance Criteria:**
- [ ] `Detect("https://jobs.lever.co/stripe/abc-123")` → `true`
- [ ] `Detect("https://greenhouse.io/stripe")` → `false`
- [ ] `Fill()` fills all standard fields and clicks submit
- [ ] `Fill()` returns `ErrCaptchaRequired` when reCAPTCHA iframe detected
- [ ] Temp resume file cleaned up after upload regardless of error
- [ ] Unit test uses mock Playwright page

---

#### ✅ ATS-2: Workday ATS Plugin

**File Created:** `internal/ats/plugins/workday.go`

**Workday-specific details:**
Workday is the most complex ATS — it's a SPA with dynamic form generation.

- URL patterns:
  ```
  {company}.wd1.myworkdayjobs.com/en-US/{job-path}
  {company}.wd5.myworkdayjobs.com/...
  wd3.myworkdayjobs.com/{company}/...
  ```
- Workday uses React; always wait for `networkidle` before interacting
- Multi-step flow (3–5 pages):
  1. "My Information" — name, email, phone, address
  2. "My Experience" — resume upload, work history (skip with resume)
  3. "Application Questions" — custom questions (use FormAnswers)
  4. "Self Identify" — optional EEO fields (skip / select "I don't wish to answer")
  5. "Review" — click "Submit"
- Each page: wait for `[data-automation-id="formContainer"]` before interacting
- Navigation: `[data-automation-id="bottom-navigation-next-btn"]` to advance pages

**Error handling:**
```go
// Workday-specific error patterns
var (
    ErrWorkdaySessionExpired = errors.New("workday: session expired, reload required")
    ErrWorkdayAddressRequired = errors.New("workday: address fields required but not in ApplicationData")
)
```

**Files to Create:**
- `internal/ats/plugins/workday.go`
- `internal/ats/plugins/workday.go` — helper: `workdayFillPage1`, `workdayFillPage2`, etc.
- `internal/ats/plugins/workday_test.go`

**Files to Modify:**
- `internal/ats/registry.go` — register Workday plugin

**Acceptance Criteria:**
- [ ] Detects all three Workday URL patterns (wd1, wd3, wd5 subdomains)
- [ ] Multi-page navigation completes without `--` button click errors
- [ ] EEO "Self Identify" page skipped gracefully (all "decline to identify")
- [ ] Returns `ErrCaptchaRequired` on bot detection
- [ ] Test coverage ≥70% on non-Playwright code paths

---

#### ✅ ATS-3: ATS Auto-Detection

**File Created:** `internal/ats/detector.go`

**Problem:** Currently `ats_type` field must be set manually. Auto-detect from `jobs.url`.

**Detection strategy:**
1. URL pattern matching (fast, no network) — covers 80% of cases
2. DOM meta-tag inspection (for white-labeled ATS) — fallback

```go
// internal/ats/detector.go
package ats

import "regexp"

var urlPatterns = []struct {
    pattern *regexp.Regexp
    atsType string
}{
    {regexp.MustCompile(`greenhouse\.io`), "greenhouse"},
    {regexp.MustCompile(`lever\.co`), "lever"},
    {regexp.MustCompile(`myworkdayjobs\.com`), "workday"},
    {regexp.MustCompile(`icims\.com`), "icims"},
    {regexp.MustCompile(`taleo\.net`), "taleo"},
    {regexp.MustCompile(`smartrecruiters\.com`), "smartrecruiters"},
    {regexp.MustCompile(`bamboohr\.com`), "bamboohr"},
}

// DetectFromURL returns ats type string or "" if unknown
func DetectFromURL(jobURL string) string

// DetectFromDOM fetches page and inspects meta tags / script src (slower)
func DetectFromDOM(ctx context.Context, page playwright.Page, jobURL string) (string, error)
```

**Files to Modify:**
- `internal/application/workers/browser_apply_worker.go` — call `ats.DetectFromURL(app.JobURL)` before looking up plugin; fallback to `ats.DetectFromDOM()` if empty

**Database migration needed:**
```sql
-- No migration needed — jobs.ats_type column already exists
-- Just populate it during job discovery:
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS ats_type VARCHAR(50);
```

**Files to Modify:**
- `internal/job/repository/` — when upserting a job, call `ats.DetectFromURL(job.URL)` and store in `jobs.ats_type`

**Acceptance Criteria:**
- [ ] `DetectFromURL("https://boards.greenhouse.io/stripe/jobs/123")` → `"greenhouse"`
- [ ] `DetectFromURL("https://jobs.lever.co/airbnb/abc")` → `"lever"`
- [ ] `DetectFromURL("https://stripe.wd1.myworkdayjobs.com/...")` → `"workday"`
- [ ] Unknown URL → `""` (not error)
- [ ] Browser apply worker uses detected type if `app.ATSType` is empty
- [ ] Unit tests: 100% coverage on `DetectFromURL` (no network calls)

---

### BRANCH: `feat/auto-apply`

**Owner:** Backend developer (Go)
**Prerequisite:** None (uses existing match + application services)
**Integrates with:** `feat/job-boards` (more matches flowing in), `feat/notifications` (triggers events)

---

#### AUTO-1: Configurable Auto-Threshold per User

**Current state:** `user_preferences.auto_approve_threshold` column exists in schema but not exposed via API.

**Files to Modify:**
- `internal/user/handlers/user_handler.go` — `GET/PUT /users/me/preferences` already handles preferences; ensure `auto_approve_threshold` (float, 0.0–1.0) and `auto_apply_enabled` (bool) are included in request/response
- `internal/user/repository/preferences_repository.go` — include new fields in SELECT/UPDATE queries
- `frontend/src/lib/types.ts` — add `autoApproveThreshold: number` and `autoApplyEnabled: boolean` to `UserPreferences` type
- `frontend/src/components/settings/preferences-form.tsx` — add slider for threshold + toggle for auto-apply

**API contract** (add to existing `PUT /users/me/preferences`):
```json
{
  "autoApplyEnabled": true,
  "autoApproveThreshold": 0.75,
  "dailyApplicationLimit": 10,
  "applyStartHour": 9,
  "applyEndHour": 17,
  "applyTimezone": "America/New_York"
}
```

**Database migration:**
```sql
-- migrations/XXXX_auto_apply_schedule.sql
ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS auto_apply_enabled BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS daily_application_limit INT DEFAULT 10,
    ADD COLUMN IF NOT EXISTS apply_start_hour INT DEFAULT 9,
    ADD COLUMN IF NOT EXISTS apply_end_hour INT DEFAULT 17,
    ADD COLUMN IF NOT EXISTS apply_timezone VARCHAR(50) DEFAULT 'America/New_York';
```

**Acceptance Criteria:**
- [ ] `PUT /users/me/preferences` accepts and persists all new fields
- [ ] `GET /users/me/preferences` returns all new fields
- [ ] `autoApproveThreshold` rejected if < 0.0 or > 1.0 (HTTP 400)
- [ ] `dailyApplicationLimit` rejected if < 1 or > 50 (HTTP 400)

---

#### AUTO-2: Batch Auto-Approval Job

**File to Create:** `internal/jobmatcher/service/auto_approve_service.go`

**Logic:**
```go
// Run after every batch of new matches is created
// For each user with auto_apply_enabled = true:
//   SELECT matches WHERE user_id = $1
//     AND status = 'pending'
//     AND match_score >= user.auto_approve_threshold
//     AND created_at > NOW() - INTERVAL '24 hours'
//   For each match: call application.Service.Submit(matchID)
//   Track daily_application_limit — stop when reached

func (s *AutoApproveService) ProcessPendingMatches(ctx context.Context, userID uuid.UUID) error
func (s *AutoApproveService) RunForAllUsers(ctx context.Context) error  // called by cron
```

**File to Create:** `cmd/job-matcher/cron.go` (or add to existing job-matcher cmd)
```go
// Cron: every 30 minutes
// Call AutoApproveService.RunForAllUsers(ctx)
```

**Acceptance Criteria:**
- [ ] Auto-approve only runs when `auto_apply_enabled = true`
- [ ] Respects `daily_application_limit` — counts applications submitted today, stops at limit
- [ ] `match_score < auto_approve_threshold` → stays `pending`
- [ ] `match_score >= threshold` → status transitions to `approved` → enqueues `TypeAIPrep` task
- [ ] Unit tests: threshold boundary (0.74 → skip, 0.75 → approve at threshold 0.75)

---

#### AUTO-3: Business Hours Scheduling

**File to Create:** `internal/application/service/scheduler.go`

**Problem:** Applications should only submit during user's configured business hours to appear more human.

```go
type Scheduler struct {
    prefs PreferencesRepository
}

// IsWithinApplyWindow returns true if current time (in user's timezone) is
// between apply_start_hour and apply_end_hour on a weekday
func (s *Scheduler) IsWithinApplyWindow(ctx context.Context, userID uuid.UUID) (bool, error)

// NextApplyTime returns the next timestamp when applications can be submitted
func (s *Scheduler) NextApplyTime(ctx context.Context, userID uuid.UUID) (time.Time, error)
```

**Integration point:**
In `internal/application/workers/browser_apply_worker.go`:
```go
// At start of ProcessTask:
withinWindow, err := scheduler.IsWithinApplyWindow(ctx, app.UserID)
if !withinWindow {
    nextTime, _ := scheduler.NextApplyTime(ctx, app.UserID)
    // Re-enqueue with ProcessAt: nextTime (Asynq supports this)
    return client.Enqueue(task, asynq.ProcessAt(nextTime))
}
```

**Acceptance Criteria:**
- [ ] Task re-enqueued (not dropped) if outside business hours
- [ ] `time.LoadLocation(userTimezone)` error → fallback to UTC, log warning
- [ ] Weekend: tasks deferred to Monday 09:00 user-local time
- [ ] Unit tests use fixed `time.Now` injection (no real clock dependency)

---

#### AUTO-4: Daily Application Count Enforcement

**File to Create:** `internal/application/service/rate_limiter.go`

**Logic:**
```go
// Uses Redis for atomic counters (fast, no DB write on every check)
// Key: "daily_apply:{userID}:{YYYY-MM-DD}"
// TTL: 25 hours (safe expiry)

func (r *RateLimiter) CheckAndIncrement(ctx context.Context, userID uuid.UUID, limit int) error
// Returns ErrDailyLimitReached if count >= limit
// Otherwise increments counter and returns nil
```

**Files to Modify:**
- `internal/application/service/application_service.go` — call `RateLimiter.CheckAndIncrement` in `Submit()` before enqueuing

**Acceptance Criteria:**
- [ ] 11th application attempt on a day with limit=10 → `ErrDailyLimitReached`
- [ ] Counter resets at midnight user-local time (key includes date in user TZ)
- [ ] Redis unavailable → fail open (log warning, allow application) — never hard-block user
- [ ] Integration test using real Redis (via Docker Compose test env)

---

### BRANCH: `feat/ai-service`

**Owner:** Python / AI developer
**Prerequisite:** None — standalone Python FastAPI service in `ai-service/`
**Critical:** `internal/application/workers/ai_prep_worker.go` already calls this service at `AI_SERVICE_URL`; must match the expected API contract

---

#### AI-1: Project Scaffolding

**Files to Create:**
```
ai-service/
├── main.py
├── requirements.txt
├── Dockerfile
├── app/
│   ├── __init__.py
│   ├── config.py          # pydantic-settings, reads env vars
│   ├── models.py          # Pydantic request/response models
│   ├── routes/
│   │   ├── resume.py      # /api/v1/resume/tailor
│   │   ├── cover_letter.py # /api/v1/cover-letter/generate
│   │   ├── form_qa.py     # /api/v1/form-qa/answer
│   │   └── health.py      # /health
│   └── services/
│       ├── anthropic_client.py  # wrapper around Anthropic Python SDK
│       ├── s3_client.py   # boto3 S3 download/upload
│       └── prompt_builder.py  # constructs prompts from resume + job data
└── tests/
    └── test_routes.py
```

**Environment variables:**
```
ANTHROPIC_API_KEY=sk-ant-...
AWS_REGION=us-east-1
S3_BUCKET_RESUMES=ada-resumes
S3_BUCKET_SCREENSHOTS=ada-screenshots  # not used by AI service but keep consistent
APP_ENV=development
```

---

#### AI-2: Resume Tailoring Endpoint

**Endpoint:** `POST /api/v1/resume/tailor`

**Request contract** (must match what `ai_prep_worker.go` sends):
```json
{
  "resume_s3_key": "users/abc123/resumes/my-resume.pdf",
  "job_title": "Senior Software Engineer",
  "job_description": "We are looking for...",
  "job_company": "Stripe",
  "tailoring_instructions": "Emphasize distributed systems experience"
}
```

**Response contract:**
```json
{
  "tailored_resume_s3_key": "users/abc123/tailored/job_456_resume.pdf",
  "changes_summary": "Highlighted 3 distributed systems projects, reordered skills section",
  "tokens_used": 1247
}
```

**Implementation:**
```python
# app/services/prompt_builder.py
RESUME_TAILOR_PROMPT = """
You are an expert resume writer. Given the original resume and job description below,
rewrite the resume to emphasize relevant experience while keeping all facts accurate.
Do NOT invent experience. Only reorder, rephrase, and highlight existing content.

ORIGINAL RESUME:
{resume_text}

JOB TITLE: {job_title}
COMPANY: {job_company}
JOB DESCRIPTION:
{job_description}

Return the tailored resume in clean plain text format, ready for PDF conversion.
"""
```

**PDF generation:** Use `reportlab` or `weasyprint` to convert tailored text → PDF → upload to S3.

**Acceptance Criteria:**
- [ ] Returns tailored resume as new S3 object (does not overwrite original)
- [ ] Response includes `tailored_resume_s3_key` matching pattern `users/{id}/tailored/job_{jobID}_resume.pdf`
- [ ] PDF is parseable (not corrupt)
- [ ] Uses Claude Haiku (`claude-haiku-20240307`) for cost efficiency (~$0.001/call)
- [ ] `tokens_used` reported for billing tracking
- [ ] Returns HTTP 422 if `resume_s3_key` not found in S3

---

#### AI-3: Cover Letter Generation Endpoint

**Endpoint:** `POST /api/v1/cover-letter/generate`

**Request contract:**
```json
{
  "resume_s3_key": "users/abc123/resumes/my-resume.pdf",
  "job_title": "Senior Software Engineer",
  "job_description": "We are looking for...",
  "job_company": "Stripe",
  "user_name": "Jane Smith",
  "tone": "professional"
}
```

**Response contract:**
```json
{
  "cover_letter_s3_key": "users/abc123/cover-letters/job_456_cover_letter.txt",
  "cover_letter_text": "Dear Hiring Team at Stripe,\n\nI am excited...",
  "tokens_used": 892
}
```

**Prompt template:**
```python
COVER_LETTER_PROMPT = """
Write a concise, compelling cover letter (3 paragraphs, max 300 words) for {user_name}
applying to {job_title} at {job_company}. Tone: {tone}.

Use the resume for facts. Opening paragraph: why this role + company.
Middle paragraph: 2-3 most relevant achievements from resume.
Closing paragraph: call to action.

RESUME:
{resume_text}

JOB DESCRIPTION:
{job_description}
"""
```

**Acceptance Criteria:**
- [ ] Cover letter stored as `.txt` in S3 (for easy retrieval in frontend preview)
- [ ] Cover letter ≤ 400 words (enforced post-generation, retry once if exceeded)
- [ ] `tone` must be one of: `professional`, `enthusiastic`, `concise` — HTTP 422 otherwise
- [ ] Model: Claude Haiku

---

#### AI-4: Form Q&A Endpoint

**Endpoint:** `POST /api/v1/form-qa/answer`

This endpoint answers common ATS screening questions using resume context.

**Request contract:**
```json
{
  "resume_s3_key": "users/abc123/resumes/my-resume.pdf",
  "questions": [
    "Why do you want to work at Stripe?",
    "Describe your experience with distributed systems",
    "What is your expected salary range?",
    "Are you authorized to work in the US?"
  ],
  "job_company": "Stripe",
  "job_title": "Senior Engineer",
  "user_salary_expectation": "$180,000 - $220,000",
  "user_work_authorization": "US Citizen"
}
```

**Response contract:**
```json
{
  "answers": {
    "Why do you want to work at Stripe?": "I'm drawn to Stripe's mission of...",
    "Describe your experience with distributed systems": "In my 5 years at...",
    "What is your expected salary range?": "$180,000 - $220,000",
    "Are you authorized to work in the US?": "Yes, I am a US Citizen"
  },
  "tokens_used": 654
}
```

**Special handling:**
- Salary questions → always return `user_salary_expectation` verbatim (no AI generation)
- Authorization questions → return `user_work_authorization` verbatim
- `"Do you have X years of experience"` → check resume, answer Yes/No accurately

**Acceptance Criteria:**
- [ ] All questions answered in single API call (batch prompt)
- [ ] Salary answer never hallucinated — uses `user_salary_expectation` directly
- [ ] Answers ≤ 150 words each
- [ ] Unknown questions answered conservatively ("I'd be happy to discuss this further in the interview")

---

#### AI-5: Wire AI Client in Go Worker

**Current state:** `internal/application/workers/ai_prep_worker.go` calls `AI_SERVICE_URL` via HTTP but the AI service stub returns empty responses.

**File to Modify:** `internal/application/workers/ai_prep_worker.go`

**Contract between worker and AI service:**
```go
// internal/ai/client.go (create this)
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
}

func (c *Client) TailorResume(ctx context.Context, req TailorResumeRequest) (*TailorResumeResponse, error)
func (c *Client) GenerateCoverLetter(ctx context.Context, req CoverLetterRequest) (*CoverLetterResponse, error)
func (c *Client) AnswerFormQuestions(ctx context.Context, req FormQARequest) (*FormQAResponse, error)
```

**Changes to `ai_prep_worker.go`:**
1. Call `aiClient.TailorResume()` → get `tailored_resume_s3_key`
2. Call `aiClient.GenerateCoverLetter()` → get `cover_letter_s3_key`
3. Store both keys in `applications` table (`tailored_resume_s3_key`, `cover_letter_s3_key` columns)
4. Advance status to `ai_ready`

**Database migration needed:**
```sql
-- migrations/XXXX_application_ai_keys.sql
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS tailored_resume_s3_key VARCHAR(500),
    ADD COLUMN IF NOT EXISTS cover_letter_s3_key VARCHAR(500);
```

**Acceptance Criteria:**
- [ ] Worker calls real AI service (not stub) when `AI_SERVICE_URL` is set
- [ ] Both S3 keys stored in DB before status advances to `ai_ready`
- [ ] If AI service returns 5xx → retry 3× with exponential backoff, then status → `ai_failed`
- [ ] `ai_failed` status creates `application_events` record with error details

---

### BRANCH: `feat/notifications`

**Owner:** Backend developer (Go) — can be same dev as `feat/auto-apply` or separate
**Prerequisite:** `feat/auto-apply` events (shares notification triggers)
**Files to study:** `internal/notification/ses.go` — existing SES client

---

#### NOTIF-1: Weekly Email Digest

**File to Create:** `internal/notification/digest_service.go`

**Logic:**
```go
type DigestService struct {
    ses    *SESClient
    db     *sql.DB
}

// SendWeeklyDigest sends one digest email per active user
// Called by: scheduled cron job (every Monday 08:00 UTC)
func (d *DigestService) SendWeeklyDigest(ctx context.Context) error

// BuildDigestData gathers stats for a single user
func (d *DigestService) BuildDigestData(ctx context.Context, userID uuid.UUID) (*DigestData, error)

type DigestData struct {
    UserName          string
    DashboardURL      string
    WeekApplications  int
    WeekMatches       int
    PendingReview     int
    ApplicationStats  []ApplicationStat  // {company, title, status}
    TopMatchThisWeek  *Match             // highest-scoring new match
}
```

**SES email template:**
- Subject: `Your AutoDreamApplier Weekly Update — {N} applications, {M} new matches`
- HTML email: use Go `html/template` with inline CSS (SES renders HTML email)
- Template file: `internal/notification/templates/weekly_digest.html`
- Unsubscribe link: `{DASHBOARD_URL}/settings?unsubscribe=digest`

**File to Create:**
- `cmd/digest-worker/main.go` — standalone binary; cron triggers via ECS scheduled task
- `internal/notification/digest_service.go`
- `internal/notification/templates/weekly_digest.html`

**Acceptance Criteria:**
- [ ] Only sent to users with ≥1 activity in past 7 days (skip inactive users)
- [ ] Unsubscribe link respected — `user_preferences.email_digest_enabled` column
- [ ] SES `ConfigurationSetName` set for bounce/complaint tracking
- [ ] Test mode: `APP_ENV=test` writes email to stdout instead of sending

---

#### NOTIF-2: Slack & Discord Webhook Notifications

**File to Create:** `internal/notification/webhook_service.go`

**Supported events:**
```go
type WebhookEvent string

const (
    EventNewMatch          WebhookEvent = "new_match"
    EventApplicationSubmitted WebhookEvent = "application_submitted"
    EventApplicationFailed    WebhookEvent = "application_failed"
    EventDailyLimitReached    WebhookEvent = "daily_limit_reached"
)
```

**Webhook payload (Slack-compatible Block Kit):**
```json
{
  "text": "✅ Applied to Senior Engineer at Stripe",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "✅ *Application Submitted*\n*Role:* Senior Engineer\n*Company:* Stripe\n*Status:* Applied"
      }
    },
    {
      "type": "actions",
      "elements": [{ "type": "button", "text": { "type": "plain_text", "text": "View Application" }, "url": "https://..." }]
    }
  ]
}
```

**Discord:** Same payload works (Discord supports Slack-compatible webhooks).

**Database migration:**
```sql
-- migrations/XXXX_webhook_settings.sql
ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS slack_webhook_url VARCHAR(500),
    ADD COLUMN IF NOT EXISTS discord_webhook_url VARCHAR(500),
    ADD COLUMN IF NOT EXISTS webhook_events TEXT[] DEFAULT '{}';
    -- webhook_events: which events to send (subset of EventXxx constants)
```

**Files to Modify:**
- `internal/user/handlers/user_handler.go` — expose webhook settings in preferences API
- `internal/application/service/application_service.go` — call `webhookService.Send()` on status changes

**Acceptance Criteria:**
- [ ] Webhook fires within 5 seconds of application status change
- [ ] Failed webhook (non-2xx) retried 3× with 5s backoff — never blocks application flow
- [ ] Webhook URL validated on save (`http.Get` with timeout 3s — reject if unreachable)
- [ ] User can configure which events trigger webhooks

---

#### NOTIF-3: Application Follow-Up Automation

**File to Create:** `internal/notification/followup_service.go`

**Logic:**
- After `application.status = 'applied'`, schedule a follow-up check
- Default: 7 days after `applied_at` — check if user received response
- If no response recorded → prompt user in dashboard ("Did you hear back from Stripe?")
- If user configured follow-up emails: draft email template + show preview for user approval

```go
type FollowUpService struct {}

// ScheduleFollowUp creates a follow-up reminder in application_events
// Called by: browser_apply_worker after successful submission
func (s *FollowUpService) ScheduleFollowUp(ctx context.Context, applicationID uuid.UUID) error

// GetPendingFollowUps returns applications needing user attention
// Called by: dashboard API (adds badge to sidebar)
func (s *FollowUpService) GetPendingFollowUps(ctx context.Context, userID uuid.UUID) ([]FollowUp, error)
```

**Acceptance Criteria:**
- [ ] Follow-up scheduled automatically after successful `applied` status
- [ ] `GET /applications/follow-ups` returns list of pending follow-ups
- [ ] User can dismiss follow-up (no recurring reminders after dismiss)

---

### BRANCH: `feat/frontend-v2`

**Owner:** Frontend developer (Next.js / React / TypeScript)
**Prerequisite:** Can start immediately; some features require backend branches to be merged first (noted inline)
**Files to study:** All existing components in `frontend/src/components/`

---

#### FE-1: Bulk Approve/Reject in Match Queue

**File to Modify:** `frontend/src/components/matches/match-queue.tsx`

**Changes:**
- Add "Select All" checkbox in header
- Add per-row checkbox (show on hover)
- Bulk action toolbar: appears when ≥1 selected — "Approve Selected", "Reject Selected" buttons
- Bulk API calls: `POST /applications/bulk-approve` with `{ matchIds: string[] }`

**Backend endpoint needed** (add to `internal/application/handler/`):
```go
// POST /applications/bulk-approve
// Body: { "match_ids": ["uuid1", "uuid2"] }
// Returns: { "approved": 5, "failed": 0 }

// POST /applications/bulk-reject
// Body: { "match_ids": ["uuid1", "uuid2"] }
```

**Frontend acceptance criteria:**
- [ ] "Select All" selects all matches on current page (not all pages)
- [ ] Bulk action shows count: "2 selected — Approve | Reject | Cancel"
- [ ] Optimistic UI: cards immediately move to correct tab; revert on API error
- [ ] Loading spinner during bulk operation
- [ ] `api.ts` exports `bulkApproveMatches(matchIds: string[])` and `bulkRejectMatches(matchIds: string[])`

---

#### ✅ FE-2: Application Detail Page

**File Created:** `frontend/src/app/dashboard/applications/[id]/page.tsx`

**Content:**
```
┌─────────────────────────────────────────────────────┐
│  Senior Engineer @ Stripe                           │
│  Applied: March 10, 2026 · via Greenhouse           │
├─────────────────────────────────────────────────────┤
│  STATUS TIMELINE                                    │
│  ○ pending → ○ ai_ready → ● applied                 │
│  [timestamp] [timestamp] [timestamp]                │
├───────────────────────┬─────────────────────────────┤
│  COVER LETTER         │  SCREENSHOT PROOF           │
│  (text preview)       │  (screenshot image from S3) │
├───────────────────────┴─────────────────────────────┤
│  APPLICATION EVENTS LOG                             │
│  Mar 10 14:32 — AI prep completed (1.2s)            │
│  Mar 10 14:33 — Browser acquired                   │
│  Mar 10 14:35 — Form submitted                     │
└─────────────────────────────────────────────────────┘
```

**API calls needed:**
- `GET /applications/{id}` — application detail (already exists)
- `GET /applications/{id}/events` — event log (add to backend)
- Pre-signed S3 URL for screenshot — include in `GET /applications/{id}` response

**Acceptance Criteria:**
- [ ] Timeline shows all status transitions with timestamps
- [ ] Screenshot displayed (or "No screenshot available" placeholder)
- [ ] Cover letter text rendered with whitespace preserved
- [ ] Event log shows human-readable messages
- [ ] Back button returns to applications list with scroll position preserved

---

#### FE-3: Application Search & Filter

**File to Modify:** `frontend/src/app/dashboard/applications/page.tsx`

**Additions:**
- Search input: filter by company name or job title (client-side for current page, server-side for all)
- Filter bar: Status chips (All / Pending / Applied / Failed), Date range picker
- Sort: "Newest first" / "Company A-Z"

**Backend change needed:**
- `GET /applications` — add query params: `?status=applied&search=stripe&sort=created_desc&page=1&limit=20`

**Acceptance Criteria:**
- [ ] Search debounced (300ms) — no API call on every keystroke
- [ ] Filter/sort state preserved in URL params (`?status=applied&search=stripe`)
- [ ] Browser back/forward navigates filter state correctly
- [ ] Empty state: "No applications match your filters" with clear filters button

---

#### FE-4: Auto-Apply Toggle + Dashboard Badge

**Requires:** `feat/auto-apply` AUTO-1 merged first

**Files to Modify:**
- `frontend/src/components/settings/preferences-form.tsx` — add auto-apply section:
  ```
  ┌──────────────────────────────────────┐
  │  Auto-Apply Settings                 │
  │  ○──────────────────● Enable         │
  │  Match threshold: [====●====] 75%    │
  │  Daily limit: [10 applications/day]  │
  │  Apply hours: 9:00 AM – 5:00 PM      │
  │  Timezone: [America/New_York ▼]      │
  └──────────────────────────────────────┘
  ```
- `frontend/src/app/dashboard/layout.tsx` — show "Auto" badge on sidebar when auto-apply enabled

**Confirmation modal:**
When enabling auto-apply: "Auto-apply will submit applications on your behalf without individual review. Are you sure?"

**Acceptance Criteria:**
- [ ] Toggle shows confirmation modal before enabling (not before disabling)
- [ ] Threshold slider shows percentage label
- [ ] Timezone selector populated from `Intl.supportedValuesOf('timeZone')`
- [ ] Settings persist across page refresh

---

#### FE-5: Match Feedback (Thumbs Up/Down)

**Files to Modify:** `frontend/src/components/matches/match-queue.tsx`

**Changes:**
- Add thumbs up / thumbs down icons on each match card (visible on hover)
- `POST /matches/{id}/feedback` with `{ "feedback": "positive" | "negative" }`
- Feedback stored in `matches.user_feedback` (already in schema)

**Purpose:** Training data for future ML-based matching improvements.

**Acceptance Criteria:**
- [ ] Feedback icons show on hover (not always visible — avoid UI clutter)
- [ ] Feedback persists (button highlighted after submission)
- [ ] Can change feedback (positive → negative)
- [ ] `api.ts` exports `submitMatchFeedback(matchId: string, feedback: 'positive' | 'negative')`

---

#### FE-6: Notification Settings

**Requires:** `feat/notifications` NOTIF-2 merged first

**File to Modify:** `frontend/src/app/dashboard/settings/page.tsx` — add Notifications section

**Content:**
```
Notifications
├── Email Digest    [Weekly ▼]  [●]
├── Slack Webhook   [https://hooks.slack.com/...      ] [Test]
├── Discord Webhook [https://discord.com/api/webhooks/] [Test]
└── Notify on:      [✓] New match  [✓] Application submitted  [ ] Daily limit
```

**Acceptance Criteria:**
- [ ] "Test" button sends a test notification to the webhook URL
- [ ] Webhook URL validated before save (frontend shows "Sending test..." spinner)
- [ ] Saves via `PUT /users/me/preferences`

---

## Phase 2: AI & Scale (Months 5–8)

> Start after MVP-B is stable in production.

### BRANCH: `feat/pgvector-matching` *(Phase 2)*

**Effort:** 2 weeks | **Owner:** Backend developer (Go) + Data engineer

#### VEC-1: Enable pgvector Extension

```sql
-- migrations/XXXX_pgvector.sql
CREATE EXTENSION IF NOT EXISTS vector;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS embedding vector(384);
ALTER TABLE resumes ADD COLUMN IF NOT EXISTS embedding vector(384);
CREATE INDEX idx_jobs_embedding ON jobs USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

#### VEC-2: Embedding Pipeline

**File to Create:** `ai-service/app/routes/embed.py`

```python
# POST /api/v1/embed/job
# Body: { "job_id": "uuid", "title": "...", "description": "..." }
# Response: { "embedding": [0.1, 0.2, ...] }  // 384-dim float array

# Uses: sentence-transformers/all-MiniLM-L6-v2 (384-dim, CPU-friendly)
# Store embedding back to jobs table via Go worker
```

**Go worker addition** (`cmd/embedding-worker/`):
- Triggered after job discovery batch completes
- Calls `/api/v1/embed/job` for each new job
- Stores `embedding` in `jobs` table

#### VEC-3: Replace Keyword Scorer with Cosine Similarity

**File to Modify:** `internal/jobmatcher/service/matching_service.go`

```go
// Current: keyword intersection score
// New: cosine similarity between user resume embedding and job embedding
// Query:
// SELECT id, title, company, 1 - (embedding <=> $1) AS similarity_score
// FROM jobs
// WHERE (1 - (embedding <=> $1)) > 0.65  -- configurable threshold
// ORDER BY similarity_score DESC
// LIMIT 50
```

**Acceptance Criteria:**
- [ ] Match quality measurably improves (A/B test: keyword vs vector on same user)
- [ ] Query time ≤ 50ms with IVFFlat index for 1M jobs
- [ ] Fallback to keyword scoring if `embedding IS NULL`

---

### BRANCH: `feat/more-ats-plugins` *(Phase 2)*

**Effort:** 3 weeks | **Owner:** Backend developer (Go)

#### ✅ ATS-4: iCIMS Plugin
- File: `internal/ats/plugins/icims.go` — registered in apply-engine
- URL pattern: `careers.icims.com/jobs/{id}` or `{company}.icims.com`

#### ✅ ATS-5: Taleo Plugin
- File: `internal/ats/plugins/taleo.go` — registered in apply-engine
- URL pattern: `{company}.taleo.net`

#### ✅ ATS-6: SmartRecruiters Plugin
- File: `internal/ats/plugins/smartrecruiters.go` — registered in apply-engine
- URL pattern: `jobs.smartrecruiters.com/{company}/{job-id}`

#### ✅ ATS-8: BambooHR Plugin
- File: `internal/ats/plugins/bamboohr.go` — registered in apply-engine
- URL pattern: `{company}.bamboohr.com/jobs` or `/careers`

#### ✅ ATS-9: SAP SuccessFactors Plugin
- File: `internal/ats/plugins/successfactors.go` — registered in apply-engine
- URL pattern: `successfactors.com` or `successfactors.eu`

#### ✅ ATS-7: Generic Form-Fill Fallback
**File Created:** `internal/ats/plugins/generic.go`

For unknown ATS types:
1. Find all visible `<input>` and `<textarea>` elements
2. Match label text against known field types (name, email, phone, resume)
3. Fill known fields; use `FormQA` for unknown text questions
4. Click primary submit button

```go
type GenericPlugin struct {
    aiClient *ai.Client  // for form Q&A on unknown fields
}

func (g *GenericPlugin) Detect(url string) bool { return true }  // always matches (fallback)
func (g *GenericPlugin) Name() string { return "generic" }
```

**Important:** Register generic plugin LAST in registry so specific plugins take priority.

---

### BRANCH: `feat/resume-ab-testing` *(Phase 2)*

**Effort:** 1.5 weeks | **Owner:** Full-stack developer

#### AB-1: Resume Versions Table

```sql
-- migrations/XXXX_resume_versions.sql
CREATE TABLE resume_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID REFERENCES applications(id),
    resume_id UUID REFERENCES resumes(id),
    tailored_s3_key VARCHAR(500),
    variant_label VARCHAR(50),  -- "A", "B", "control"
    created_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE applications ADD COLUMN resume_version_id UUID REFERENCES resume_versions(id);
```

#### AB-2: A/B Assignment Logic
- Every 2nd application uses alternative tailoring prompt variant
- Track `variant_label` on application
- Dashboard shows conversion rate per variant

---

### BRANCH: `feat/scam-detection` *(Phase 2)*

**Effort:** 1 week | **Owner:** Backend developer (Go)

#### SCAM-1: Scam Score Computation

**File to Create:** `internal/job/scam/detector.go`

```go
// Rule-based scorer (no ML needed for MVP):
// Patterns that indicate scam:
var scamPatterns = []struct {
    pattern *regexp.Regexp
    weight  float64
}{
    {regexp.MustCompile(`(?i)gift card`), 0.9},
    {regexp.MustCompile(`(?i)no experience required`), 0.3},
    {regexp.MustCompile(`(?i)earn \$[0-9,]+ per (day|week) from home`), 0.8},
    {regexp.MustCompile(`(?i)work from home.*\$[0-9]{3,}.*guaranteed`), 0.9},
    {regexp.MustCompile(`(?i)western union|money transfer`), 0.95},
}

func ScoreJob(job Job) float64  // returns 0.0–1.0
```

**Database:**
```sql
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS scam_score FLOAT DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS is_scam BOOLEAN GENERATED ALWAYS AS (scam_score > 0.7) STORED;
```

**Integration:** Call `scam.ScoreJob(job)` in job discovery before upsert.

**Acceptance Criteria:**
- [ ] Jobs with `is_scam = true` excluded from match queue by default
- [ ] User can toggle "Show possible scam jobs" in preferences
- [ ] Scam score visible on match card (warning icon if score > 0.5)

---

## Phase 3: Analytics & Growth (Months 9–14)

> Start after Phase 2 is production-stable. Specs are written at MVP-B detail level.

### Phase 3 Branch Map

| Branch | Focus | Effort |
|--------|-------|--------|
| `feat/analytics` | Full funnel tracking + dashboard charts | 2 weeks |
| `feat/followup-emails` | Automated follow-up email sequences | 1 week |
| `feat/i18n` | Multi-language resume + cover letter | 3 weeks |
| `feat/interview-scheduler` | Calendar-based interview auto-reply | 2 weeks |
| `feat/salary-data` | Salary aggregation + benchmarks on match cards | 1.5 weeks |
| `feat/multi-tenant` | Team / agency accounts + admin role | 4 weeks |
| `feat/public-api` | REST API keys + outbound webhook delivery | 2 weeks |
| `feat/referrals` | Referral codes + credit system | 1 week |
| `feat/retention` | Churn prediction + re-engagement email | 1.5 weeks |

---

### BRANCH: `feat/analytics`

**Owner:** Full-stack developer
**Effort:** 2 weeks
**Prerequisite:** MVP-B complete (need real application volume)

---

#### ANAL-1: Outcome Tracking Schema

**Files to Create:** `migrations/XXXX_outcomes.sql`

```sql
CREATE TYPE application_outcome AS ENUM (
    'applied', 'viewed', 'phone_screen', 'interview',
    'offer', 'accepted', 'rejected', 'ghosted'
);

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS outcome application_outcome DEFAULT 'applied',
    ADD COLUMN IF NOT EXISTS outcome_updated_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS outcome_notes TEXT;

-- Aggregation table (pre-computed nightly, for fast dashboard queries)
CREATE TABLE application_daily_stats (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    applied     INT DEFAULT 0,
    phone_screens INT DEFAULT 0,
    interviews  INT DEFAULT 0,
    offers      INT DEFAULT 0,
    UNIQUE (user_id, date)
);
CREATE INDEX idx_daily_stats_user_date ON application_daily_stats(user_id, date DESC);
```

---

#### ANAL-2: Outcome Update API

**File to Modify:** `internal/application/handler/application_handler.go`

```go
// PATCH /applications/{id}/outcome
// Body: { "outcome": "interview", "notes": "Scheduled for March 20" }
// Auth: must be application owner

// GET /applications/stats?from=2026-01-01&to=2026-03-31
// Returns: { "applied": 45, "interviews": 8, "offers": 2, "interview_rate": "17.8%" }
```

**Acceptance Criteria:**
- [ ] Only the application owner can update outcome
- [ ] `outcome_updated_at` timestamped on every change
- [ ] Stats endpoint aggregates from `application_daily_stats` (fast) with fallback to raw query
- [ ] Stats respect `from` / `to` date range query params

---

#### ANAL-3: Dashboard Analytics Charts

**Files to Create:**
- `frontend/src/app/dashboard/analytics/page.tsx`
- `frontend/src/components/analytics/funnel-chart.tsx`
- `frontend/src/components/analytics/applications-over-time.tsx`
- `frontend/src/components/analytics/outcome-entry-modal.tsx`

**Charts to implement:**

1. **Applications Over Time** — line chart, daily count, last 30 / 90 / 180 days toggle
2. **Conversion Funnel** — horizontal bar: Applied → Phone Screen → Interview → Offer
   ```
   Applied       ████████████████████████ 45
   Phone Screen  ███████░░░░░░░░░░░░░░░░░  8  (17.8%)
   Interview     █████░░░░░░░░░░░░░░░░░░░  4  (8.9%)
   Offer         ██░░░░░░░░░░░░░░░░░░░░░░  1  (2.2%)
   ```
3. **Interview Rate by Resume Version** — bar chart (unlocks after `feat/resume-ab-testing`)
4. **Top Companies Applied** — horizontal bar, top 10

**Outcome entry:** `<OutcomeEntryModal>` — shown 7 days after application, prompts "Did you hear back from [Company]?" with outcome dropdown.

**Chart library:** Use `recharts` (already common in Next.js projects; no heavy bundle).

**Acceptance Criteria:**
- [ ] All charts use real API data (no mock)
- [ ] Empty state: "Apply to some jobs first to see analytics" with CTA to match queue
- [ ] Responsive: charts readable on mobile (375px)
- [ ] Date range filter persists in URL params

---

### BRANCH: `feat/followup-emails`

**Owner:** Backend developer (Go)
**Effort:** 1 week
**Prerequisite:** `feat/notifications` NOTIF-3 (follow-up scheduler already wired)

---

#### FOLLOW-1: Follow-Up Email Templates

**Files to Create:**
- `internal/notification/templates/followup_1week.html`
- `internal/notification/templates/followup_2week.html`
- `internal/notification/followup_email_service.go`

**1-week follow-up template:**
```
Subject: Following up on my application — [Job Title] at [Company]

Dear Hiring Team,

I wanted to follow up on my application for the [Job Title] role at [Company],
submitted on [Applied Date]. I remain very excited about this opportunity...

[Auto-generated from user's cover letter context]

Best regards,
[User Name]
```

**Logic:**
```go
type FollowUpEmailService struct {
    ses      *SESClient
    aiClient *ai.Client  // generates personalized follow-up text
    db       *sql.DB
}

// SendPendingFollowUps called by daily cron
// Finds applications where:
//   status = 'applied'
//   AND applied_at < NOW() - INTERVAL '7 days'
//   AND outcome = 'applied'  (no response recorded)
//   AND followup_sent_at IS NULL
//   AND user_preferences.followup_emails_enabled = true
func (f *FollowUpEmailService) SendPendingFollowUps(ctx context.Context) error
```

**Database migration:**
```sql
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS followup_sent_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS followup_2week_sent_at TIMESTAMP;

ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS followup_emails_enabled BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS followup_delay_days INT DEFAULT 7;
```

**Acceptance Criteria:**
- [ ] Follow-up email only sent once per application (idempotent — `followup_sent_at` guard)
- [ ] 2-week follow-up only sent if no response to 1-week follow-up
- [ ] User can disable follow-up emails in preferences
- [ ] SES `ReplyTo` set to user's email so replies go to user, not our system
- [ ] `APP_ENV=test` → writes email content to stdout, no SES call

---

### BRANCH: `feat/i18n`

**Owner:** Full-stack developer + AI developer
**Effort:** 3 weeks
**Prerequisite:** `feat/ai-service` complete

---

#### I18N-1: Resume Translation Endpoint

**File to Create:** `ai-service/app/routes/translate.py`

```python
# POST /api/v1/translate/resume
# Body: {
#   "resume_s3_key": "users/.../resume.pdf",
#   "target_language": "de",   # ISO 639-1 code
#   "target_locale": "DE"      # ISO 3166-1 alpha-2
# }
# Response: {
#   "translated_resume_s3_key": "users/.../resume_de.pdf",
#   "language": "de"
# }
```

**Supported languages (MVP):** de (German), fr (French), es (Spanish), pt (Portuguese)

**Prompt template:**
```python
TRANSLATE_RESUME_PROMPT = """
Translate the following resume to {target_language_name}. Preserve all formatting,
dates, numbers, and proper nouns (company names, tools, technologies).
Do NOT translate: English technical terms commonly used in the target country's industry
(e.g., "React", "PostgreSQL", "API" remain in English in German resumes).
"""
```

---

#### I18N-2: Internationalized Job Boards

**Files to Create:**
- `internal/job/scrapers/reed_uk.go` — Reed.co.uk (UK)
- `internal/job/scrapers/stepstone_de.go` — StepStone (Germany)
- `internal/job/scrapers/workopolis_ca.go` — Workopolis (Canada)

**Each scraper follows the same `Scraper` interface** as Indeed/ZipRecruiter.
Additional field: `Country string` (ISO 3166-1 alpha-2: "GB", "DE", "CA")

**Database migration:**
```sql
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS country CHAR(2) DEFAULT 'US';
ALTER TABLE user_preferences ADD COLUMN IF NOT EXISTS target_countries TEXT[] DEFAULT '{"US"}';
```

**Acceptance Criteria:**
- [ ] Job discovery respects `user_preferences.target_countries`
- [ ] Resume translated to matching language before apply (DE jobs → German resume)
- [ ] Cover letter generated in target language

---

#### I18N-3: Frontend Language Selector

**File to Modify:** `frontend/src/app/dashboard/settings/page.tsx`

Add section:
```
Target Countries & Languages
☑ United States (English)
☐ United Kingdom (English)
☐ Germany (German — resume will be auto-translated)
☐ France (French — resume will be auto-translated)
```

**Acceptance Criteria:**
- [ ] Multi-select (user can target multiple countries)
- [ ] Warning shown when non-English country added: "We'll auto-translate your resume using AI"
- [ ] Saves via `PUT /users/me/preferences`

---

### BRANCH: `feat/interview-scheduler`

**Owner:** Full-stack developer
**Effort:** 2 weeks
**Prerequisite:** `feat/analytics` ANAL-2 (outcome tracking)

---

#### SCHED-1: Calendar Availability in Preferences

**Database migration:**
```sql
ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS calendar_availability JSONB DEFAULT '{}';
-- Schema:
-- {
--   "timezone": "America/New_York",
--   "slots": [
--     { "day": "Monday", "start": "09:00", "end": "17:00" },
--     { "day": "Tuesday", "start": "09:00", "end": "17:00" }
--   ],
--   "buffer_minutes": 30
-- }
```

**Files to Create:**
- `frontend/src/components/settings/availability-form.tsx` — weekly grid picker

**Acceptance Criteria:**
- [ ] User can set available time slots per day
- [ ] Buffer time between interviews configurable (30–120 min)
- [ ] Times displayed in user's local timezone

---

#### SCHED-2: Interview Request Detection

**File to Create:** `internal/notification/interview_detector.go`

**Problem:** Users forward recruiter emails to a dedicated inbox; system detects interview requests.

```go
// SES inbound email → SNS → Lambda → Go endpoint
// POST /webhooks/email/inbound
// Parses email body for interview request keywords:
//   "schedule", "availability", "interview", "call", "meet"
// If detected → creates application_event with type "interview_request"
// Notifies user via dashboard badge + webhook

func DetectInterviewRequest(emailBody string) (bool, *InterviewRequestDetails)
```

**Infrastructure needed:**
- SES inbound rule: `interview@autodreamapplier.com` → SNS topic → our webhook
- `terraform/staging/ses_inbound.tf`

**Acceptance Criteria:**
- [ ] Email with "Please let us know your availability for a 30-minute call" → detected
- [ ] Detection runs in < 200ms
- [ ] False positive rate < 5% on test corpus

---

#### SCHED-3: Auto-Reply with Availability

**File to Create:** `internal/notification/availability_reply.go`

```go
// Generates reply email with user's available slots
// Uses AI to format naturally: "I'm available Monday March 25 at 10am or 2pm EST..."

func BuildAvailabilityReply(
    ctx context.Context,
    userID uuid.UUID,
    originalEmail string,
    aiClient *ai.Client,
) (string, error)
```

**⚠️ User approval required:**
Auto-reply is **drafted, not sent**. User sees preview in dashboard with "Send Reply" button — never sent without explicit confirmation.

**Acceptance Criteria:**
- [ ] Reply never sent without user clicking "Send Reply" in dashboard
- [ ] Proposed times calculated from `calendar_availability` preferences
- [ ] Reply tone matches original email (formal if formal, casual if casual)

---

### BRANCH: `feat/salary-data`

**Owner:** Backend developer (Go)
**Effort:** 1.5 weeks
**Prerequisite:** `feat/job-boards` (more salary data in jobs table)

---

#### SAL-1: Salary Aggregation Service

**File to Create:** `internal/salary/service.go`

```go
// Aggregates salary data from jobs table
// Groups by: title (normalized) + location + seniority level
// Returns: min, median, max, sample_size

func (s *SalaryService) GetBenchmark(
    ctx context.Context,
    title string,
    location string,
) (*SalaryBenchmark, error)

type SalaryBenchmark struct {
    Title      string
    Location   string
    Min        int
    Median     int
    Max        int
    SampleSize int
    UpdatedAt  time.Time
}
```

**Database migration:**
```sql
-- Pre-computed salary benchmarks (rebuilt nightly)
CREATE TABLE salary_benchmarks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_key    VARCHAR(200),   -- normalized: lowercase, stripped seniority
    location_key VARCHAR(200),   -- "new-york-ny" format
    min_salary   INT,
    median_salary INT,
    max_salary   INT,
    sample_size  INT,
    updated_at   TIMESTAMP DEFAULT NOW(),
    UNIQUE (title_key, location_key)
);
```

**Title normalization:** Strip seniority prefixes: "Senior", "Junior", "Staff", "Principal", "Lead" → `"software engineer"` for grouping.

---

#### SAL-2: Salary Display on Match Cards

**File to Modify:** `frontend/src/components/matches/match-queue.tsx`

Add to each match card:
```
├──────────────────────────────────────────────────┤
│ 💰 Market rate: $165k – $210k  (based on 34 jobs) │
│    This role: $180k – $200k    ✓ Above market     │
└──────────────────────────────────────────────────┘
```

**API:** `GET /salary/benchmark?title=software+engineer&location=new+york`

**Acceptance Criteria:**
- [ ] Benchmark shown only when `sample_size >= 5` (not enough data otherwise)
- [ ] "Above/Below/At market" label computed and shown
- [ ] Benchmark data cached in Redis for 24 hours (not re-queried per card render)

---

### BRANCH: `feat/multi-tenant`

**Owner:** Senior backend developer (Go) — highest complexity branch in Phase 3
**Effort:** 4 weeks
**Prerequisite:** All MVP-B + Phase 2 branches stable

---

#### MT-1: Organization Schema

**Files to Create:** `migrations/XXXX_organizations.sql`

```sql
CREATE TABLE organizations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(200) NOT NULL,
    slug         VARCHAR(100) UNIQUE NOT NULL,  -- "acme-staffing"
    plan         VARCHAR(50) DEFAULT 'team',    -- team, agency, enterprise
    created_at   TIMESTAMP DEFAULT NOW(),
    owner_id     UUID REFERENCES users(id)
);

CREATE TABLE organization_members (
    org_id     UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) DEFAULT 'member',  -- admin, manager, member
    joined_at  TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id)
);

-- Org-level daily limits (override per-user limits)
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS daily_application_limit INT DEFAULT 100;
```

---

#### MT-2: Organization API

**File to Create:** `internal/organization/` (full package: handler, service, repository)

```go
// POST /organizations          — create org (owner auto-added as admin)
// GET  /organizations/me       — current user's org
// GET  /organizations/{id}/members — list members (admin only)
// POST /organizations/{id}/members — invite by email (admin only)
// DELETE /organizations/{id}/members/{userId} — remove member (admin only)
// GET  /organizations/{id}/stats — aggregate stats across all members
```

**Admin stats response:**
```json
{
  "members": [
    {
      "name": "Alice Smith",
      "email": "alice@company.com",
      "applications_today": 8,
      "applications_total": 142,
      "interviews": 12,
      "offers": 2
    }
  ],
  "org_totals": { "applications_today": 35, "interviews": 48, "offers": 7 }
}
```

**Acceptance Criteria:**
- [ ] Regular members cannot see other members' application details
- [ ] Admin can see aggregate stats but NOT individual cover letters/resumes
- [ ] Org `daily_application_limit` enforced across all members combined (shared Redis counter)
- [ ] Invite flow: sends email with magic link → recipient creates account → auto-joins org

---

#### MT-3: Billing Per Seat

**Prerequisite:** Stripe integration (separate `feat/billing` branch)

```
Agency plan: $X/seat/month (Volume discount: >10 seats = 20% off)
Admin billing page: shows seat count, invoice history, add/remove seats
```

**Files to Create:** `internal/billing/` (Stripe webhook handler + seat management)

---

### BRANCH: `feat/public-api`

**Owner:** Backend developer (Go)
**Effort:** 2 weeks
**Prerequisite:** MVP-B stable (stable API surface to expose)

---

#### API-1: API Key Management

**Files to Create:**
- `migrations/XXXX_api_keys.sql`
- `internal/apikeys/` (handler + repository)

```sql
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID REFERENCES users(id) ON DELETE CASCADE,
    key_hash     VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 of raw key
    key_prefix   VARCHAR(8) NOT NULL,           -- first 8 chars shown in UI: "ada_live"
    name         VARCHAR(100),                  -- "Production", "Test"
    last_used_at TIMESTAMP,
    created_at   TIMESTAMP DEFAULT NOW(),
    revoked_at   TIMESTAMP
);
```

**Key format:** `ada_live_<32 random bytes base62>` or `ada_test_<...>` for test mode.

**Endpoints:**
```go
// POST /api-keys            — create key (returns raw key once, then only prefix)
// GET  /api-keys            — list keys (only prefix + metadata, never raw)
// DELETE /api-keys/{id}     — revoke key
```

**Middleware:** `internal/auth/apikey_middleware.go`
- Reads `Authorization: Bearer ada_live_xxx` header
- SHA-256 hashes it, looks up `api_keys` table
- Injects user context — same as JWT middleware downstream

---

#### API-2: Rate Limiting by API Key Tier

```go
// Limits enforced via Redis sliding window
// Free tier:  100 requests/hour
// Pro tier:   1,000 requests/hour
// Enterprise: 10,000 requests/hour

// Rate limit headers returned on every response:
// X-RateLimit-Limit: 1000
// X-RateLimit-Remaining: 847
// X-RateLimit-Reset: 1710000000
```

---

#### API-3: Outbound Webhook Delivery

**File to Create:** `internal/webhook/delivery_worker.go`

Distinct from NOTIF-2 (user-configured Slack/Discord). This is **programmatic webhook delivery** for API customers.

```go
// Developer registers endpoint: POST /webhooks { "url": "...", "events": ["application.submitted"] }
// On event: enqueue delivery task → delivery_worker retries with exponential backoff
// Delivery log: store last 100 deliveries per endpoint with status + response body

type DeliveryWorker struct{}

func (w *DeliveryWorker) Deliver(ctx context.Context, endpoint string, payload WebhookPayload) error
// Retry: 3× at 30s, 5min, 30min — then mark endpoint as failing
// HMAC-SHA256 signature in X-Webhook-Signature header (like Stripe)
```

**Database:**
```sql
CREATE TABLE webhook_endpoints (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    url        VARCHAR(500) NOT NULL,
    secret     VARCHAR(64) NOT NULL,    -- for HMAC signing
    events     TEXT[] NOT NULL,
    enabled    BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id  UUID REFERENCES webhook_endpoints(id),
    event_type   VARCHAR(100),
    payload      JSONB,
    response_status INT,
    delivered_at TIMESTAMP,
    attempt      INT DEFAULT 1
);
```

**Acceptance Criteria:**
- [ ] Signature verified by receiving side: `HMAC-SHA256(secret, payload_body)`
- [ ] Failed delivery retried 3× before marking endpoint `enabled = false`
- [ ] Delivery log available via `GET /webhooks/{id}/deliveries`
- [ ] Delivery latency < 5 seconds from event trigger

---

### BRANCH: `feat/referrals`

**Owner:** Full-stack developer
**Effort:** 1 week
**Prerequisite:** Billing/payments in place

---

#### REF-1: Referral Schema & Code Generation

```sql
CREATE TABLE referrals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id     UUID REFERENCES users(id),
    referred_id     UUID REFERENCES users(id),
    code            VARCHAR(20) UNIQUE NOT NULL,  -- "ADA-JANE-X7K2"
    status          VARCHAR(50) DEFAULT 'pending', -- pending, converted, rewarded
    referrer_credit_applied BOOLEAN DEFAULT FALSE,
    referred_credit_applied BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP DEFAULT NOW(),
    converted_at    TIMESTAMP
);
```

**Referral flow:**
1. User generates code at `GET /referrals/my-code` (auto-created on first call)
2. New user signs up with `?ref=ADA-JANE-X7K2` in URL
3. On first paid month → both users get 1 month free credit
4. Credit applied via `billing_credits` table (deducted from next invoice)

**Acceptance Criteria:**
- [ ] Referral code is stable (same code on every call, not re-generated)
- [ ] Credit awarded only once per referral (idempotent)
- [ ] Referred user must complete first payment before credit activates
- [ ] Dashboard shows "You've referred N people — N credits earned"

---

### BRANCH: `feat/retention`

**Owner:** Backend developer (Go) + Data analyst
**Effort:** 1.5 weeks
**Prerequisite:** `feat/analytics` ANAL-1 (need outcome data)

---

#### RET-1: Churn Risk Scoring

**File to Create:** `internal/retention/churn_scorer.go`

```go
// Rule-based churn score (0.0–1.0):
// High churn risk indicators:
//   - No login in 7 days: +0.3
//   - No applications in 7 days (despite active subscription): +0.25
//   - Pending matches > 10 (not reviewing): +0.15
//   - No preferences set: +0.2
//   - Failed applications > 50%: +0.1

type ChurnScore struct {
    UserID    uuid.UUID
    Score     float64
    Reasons   []string    // human-readable: "No applications in 7 days"
    RiskLevel string      // "low", "medium", "high"
}

func ScoreUser(ctx context.Context, userID uuid.UUID, db *sql.DB) (*ChurnScore, error)
```

**Runs:** Nightly cron, stores results in `user_churn_scores` table.

---

#### RET-2: Re-Engagement Email Campaign

**File to Create:** `internal/retention/reengagement_service.go`

**Email triggers:**
| Trigger | Delay | Subject |
|---------|-------|---------|
| No login | 7 days | "You have {N} new job matches waiting" |
| No applications | 7 days | "Jobs you might have missed at {Company}" |
| High churn score | Immediate | "Tips to get more interviews with AutoDreamApplier" |

```go
func (r *ReengagementService) SendPendingCampaigns(ctx context.Context) error
// Called by nightly cron
// Skips users who already received a re-engagement email in the past 14 days
// Skips users who unsubscribed
```

**Acceptance Criteria:**
- [ ] No more than 1 re-engagement email per user per 14 days
- [ ] Unsubscribe link in every email — one-click, no confirmation required
- [ ] Email personalized with actual pending match count
- [ ] `APP_ENV=test` logs email to stdout, no SES send

---

## Developer Workflow & Conventions

> Read this section before starting work on any branch.

---

### Git Branching Convention

```bash
# Branch naming
feat/<feature-name>          # new feature (maps to branch names in this doc)
fix/<short-description>      # bug fix
chore/<short-description>    # non-functional (deps, tooling, docs)
hotfix/<short-description>   # emergency prod fix (branch from main, merge to main immediately)

# Examples
git checkout -b feat/ats-plugins
git checkout -b fix/lever-captcha-detection
git checkout -b chore/update-go-deps
```

**PR rules:**
- Branch from `main`, merge back to `main` via PR
- PR title format: `[TASK-ID] Short description` → e.g. `[ATS-1] Add Lever ATS plugin`
- Every PR needs: description of changes, acceptance criteria checklist (copy from this doc), test evidence
- PRs blocked from merging if CI fails or if another dev is mid-merge on a conflicting file

---

### Database Migration Naming Convention

```
migrations/
  0001_initial_schema.sql
  0002_add_user_preferences.sql
  0003_add_application_events.sql
  ...
  0010_auto_apply_schedule.sql       ← feat/auto-apply branch
  0011_application_ai_keys.sql       ← feat/ai-service branch
  0012_webhook_settings.sql          ← feat/notifications branch
  0013_linkedin_daily_count.sql      ← feat/job-boards branch
  0014_pgvector.sql                  ← feat/pgvector-matching (Phase 2)
```

**Rules:**
- Always sequential — check `migrations/` before creating a new one
- Never modify an existing migration — create a new one with `ALTER TABLE`
- Every migration must be **idempotent**: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`
- Every migration **must have a rollback comment** at the top:
  ```sql
  -- ROLLBACK: ALTER TABLE user_preferences DROP COLUMN IF EXISTS auto_apply_enabled;
  ```

---

### Go Error Handling Patterns

All packages must follow these patterns for consistency:

```go
// 1. Package-level sentinel errors (define at top of package)
var (
    ErrNotFound         = errors.New("ats: record not found")
    ErrCaptchaRequired  = errors.New("ats: captcha required")
    ErrDailyLimitReached = errors.New("apply: daily limit reached")
    ErrSourceBlocked    = errors.New("scraper: source returned 403/429")
)

// 2. Wrap errors with context (always use %w for unwrapping)
if err != nil {
    return fmt.Errorf("lever plugin Fill: clicking submit: %w", err)
}

// 3. HTTP handler error mapping pattern
func respondError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, ErrNotFound):
        http.Error(w, "not found", http.StatusNotFound)
    case errors.Is(err, ErrDailyLimitReached):
        http.Error(w, "daily limit reached", http.StatusTooManyRequests)
    default:
        log.Error().Err(err).Msg("internal error")
        http.Error(w, "internal server error", http.StatusInternalServerError)
    }
}
```

---

### Go Testing Requirements

Every new package needs tests. Minimum coverage rules:

| Package Type | Min Coverage | Test Type |
|-------------|-------------|-----------|
| ATS plugins | 70% | Unit (mock Playwright page) |
| Scrapers | 60% | Unit (fixture HTML, no network) |
| Services | 80% | Unit (mock repository) |
| Repositories | 70% | Integration (real Postgres via Docker Compose) |
| HTTP handlers | 70% | Integration (httptest.Server) |
| AI service routes | 80% | Unit (mock Anthropic client) |

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage

# Run only unit tests (no DB)
make test-unit

# Run integration tests (requires Docker Compose up)
make test-integration
```

**Fixture file convention** (for scraper tests):
```
internal/job/scrapers/testdata/
  ziprecruiter_search_page.html    # saved HTML from ZipRecruiter search
  glassdoor_results.html
  linkedin_jobs.html
```

---

### Python AI Service Standards

```python
# All endpoints must:
# 1. Validate inputs with Pydantic (raises 422 automatically)
# 2. Log tokens used: logger.info("tokens_used", extra={"tokens": response.usage.total_tokens})
# 3. Never log resume content or cover letter text (PII)
# 4. Return consistent error format:
#    { "detail": "resume_s3_key not found in S3", "error_code": "S3_NOT_FOUND" }

# requirements.txt — pin all versions
anthropic==0.25.0
fastapi==0.111.0
pydantic-settings==2.2.1
boto3==1.34.0
reportlab==4.1.0
sentence-transformers==2.7.0  # Phase 2 only
uvicorn[standard]==0.29.0
pytest==8.2.0
httpx==0.27.0  # for test client
```

---

### Frontend Conventions

```typescript
// 1. All API calls go through frontend/src/lib/api.ts — never fetch() directly in components
// 2. Types live in frontend/src/lib/types.ts — camelCase
// 3. All API responses are snake_case from backend — map in api.ts
// 4. Loading states: use React's Suspense + loading.tsx where possible
// 5. Error states: every async component needs an error.tsx sibling
// 6. Forms: use react-hook-form + zod for validation (already in codebase)

// api.ts function naming convention:
export async function getMatches(params: MatchQueryParams): Promise<Match[]>
export async function approveMatch(matchId: string): Promise<void>
export async function bulkApproveMatches(matchIds: string[]): Promise<BulkResult>
//                    ^ verb + noun (camelCase)
```

---

### Local Development Setup

```bash
# 1. Start infrastructure
docker compose up -d postgres redis minio

# 2. Run migrations
make migrate

# 3. Start API gateway (hot reload via air)
make dev-api

# 4. Start frontend
cd frontend && npm run dev

# 5. Optional: start AI service
cd ai-service && uvicorn main:app --reload --port 8001

# 6. Optional: start worker
make dev-worker

# Environment file
cp .env.example .env
# Edit .env — at minimum set:
# APP_ENV=development
# DATABASE_URL=postgres://postgres:postgres@localhost:5432/autodreamapplier?sslmode=disable
# REDIS_HOST=localhost
# DEV_JWT_SECRET=dev-secret-change-in-production-32b
# ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
```

---

### PR Checklist Template

Copy this into every PR description:

```markdown
## Changes
<!-- What this PR does -->

## Task Reference
<!-- e.g. ATS-1: Lever ATS Plugin -->

## Acceptance Criteria
<!-- Copy from IMPLEMENTATION_PLAN.md -->
- [ ] ...
- [ ] ...

## Testing Evidence
<!-- Screenshot, curl output, or test output showing it works -->

## Migration Notes
<!-- If adding a migration: is it idempotent? Does it have a rollback comment? -->

## Breaking Changes
<!-- Does this change any API contract or shared interface? -->
```

---

## Tech Stack Reference

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+ (Chi, Asynq) |
| AI Service | Python 3.11+ (FastAPI, Anthropic API) |
| Frontend | Next.js 14, Tailwind CSS |
| Database | PostgreSQL 16 + pgvector (Phase 2) |
| Cache/Queue | Redis 7 + Asynq |
| Auth | AWS Cognito (prod) / HS256 dev tokens (local) |
| Browser | Playwright Chromium + EC2 Spot Fleet |
| Proxy | Residential proxies (Bright Data / IPRoyal) |
| CAPTCHA | Behavioral avoidance + 2Captcha fallback |
| LLM | Claude Haiku (~$0.003/application) |
| Embeddings | all-MiniLM-L6-v2, 384-dim CPU (Phase 2) |
| Storage | AWS S3 (versioned buckets) |
| IaC | Terraform |
| CI/CD | GitHub Actions |
| Monitoring | Grafana + Prometheus + Sentry + CloudWatch |
| Email | AWS SES |

---

## Environment Variables Checklist

```bash
# Required for production
APP_ENV=production
APP_PORT=8080
DATABASE_URL=postgres://...
REDIS_HOST=...
REDIS_PORT=6379
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
S3_BUCKET_RESUMES=...
S3_BUCKET_SCREENSHOTS=...
COGNITO_USER_POOL_ID=...
COGNITO_APP_CLIENT_ID=...
ENCRYPTION_KEY=<32-byte hex>
SES_FROM_EMAIL=...
SES_DASHBOARD_URL=...
BROWSER_POOL_URL=...
AI_SERVICE_URL=...

# Phase 2 additions
ANTHROPIC_API_KEY=sk-ant-...      # used by ai-service
LINKEDIN_PROXY_URL=...             # required for LinkedIn scraper
TWOCAPTCHA_API_KEY=...             # CAPTCHA fallback

# Development only (APP_ENV != production)
DEV_JWT_SECRET=dev-secret-change-in-production-32b
```

---

## Integration Points — Quick Reference

When branches merge, these are the contracts they must honour:

| Producer Branch | Consumer Branch | Contract |
|----------------|----------------|---------|
| `feat/ai-service` | `feat/ats-plugins` | `POST /api/v1/resume/tailor` + `/cover-letter/generate` + `/form-qa/answer` — request/response schemas defined in AI-2, AI-3, AI-4 |
| `feat/job-boards` | `feat/auto-apply` | `jobs.ats_type` populated by `ats.DetectFromURL()` during discovery — apply worker reads this |
| `feat/auto-apply` | `feat/notifications` | `application_service.Submit()` fires webhook events via `webhookService.Send()` — NOTIF-2 |
| `feat/infra-staging` | All | ECS task definitions + SSM params in place before any service deploy |
| `feat/ats-plugins` + `feat/ai-service` | `feat/frontend-v2` FE-2 | `GET /applications/{id}` must include `tailored_resume_s3_key` pre-signed URL + `cover_letter_text` |

---

## Merge Order

```
feat/infra-staging       ← merge first (staging exists)
feat/job-boards          ← independent
feat/ats-plugins         ← independent
feat/auto-apply          ← after job-boards (more match volume)
feat/ai-service          ← independent; unblocks ats-plugins worker
feat/notifications       ← after auto-apply (shares event hooks)
feat/frontend-v2         ← after all backend branches
```
