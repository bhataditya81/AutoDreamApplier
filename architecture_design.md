# AutoDreamApplier: Architecture & Low-Level Design

This document is the authoritative architectural reference for AutoDreamApplier — a cloud-native SaaS that automates job applications end-to-end using AI resume tailoring, semantic job matching, headless browser automation, and a pluggable ATS engine.

> **Status Legend:** ✅ Implemented | 🔄 In Progress | 🗓️ Planned

---

## 1. High-Level Architecture

AutoDreamApplier is built as an asynchronous, microservice-oriented system. Scraping unstructured job boards and driving headless browsers are inherently brittle operations — isolating them from the core API ensures the system remains stable under scraper failures, rate limits, and ATS DOM changes.

<details>
<summary><b>Click to expand — Full System Diagram</b></summary>

```mermaid
flowchart TD
    %% External
    User((User Browser))
    JobBoards["Job Boards\nIndeed · Glassdoor\nZipRecruiter · LinkedIn"]
    ATS["ATS Portals\nGreenhouse · Lever · Workday\nTaleo · iCIMS · SmartRecruiters\nBambooHR · Ashby"]
    Claude["Anthropic Claude API\n(Cover Letter / Resume)"]
    Cognito["AWS Cognito\n(RSA JWT in prod)"]
    Slack["Slack / Discord\nWebhooks"]
    SES["AWS SES\nEmail"]

    %% Frontend
    UI["Next.js 14 Frontend\nDashboard · Match Queue\nResumes · Analytics · Salary"]

    %% Core API
    subgraph CoreAPI["Core Network"]
        API["API Gateway\nGo / chi router\nJWT Auth · REST"]
    end

    %% Background Workers (Go)
    subgraph WorkerNetwork["Worker Network"]
        JD["Job Discovery\nGo · Scrapers + Scam Detector"]
        JM["Job Matcher\nGo · Keyword + Semantic Scorer"]
        AE["Apply Engine\nGo · Asynq Server\nAI Prep → Browser Apply"]
        FOLLOW["Follow-Up Scheduler\n1hr tick · 7-day threshold"]
        ANALYTICS["Analytics Service\nGo · Funnel + Trend"]
        SALARY["Salary Benchmarks\nGo · Market Positioning"]
    end

    %% Compute
    subgraph ComputeNetwork["Isolated Compute"]
        AI["AI Service\nPython / FastAPI\nEmbeddings · Cover Letters"]
        BP["Browser Pool\nGo / Playwright\nATS Form Automation"]
    end

    %% Data
    subgraph DataLayer["Data Persistence"]
        DB[("PostgreSQL 16\n+ pgvector\nAll tables")]
        Redis[("Redis 7\nAsynq Queue\nRate Limiting")]
        S3[("AWS S3\nResumes · Screenshots\nTailored PDFs")]
    end

    %% Notification
    NOTIF["Notification Service\nWebhook · Email · Digest"]

    %% Connections
    User -- HTTPS --> UI
    UI -- REST / JWT --> API
    API -- Auth --> Cognito

    API -- Read/Write --> DB
    API -- Enqueue TypeAIPrep --> Redis

    JD -- Scrapes --> JobBoards
    JD -- Upsert Jobs --> DB
    JD -- Scam Score --> DB

    JM -- Read Jobs + Prefs --> DB
    JM -- Semantic Embed --> AI
    JM -- Write Matches --> DB

    AE -- Dequeue --> Redis
    AE -- Read/Write --> DB
    AE -- Tailor Resume --> AI
    AI -- Tokens --> Claude
    AE -- Store PDF --> S3
    AE -- Execute --> BP
    BP -- DOM --> ATS
    BP -- Screenshot --> S3
    AE -- Notify --> NOTIF

    FOLLOW -- Query stale apps --> DB
    FOLLOW -- Remind --> NOTIF

    ANALYTICS -- Aggregate --> DB
    SALARY -- Benchmark --> DB

    NOTIF -- Webhook --> Slack
    NOTIF -- Email --> SES
```

</details>

---

## 2. Component Reference

### 2.1 API Gateway ✅ `cmd/api-gateway`

The synchronous, stateless entrypoint for all user interactions.

**Responsibilities:**
- JWT authentication — `peekAlg()` routes HS256 tokens to `validateDevToken` (local dev) and RS256 tokens to Cognito JWKS (production). Zero flag switching.
- `UserResolver` interface resolves `user_id` from JWT context rather than query parameters — frontend never sends user ID explicitly.
- REST endpoints: profile, preferences, resumes (with A/B toggle), match queue, application tracking, emergency stop, analytics, salary benchmarks, notification settings.
- On match approval: creates `applications` row (`status=queued`), enqueues `TypeAIPrep` to Redis, returns `202 Accepted` immediately.
- Mounts FollowUpScheduler as a background goroutine.

**Key interfaces:**
```go
type UserResolver interface {
    GetUserIDBySub(ctx context.Context, sub string) (uuid.UUID, error)
}
```

**Auth flow:**
```
Request Header: Authorization: Bearer <token>
        │
        ▼
   peekAlg(token)
        ├── alg=HS256 → validateDevToken(secret) → ctx with userSub
        └── alg=RS256 → fetchCognitoJWKS() → validateRS256() → ctx with userSub
                                │
                         getUserID(r, fallback)
                         ├── JWT context sub → UserResolver.GetUserIDBySub()
                         └── fallback: query param user_id (deprecated)
```

---

### 2.2 Job Discovery Service ✅ `cmd/job-discovery`

Continuous data ingestion pipeline running on a 2-hour schedule.

**Scrapers implemented:**
| Scraper | Strategy | Notes |
|:--------|:---------|:------|
| Indeed | HTTP + CSS selectors | Standard HTML parsing |
| Glassdoor | `__NEXT_DATA__` JSON extraction (primary) + DOM fallback | Next.js SSR data embedded in page |
| ZipRecruiter | REST API via search URL | Structured JSON response |
| LinkedIn | Authenticated session scraping | Requires cookie rotation |

**Scam Detection Pipeline ✅** (`internal/jobdiscovery/scam/detector.go`)

Every job passes through an 8-signal heuristic scorer before DB insertion:

| Signal | Weight | Example |
|:-------|:------:|:--------|
| Unrealistic salary ($500k+ entry level) | 0.20 | "Entry level, $500k/yr" |
| Upfront payment required | 0.25 | "Pay $99 for training kit" |
| No company name | 0.15 | `company=""` |
| Suspicious title keywords | 0.10 | "Work from home", "Make $1000/day" |
| Personal email domain | 0.10 | `@gmail.com`, `@yahoo.com` |
| Apply URL ≠ company domain | 0.10 | Company=Google, URL=sketchy.biz |
| Description < 100 chars | 0.05 | Stub posting |
| Buzzword overload | 0.05 | "unlimited income potential" |

`IsScam = score >= 0.5`. Scam jobs are stored with `is_scam=true` and filtered from match generation.

**DB Upsert pattern:**
```sql
INSERT INTO jobs (...) VALUES (...)
ON CONFLICT (external_id, source_board) DO UPDATE
SET title=EXCLUDED.title, salary_min=EXCLUDED.salary_min, ...
```

---

### 2.3 Job Matcher Service ✅ `cmd/job-matcher`

Filters discovered jobs against user preferences using combined keyword + semantic scoring.

**Scoring formula:**
```
combined_score = 0.6 × keyword_score + 0.4 × semantic_score
```

**Keyword score** — evaluates: title overlap, location match, remote preference, salary range fit, exclusion list filtering (case-insensitive).

**Semantic score** ✅ (`internal/jobmatcher/scorer/semantic_scorer.go`) — cosine similarity between `all-MiniLM-L6-v2` embeddings of resume text and job description. Falls back to `0.5` (neutral) when AI service is unavailable — pipeline continues without degradation.

**pgvector integration** ✅ (`migrations/000006_add_embeddings.up.sql`):
```sql
ALTER TABLE jobs ADD COLUMN embedding vector(384);
CREATE INDEX jobs_embedding_ivfflat ON jobs
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

Jobs batch-embedded every 5 minutes by the embedding ticker in `cmd/job-discovery`.

**Match output:** Creates `matches` row with `match_score`, `match_breakdown` JSON (per-signal explanation), `status=pending`.

---

### 2.4 Apply Engine ✅ `cmd/apply-engine`

The asynchronous orchestrator that drives the two-stage application pipeline.

**Architecture:** Asynq server with two weighted queues:
- `QueueAIPrep` (weight 6) — cheap CPU/network operations
- `QueueBrowserApply` (weight 3) — expensive browser compute

**Additional schedulers running as goroutines:**
- **Auto-Apply Scheduler:** Reads users with `auto_apply_enabled=true`, respects `apply_start_hour`, `apply_end_hour`, `apply_timezone`, enforces `daily_application_limit` via Redis rate limiter.
- **Follow-Up Scheduler** ✅: 1-hour tick, queries applications older than 7 days with no outcome, sends reminder emails, marks `follow_up_sent` to prevent re-sending.
- **Weekly Digest Scheduler:** Aggregates weekly activity stats, sends email digest.

**Resume A/B Testing** ✅: On each application, `GetABResume()` uses weighted random sampling across `ab_enabled` resumes. DB triggers auto-update `total_applications`, `total_interviews`, `total_offers` stats.

---

### 2.5 Browser Pool ✅ `cmd/browser-pool`

Stateless headless browser execution fleet.

**Operates as a standalone HTTP server.** Apply Engine sends `POST /apply` with:
```json
{
  "ats_type": "greenhouse",
  "apply_url": "https://boards.greenhouse.io/company/jobs/123",
  "resume_s3_key": "users/u1/tailored/r1.pdf",
  "user_data": { "name": "...", "email": "...", "phone": "..." },
  "cover_letter": "...",
  "custom_answers": { "question_text": "answer" }
}
```

**ATS Plugin Registry** ✅ (`internal/ats/`):

| Plugin | Match Pattern | Status |
|:-------|:-------------|:------:|
| Greenhouse | `boards.greenhouse.io/*` | ✅ |
| Lever | `jobs.lever.co/*` | ✅ |
| Workday | `*.myworkdayjobs.com/*` | ✅ |
| Taleo | `*.taleo.net/*` | ✅ |
| iCIMS | `*.icims.com/*` | ✅ |
| SmartRecruiters | `jobs.smartrecruiters.com/*` | ✅ |
| BambooHR | `*.bamboohr.com/jobs/*` | ✅ |
| Ashby | `jobs.ashbyhq.com/*` | ✅ |

Plugin interface:
```go
type Plugin interface {
    Name()       string
    Match(url string) bool
    Fill(ctx context.Context, page *playwright.Page, data *ApplicationData) error
    Submit(ctx context.Context, page *playwright.Page) error
    ValidateURL(url string) bool
}
```

---

### 2.6 AI Service ✅ `ai-service/`

Python FastAPI microservice for ML operations. **Fully implemented and deployed.**

**Endpoints:**
| Endpoint | Method | Function |
|:---------|:-------|:---------|
| `/api/v1/embeddings/text` | POST | Single text → 384-dim float32 vector |
| `/api/v1/embeddings/batch` | POST | Up to 100 texts batch embed |
| `/api/v1/tailor` | POST | Resume tailoring via Claude API |
| `/api/v1/cover-letter` | POST | Cover letter generation |
| `/api/v1/qa` | POST | ATS form Q&A via RAG |

**Embedding model:** `all-MiniLM-L6-v2` (sentence-transformers), lazy-loaded on first request, ~90 MB in memory.

---

### 2.7 Notification Service ✅ `internal/notification/`

Multi-channel notification delivery.

**Channels:**
- **Slack webhooks:** Rich message blocks with application status, job title, company, dashboard link.
- **Discord webhooks:** Embed format with color-coded status.
- **SES email:** Go HTML templates for application submitted, failed, weekly digest, follow-up reminder.

**Event types fired:**
| Event | Trigger | Channels |
|:------|:--------|:---------|
| `EventApplicationSubmitted` | Browser apply success | Slack, Discord, Email |
| `EventApplicationFailed` | AI prep or browser failure | Slack, Discord, Email |
| `EventDailyLimitReached` | Auto-apply daily cap hit | Slack, Discord |
| `EventFollowUpReminder` | 7-day stale application | Email |
| `EventWeeklyDigest` | Sunday 9am | Email |

Webhook delivery is nil-safe: if `WebhookService` is not configured, all `Send*` calls are no-ops.

---

### 2.8 Analytics Service ✅ `internal/analytics/`

Aggregates application funnel metrics and trend data for the dashboard.

**Metrics tracked:**
- Application counts by status (queued, prep, applying, applied, viewed, interview, offer, failed, rejected)
- 7-day daily activity trend (bar chart data)
- Interview conversion rate, offer rate
- Auto-apply usage metrics

**Data flow:** Analytics handler reads directly from `applications` table with GROUP BY status queries. No separate analytics DB or event stream at MVP scale.

---

### 2.9 Salary Benchmarking Service ✅ `internal/salary/`

Provides market salary comparison for job titles and locations.

**Features:**
- `GetBenchmark(title, location)` — queries aggregated salary ranges, p25/p50/p75 percentiles
- `CompareToMarket(userSalary, benchmark)` — returns `PositionAbove`, `PositionAt`, or `PositionBelow`
- Redis cache layer: benchmark hits skip DB for 24 hours
- Minimum sample size of 5 jobs before returning data (prevents single-job skew)

**Frontend:** `SalaryBenchmarkBadge` component shows inline `$90k–$120k · Above market` next to job cards.

---

## 3. The Two-Stage Application Pipeline

Splitting AI preparation (cheap) from browser execution (expensive) prevents wasted browser sessions on AI failures.

<details>
<summary><b>Click to expand — Full Sequence Diagram</b></summary>

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant API as API Gateway
    participant DB as PostgreSQL
    participant Redis as Redis (Asynq)
    participant W1 as Apply Engine (AIPrep Worker)
    participant AI as AI Service
    participant S3 as AWS S3
    participant W2 as Apply Engine (Browser Worker)
    participant BP as Browser Pool
    participant NOTIF as Notification Service

    Note over User,API: User approves a match
    User->>API: POST /api/v1/matches/{id}/approve
    API->>DB: INSERT applications (status='queued')
    API->>Redis: Enqueue TypeAIPrep {app_id, user_id}
    API-->>User: 202 Accepted

    Note over W1,S3: Stage 1 — AI Prep (cheap, retryable)
    W1->>Redis: Dequeue TypeAIPrep
    W1->>DB: UPDATE status='ai_preparing'
    W1->>DB: SELECT resume (weighted A/B selection)
    W1->>AI: POST /tailor (resume_text, job_description)
    AI-->>W1: tailored_resume_text + cover_letter
    W1->>AI: POST /qa (form_questions, user_profile)
    AI-->>W1: {question: answer} map
    W1->>S3: PutObject tailored_resume.pdf
    W1->>DB: UPDATE tailored_resume_s3, cover_letter, status='queued_browser'
    W1->>Redis: Enqueue TypeBrowserApply {app_id, s3_key, ats_type}

    Note over W2,BP: Stage 2 — Browser Apply (expensive, isolated)
    W2->>Redis: Dequeue TypeBrowserApply
    W2->>DB: UPDATE status='applying'
    W2->>S3: GetObject tailored_resume.pdf
    W2->>BP: POST /apply {ats_type, apply_url, resume, answers}
    activate BP
    BP->>BP: plugin.Match(url) → select ATS plugin
    BP->>BP: plugin.Fill(page, data) → fill all form fields
    BP->>BP: plugin.Submit(page) → click submit
    BP->>S3: PutObject screenshot_proof.png
    BP-->>W2: 200 OK {screenshot_s3_key}
    deactivate BP
    W2->>DB: UPDATE status='applied', applied_at=NOW(), screenshot_s3_key
    W2->>NOTIF: Fire EventApplicationSubmitted
    NOTIF-->>User: Slack/Discord/Email notification

    Note over W2: On any failure
    W2->>DB: UPDATE status='failed', error_message
    W2->>NOTIF: Fire EventApplicationFailed
```

</details>

### Failure Recovery

| Failure Point | Behaviour |
|:-------------|:----------|
| AI service rate-limited | Asynq exponential backoff, retries up to 5× — browser slot never allocated |
| Claude API error | `TypeAIPrep` fails, `applications.status='failed'`, user notified |
| Browser pool 504 timeout | `TypeBrowserApply` fails, status='failed', screenshot skipped |
| ATS DOM change | Browser worker catches Playwright error, stores error message, marks failed |
| Spot interruption (Fargate) | Asynq re-queues interrupted tasks automatically on worker reconnect |
| Emergency stop | `DELETE /emergency-stop` cancels queued tasks, resets status |

---

## 4. Data Models

### Core Tables

```
users
├── id (UUID PK)
├── email, password_hash, full_name
├── cognito_sub
└── created_at, updated_at

user_preferences
├── user_id (FK → users)
├── target_titles TEXT[]
├── remote_pref (remote/hybrid/onsite)
├── salary_min, salary_max, salary_currency
├── excluded_companies TEXT[]
├── auto_apply_enabled BOOL, daily_application_limit INT
├── apply_start_hour, apply_end_hour INT, apply_timezone TEXT
├── slack_webhook_url, discord_webhook_url TEXT
├── webhook_events TEXT[]
└── email_digest_enabled BOOL

resumes
├── id (UUID PK)
├── user_id (FK → users)
├── file_name, s3_key, raw_text, parsed_json
├── is_primary BOOL
├── ab_enabled BOOL, ab_weight INT
├── total_applications, total_views, total_interviews, total_offers INT
└── created_at

jobs
├── id (UUID PK)
├── external_id TEXT, source_board TEXT
├── title, company, location, description
├── salary_min, salary_max, salary_currency
├── apply_url, ats_type
├── is_scam BOOL, scam_score FLOAT
├── embedding vector(384)          ← pgvector
├── embedding_generated_at TIMESTAMP
└── created_at, updated_at

matches
├── id (UUID PK)
├── user_id (FK → users), job_id (FK → jobs)
├── status (pending/approved/rejected)
├── match_score FLOAT
├── match_breakdown JSONB
└── created_at, updated_at

applications
├── id (UUID PK)
├── user_id (FK → users), job_id (FK → jobs), match_id (FK → matches)
├── resume_id (FK → resumes)
├── status ENUM(queued/ai_preparing/queued_browser/applying/applied/failed/...)
├── tailored_resume_s3 TEXT
├── cover_letter TEXT
├── screenshot_s3_key TEXT
├── error_message TEXT
├── applied_at, outcome_updated_at TIMESTAMP
└── created_at

application_events
├── id (UUID PK)
├── application_id (FK → applications)
├── event_type TEXT     ← 'follow_up_sent', 'webhook_sent', etc.
├── payload JSONB
└── created_at

salary_benchmarks
├── id (UUID PK)
├── title_normalized TEXT, location TEXT
├── p25, p50, p75 NUMERIC
├── sample_size INT
└── updated_at
```

### DB Triggers (auto-maintained stats)

```sql
-- Increments resume.total_applications on each new application
trg_increment_resume_app_count: AFTER INSERT ON applications

-- Updates total_interviews/total_offers when application outcome changes
trg_update_resume_stats: AFTER UPDATE OF status ON applications
```

---

## 5. Implemented Features

All features below are **fully built, tested, and integrated** as of March 2026.

| Feature | Location | Notes |
|:--------|:---------|:------|
| JWT auth (dev HS256 + prod RS256) | `internal/auth/` | Automatic algorithm routing |
| User registration + profile | `internal/user/` | Cognito sub linking |
| Resume upload + management | `internal/user/repository/` | S3 upload, raw text extraction |
| Resume A/B testing | `internal/user/repository/` + migrations 007 | Weighted selection, DB triggers |
| Job scraping (4 sources) | `internal/jobdiscovery/scrapers/` | Indeed, Glassdoor, ZipRecruiter, LinkedIn |
| Scam detection | `internal/jobdiscovery/scam/` | 8-signal heuristic, `score >= 0.5` |
| Keyword + semantic matching | `internal/jobmatcher/service/` | 0.6/0.4 combined score |
| pgvector embeddings | `internal/embedding/` + migrations 006 | IVFFlat index, all-MiniLM-L6-v2 |
| 8 ATS plugins | `internal/ats/plugins/` | See §2.5 table |
| 2-stage apply pipeline | `internal/application/workers/` | AIPrep → BrowserApply |
| Auto-apply scheduler | `internal/application/service/` | Per-user timezone, Redis rate limit |
| Webhook notifications | `internal/notification/` | Slack, Discord, 4 event types |
| Follow-up email scheduler | `internal/notification/followup_scheduler.go` | 7-day threshold, dedup |
| Weekly email digest | `internal/notification/` | Sunday 9am aggregation |
| Analytics dashboard | `internal/analytics/` + `frontend/.../analytics/` | Funnel chart, 7-day trend |
| Salary benchmarking | `internal/salary/` + `frontend/.../salary/` | p25/p50/p75, market position |
| Emergency stop | `internal/application/handler/` | Cancels queued + in-progress |
| Dashboard overview | `frontend/.../dashboard/overview/` | 5 stat cards, rate limit bar |
| Test coverage | `go test ./...` + Jest | 234 frontend tests, 60+ Go tests |

---

## 6. Future Work 🗓️

### 6.1 Multi-Region Deployment
- Active-active in `us-east-1` + `eu-west-1` for GDPR compliance.
- RDS Global Database for cross-region replication.
- CloudFront geo-routing to nearest API Gateway.

### 6.2 ML-Based Scam Detection
- Replace heuristic scorer with a fine-tuned binary classifier trained on labeled job postings.
- Model served via AI Service `/api/v1/classify`.

### 6.3 Resume Quality Scorer
- LLM-based evaluation of uploaded resume before applying.
- Score displayed on resume card with actionable suggestions.

### 6.4 Interview Prep Module
- After `status=interview`, surface likely interview questions for the specific company/role.
- Uses Claude with RAG over company Glassdoor reviews.

### 6.5 Chrome Extension
- Companion extension to let users submit jobs found manually.
- One-click sends job URL to API Gateway for match + apply queueing.

### 6.6 Additional ATS Plugins 🗓️
| Plugin | Target |
|:-------|:-------|
| Jobvite | `jobs.jobvite.com/*` |
| JazzHR | `app.jazz.co/*` |
| Rippling | `app.rippling.com/job-listing/*` |
| Recruitee | `*.recruitee.com/*` |

---

## 7. Local Development

### Prerequisites
```bash
docker compose up -d postgres redis minio   # core data services
```

### Environment Variables (`.env.local`)
```bash
DATABASE_URL=postgres://autodream:secret@localhost:5432/autodream?sslmode=disable
REDIS_URL=redis://localhost:6379
AI_SERVICE_URL=http://localhost:8000
MINIO_ENDPOINT=localhost:9000
JWT_SECRET=dev-secret-32-chars-minimum-here
APP_ENV=development
```

### Service Ports
| Service | Port |
|:--------|:----:|
| API Gateway | 8080 |
| Job Discovery | 8082 |
| Job Matcher | 8083 |
| Apply Engine | 8084 |
| AI Service | 8000 |
| Browser Pool | 9222 |
| PostgreSQL | 5432 |
| Redis | 6379 |
| MinIO | 9000 |

### Running Migrations
```bash
migrate -path ./migrations -database "$DATABASE_URL" up
```

### Test Suite
```bash
# Go
go test -race -timeout 120s ./...

# Frontend
cd frontend && npm test -- --forceExit  # 234 tests across 21 suites

# E2E (requires running stack)
cd frontend && npx playwright test
```
