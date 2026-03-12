# AutoDreamApplier — Implementation Plan

Cloud-native, server-side job application automation SaaS. This document tracks
current implementation state and the full roadmap through all phases.

---

## Current Status: MVP-A In Progress (Weeks 1–10)

### Completed ✅

| Component | Details |
|-----------|---------|
| Project scaffolding | Go module, Chi router, Docker Compose dev env |
| Database schema | Full PostgreSQL migrations (`migrations/`) — users, resumes, jobs, matches, applications, board_credentials, application_events |
| Config system | `pkg/config/config.go` — env-based, defaults for local dev |
| Logger | `pkg/logger/` — zerolog, production/dev modes |
| S3 client | `pkg/s3/` — upload, download, delete, pre-signed URLs |
| Auth middleware | `internal/auth/cognito.go` — Cognito RSA JWT + dev HS256 JWT routing via `WithDevSecret` |
| Dev auth handler | `internal/auth/dev_handler.go` — bcrypt email/password + HS256 JWT for local dev (no Cognito needed) |
| User handler | `internal/user/handlers/user_handler.go` — profile CRUD, resume upload/list/delete/set-primary, board credentials (AES-256-GCM), dashboard stats |
| User repository | `internal/user/repository/` — all DB queries for users, resumes, credentials |
| Application service | `internal/application/service/` — approve, reject, submit, emergency stop |
| Application handler | `internal/application/handler/` — HTTP routes for application actions |
| Application repository | `internal/application/repository/` — DB queries with Asynq task enqueue |
| 2-stage async pipeline | Asynq tasks: `TypeAIPrep` → `TypeBrowserApply`; workers in `internal/application/workers/` |
| AI prep worker | `internal/application/workers/ai_prep_worker.go` — calls AI service, stores to S3, advances status to `ai_ready`, enqueues browser task |
| Browser apply worker | `internal/application/workers/browser_apply_worker.go` — acquires browser, fills ATS form, stores screenshot, advances status to `applied` or `failed` |
| ATS plugin system | `internal/ats/` — plugin interface + registry; Greenhouse plugin implemented |
| Browser pool client | `internal/browser/` — HTTP client to EC2 browser pool |
| Job matcher | `internal/jobmatcher/` — keyword scoring, preference filtering, match queue handler |
| Notification client | `internal/notification/` — AWS SES email (nil-safe when unconfigured) |
| API Gateway | `cmd/api-gateway/main.go` — all routes wired, graceful shutdown |
| Frontend (Next.js 14) | Auth pages, dashboard layout, match queue, applications list, resumes page, settings page, sidebar nav |
| Worker integration tests | `internal/application/workers/*_test.go` — happy path, error path, bad payload tests |

### Bug Fixes Applied (This Session) 🐛→✅

| Bug | File | Fix |
|-----|------|-----|
| Dev HS256 JWT rejected by Cognito RSA middleware | `internal/auth/cognito.go` | Added `devSecret` field + `WithDevSecret` + `validateDevToken` + `peekAlg`; routes HS256 tokens to dev validator |
| Dev token `sub` was UUID, but DB lookup uses `cognito_sub = "dev:email"` | `internal/auth/dev_handler.go` | Changed `mintDevToken(user.ID.String(), ...)` to `mintDevToken(user.CognitoSub, ...)` in Login + Register |
| Dev secret never wired into `CognitoAuth` | `cmd/api-gateway/main.go` | Added `cognitoAuth.WithDevSecret(cfg.App.DevJWTSecret)` when `env != "production"` |
| Resume upload: handler expects `"resume"`, frontend sends `"file"` | `internal/user/handlers/user_handler.go:210` | Changed `r.FormFile("resume")` → `r.FormFile("file")` |
| `handleSetPrimary` state update uses `is_primary` (snake_case) not `isPrimary` (camelCase) | `frontend/src/components/resumes/resume-list.tsx:70` | Changed `is_primary:` → `isPrimary:` |

---

## MVP-A Remaining (Weeks 5–10)

### Job Discovery Service (`cmd/job-discovery/`)
- [x] Indeed scraper — job search API or HTML scraping, pagination, deduplication by `external_id`
- [x] Glassdoor scraper — similar pattern to Indeed
- [x] Job storage — `internal/job/repository/` with upsert on `(external_id, source_board)`
- [x] Scheduler — cron-style periodic discovery (every 4 hours per board)
- [x] Rate limiting per board (existing `pkg/ratelimit/` or simple time.Sleep)

### Job Matching Service (partial — needs preference integration)
- [x] `user_preferences` table population — currently schema exists but no UI/handler to set preferences
- [x] Preferences handler in `internal/user/handlers/user_handler.go` — `GET/PUT /users/me/preferences`
- [x] Matching engine — keyword intersection between `user_preferences.target_titles` and `jobs.title`, location filter, salary filter
- [x] Match creation — write matched jobs to `matches` table with `match_score` and `match_breakdown`
- [x] Auto-approve flow — if `user.apply_mode = "auto"` and `match_score >= user.auto_threshold`, skip review

### Frontend Gaps
- [x] Match queue actions — approve/reject buttons with API calls wired
- [x] Preferences form — titles, locations, remote preference, salary range, exclusions
- [x] Application detail page — status timeline, screenshot proof, cover letter preview
- [x] Settings page — auto-apply toggle, daily limit, timezone

### Infrastructure (Local Dev Complete, AWS Needed for Staging)
- [x] Docker Compose: confirm all services start and pass healthchecks
- [x] `make test` CI passing — ensure `TEST_DATABASE_URL` set in GitHub Actions
- [x] Terraform initial apply for staging environment

---

## MVP-B: Expand Coverage (Weeks 11–16)

### ATS Plugins
- [ ] Lever plugin — `internal/ats/plugins/lever.go`
- [ ] Workday plugin — `internal/ats/plugins/workday.go`
- [ ] ATS auto-detection from `jobs.url` or DOM (`internal/ats/detector.go`)

### More Job Boards
- [ ] ZipRecruiter scraper
- [ ] LinkedIn (conservative: 3/day/user, Easy Apply only — Phase 2 candidate)

### Auto-Apply Mode
- [ ] Configurable per-user `auto_threshold` in preferences
- [ ] Batch auto-approval job triggered on match creation when score exceeds threshold
- [ ] Scheduling: business hours only (configurable timezone), rate-limit enforcement
- [ ] Dashboard: auto-apply toggle with confirmation modal

### Dashboard Improvements
- [ ] Bulk approve/reject in match queue
- [ ] Application search + filter (by status, company, date)
- [ ] Match thumbs up/down feedback (writes `matches.user_feedback`)
- [ ] Weekly email digest (SES template + cron job)

---

## Phase 2: AI & Scale (Months 5–8)

### AI Service (Python FastAPI — `ai-service/`)
- [ ] Resume tailoring endpoint — `POST /api/v1/resume/tailor` (Claude Haiku via Anthropic API)
- [ ] Cover letter generation — `POST /api/v1/cover-letter/generate`
- [ ] Form Q&A — `POST /api/v1/form-qa/answer` (common ATS questions answered using resume context)
- [ ] Wire AI client in `AIPrepWorker` — currently calling stub; needs real AI service URL

### Semantic Matching (pgvector)
- [ ] Enable `pgvector` extension in migrations
- [ ] Embed job descriptions with `all-MiniLM-L6-v2` (384-dim, CPU) → `job_embeddings.embedding`
- [ ] Embed user resume → `resumes.embedding`
- [ ] Replace keyword scoring with cosine similarity search
- [ ] IVFFlat index on `job_embeddings` for fast retrieval

### More ATS Plugins
- [ ] iCIMS, Taleo, SuccessFactors, SmartRecruiters, BambooHR
- [ ] Generic form-fill fallback (labeled input detection + AI Q&A for unknown fields)

### More Job Boards
- [ ] CareerBuilder, AngelList, RemoteOK, Dice
- [ ] LinkedIn Easy Apply (3/day/user hard limit; residential proxy per request)

### Resume A/B Testing
- [ ] `resume_versions` table — multiple tailored variants per job
- [ ] Track which version leads to callbacks (`resumes.interview_count`)
- [ ] A/B dashboard showing conversion rates per version

### Scam Detection
- [ ] ML classifier on `jobs.description` — keyword patterns (gift cards, no experience, vague pay)
- [ ] `jobs.is_scam` + `jobs.scam_score` populated during discovery
- [ ] Filter scam jobs from match queue (configurable threshold)

### Slack/Discord Notifications
- [ ] Notification service: add Slack webhook + Discord webhook alongside SES
- [ ] User configures webhook URL in Settings
- [ ] Events: new match, application submitted, interview scheduled

---

## Phase 3: Analytics & Growth (Months 9–14)

### Full Funnel Analytics
- [ ] Track: `applied → viewed → phone_screen → interview → offer → accepted`
- [ ] Dashboard charts: applications per day, interview rate per resume version, offer rate per board
- [ ] Outcome entry: user manually marks interviews/offers received (email parsing optional later)

### Follow-Up Automation
- [ ] Configurable follow-up email templates (1-week, 2-week)
- [ ] Queue follow-up tasks after `applied_at + N days`
- [ ] Unsubscribe detection (pause follow-ups if rejection received)

### Multi-Language Support
- [ ] Resume translation endpoint in AI service
- [ ] Cover letter generation in target language
- [ ] Internationalized job boards (UK, Germany, Canada)

### Interview Scheduling Assistant
- [ ] Calendar availability extraction from user preferences
- [ ] Auto-reply to interview request emails with available slots (SES + calendar API)

### Salary Benchmarking
- [ ] Aggregate salary data from discovered jobs
- [ ] Salary range estimates by title + location displayed on match cards

### Team/Agency Accounts
- [ ] Organization model — one org, N user seats
- [ ] Admin role — can view all org members' stats
- [ ] Agency billing: volume discounts

### API Access (Pro/Enterprise)
- [ ] REST API with API key auth
- [ ] Webhook delivery for application events
- [ ] Rate-limited by tier

### Growth Systems
- [ ] Referral program — `referrals` table, give 1 month free, get 1 month free
- [ ] Churn prediction — flag users with 7-day inactivity + no applications
- [ ] Re-engagement email campaign (SES)

---

## Tech Stack Reference

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+ (Chi, Asynq) |
| AI Service | Python 3.11+ (FastAPI, Anthropic API) |
| Frontend | Next.js 14, Tailwind CSS |
| Database | PostgreSQL 16 + pgvector (RDS Multi-AZ) |
| Cache/Queue | Redis 7 + Asynq (ElastiCache) |
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

```
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
ENCRYPTION_KEY=<32-byte hex>   # required to save board credentials
SES_FROM_EMAIL=...
SES_DASHBOARD_URL=...
BROWSER_POOL_URL=...
AI_SERVICE_URL=...

# Development only (APP_ENV != production)
DEV_JWT_SECRET=dev-secret-change-in-production-32b
```

---

## Key Architecture Decisions

1. **2-stage async pipeline** — AI prep (3–8s) runs first; browsers only spin up for pre-prepared content, cutting apply time from ~70s to ~40s and wasting zero expensive browser idle time.

2. **Dev HS256 JWT alongside Cognito RSA** — `CognitoAuth.WithDevSecret` allows local dev without AWS Cognito. Routed by peeking at the JWT `alg` header before validation. Never enabled in production (`cfg.App.Env != "production"` guard).

3. **ATS plugin architecture** — `internal/ats/registry.go` provides a `Plugin` interface so new ATS types can be added without touching the apply worker. Auto-detection via URL patterns will be added in MVP-B.

4. **Conservative rate limits** — Indeed max 10/day/user, Glassdoor max 8/day/user, LinkedIn max 3/day/user. Auto-pause on CAPTCHA frequency >3 in 10 minutes.

5. **Scam detection before matching** — Jobs filtered during discovery to prevent wasting browser time and user attention on fraudulent listings.

6. **Credential encryption** — Board credentials encrypted AES-256-GCM at rest using a 32-byte KMS-managed key. Empty `ENCRYPTION_KEY` returns HTTP 500 with a clear error rather than silently storing plaintext.
