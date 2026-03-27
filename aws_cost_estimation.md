# AWS Cost Estimation & Optimization Plan

AutoDreamApplier runs a Go multi-service backend, Python AI service, Playwright browser pool, PostgreSQL + pgvector, Redis, and a Next.js frontend. This document breaks down **five deployment tiers** from a full managed-service production setup (~$180/mo) down to an aggressively optimized lean cloud deployment (~$8/mo), with honest tradeoff analysis for each.

---

## Executive Summary

| Tier | Architecture | Est. Monthly Cost | Code Changes Required |
|:-----|:-------------|:-----------------:|:---------------------:|
| **1 — Baseline** | All ECS Fargate + RDS + ElastiCache | ~$180 | None |
| **2 — Optimized Fargate** | Fargate Spot + ARM64 + No NAT | ~$66 | None |
| **3 — Hybrid Serverless** | Lambda APIs + Fargate Spot Browser | ~$32 | Low (Lambda adapter) |
| **4 — Ultra-Lean** ⭐ | Lambda + EC2 nano + Neon + Upstash | ~$8–10 | Medium |
| **5 — Extreme Rewrite** | DynamoDB + Event-Driven EC2 Spot | ~$3 | Very High |
| **Year 1 Free Tier** | Tier 4 + AWS free tier | ~$0–2 | Medium |

> **Recommendation:** Start with **Tier 4** (Ultra-Lean). It costs $8–10/month post-free-tier, requires no architectural compromises, keeps pgvector and Asynq intact, and scales cleanly to Tier 2 as revenue grows.

---

## Tier 1 — Baseline: All Managed Services (~$180/mo)

Full production setup with every service on managed AWS infrastructure. Zero operational friction, maximum observability.

| Component | AWS Service | Spec | Est. Cost |
|:----------|:------------|:-----|----------:|
| Frontend (Next.js SSR) | AWS Amplify Hosting | Managed CI/CD + CDN | ~$5 |
| API Gateway (Go/chi) | ECS Fargate | 0.25 vCPU / 0.5 GB × 2 | ~$18 |
| Job Discovery (Go) | ECS Fargate | 0.25 vCPU / 0.5 GB × 1 | ~$9 |
| Job Matcher (Go) | ECS Fargate | 0.25 vCPU / 0.5 GB × 1 | ~$9 |
| Apply Engine (Go) | ECS Fargate | 0.25 vCPU / 0.5 GB × 1 | ~$9 |
| AI Service (Python) | ECS Fargate | 0.5 vCPU / 2 GB × 1 | ~$18 |
| Browser Pool (Go/Playwright) | ECS Fargate | 1 vCPU / 2 GB × 2 | ~$50 |
| PostgreSQL + pgvector | RDS `db.t4g.micro` (Single-AZ) | 2 vCPU / 1 GB | ~$12 |
| Redis (Asynq queue + cache) | ElastiCache `cache.t4g.micro` | 0.5 GB | ~$12 |
| Resumes + screenshots | Amazon S3 Standard | ~20 GB | ~$1 |
| Load balancer | ALB (1 listener) | — | ~$17 |
| Private subnet egress | NAT Gateway (1 AZ) | — | ~$32 |
| Logs + metrics | CloudWatch (5-day retention) | — | ~$8 |
| **Total** | | | **~$200/mo** |

> **Notes:**
> - Browser pool is the single largest compute line item — Playwright's Chromium requires ~512 MB per concurrent session.
> - NAT Gateway ($32/mo) is the most wasteful line item for a small startup with minimal egress volume.
> - LLM costs (Anthropic Claude API / Bedrock) and SES email are usage-based and excluded from the fixed floor.

---

## Tier 2 — Optimized Fargate (~$66/mo)

Apply four zero-code-change optimizations to the baseline to cut costs by 63%.

### Optimizations Applied

**1. Fargate Spot for async workers (saves ~$55/mo)**
The `browser-pool`, `ai-service`, and `apply-engine` never serve synchronous HTTP traffic — they consume from Asynq/Redis queues. Asynq automatically retries interrupted tasks, making Spot interruptions harmless.
- Switch these three services to **ECS Fargate Spot** capacity providers.
- Savings: **~70% off** on their compute.

**2. ARM64 (Graviton) everywhere (saves ~$10/mo)**
- Build all Docker images for `linux/arm64`.
- Deploy ECS tasks, RDS, and ElastiCache on Graviton (`t4g`) families.
- Savings: **~20% better price-performance** across all compute and database.

**3. Eliminate NAT Gateway (saves $32/mo)**
- Run ECS tasks in **public subnets** with public IPs enabled.
- Harden Security Groups: inbound only from ALB security group, no inbound 0.0.0.0/0.
- Scrapers and browser pool can still reach the internet outbound (to job boards and ATS portals) without a NAT hop.
- Savings: **$32/month immediately**.

**4. Scale-to-zero browser pool (saves ~$35/mo at low traffic)**
- Deploy Application Auto Scaling on the browser-pool ECS service.
- Publish a CloudWatch custom metric from the Asynq queue depth (inspectable via Redis `LLEN`).
- Scale to 0 tasks when queue depth = 0, scale up on demand.
- At MVP traffic levels, browser pool is idle ~70% of the time.

| Category | Optimized Setup | Est. Cost |
|:---------|:----------------|----------:|
| All Go services (ARM64, Fargate) | Monolith or 2 tasks | ~$12 |
| AI Service (Fargate Spot, ARM64) | 0.5 vCPU / 2 GB | ~$5 |
| Browser Pool (Fargate Spot, scale-to-zero) | 1 vCPU / 2 GB × 0–2 | ~$8 |
| RDS PostgreSQL `db.t4g.micro` (ARM64) | Single-AZ | ~$12 |
| ElastiCache `cache.t4g.micro` (ARM64) | 0.5 GB | ~$12 |
| ALB (no NAT) | 1 listener | ~$17 |
| S3 + CloudWatch + Amplify | — | ~$8 |
| **Optimized Total** | | **~$74/mo** |

> **What breaks at scale:** Single-AZ RDS has no automatic failover. ElastiCache single node has no replica. ALB without WAF is exposed to scraper abuse. Acceptable for MVP; upgrade to Multi-AZ and WAF at $5k MRR.

---

## Tier 3 — Hybrid Serverless (~$32/mo)

Move stateless Go services to Lambda; keep the browser pool on Fargate Spot (Lambda's 15-minute timeout cannot handle long ATS form sessions reliably). Replace Redis with SQS.

### Architecture

```
Users
  └── Vercel (free) ──── Next.js frontend

Internet ──── API Gateway HTTP API ──── Lambda (api-gateway Go, arm64, 256 MB)
                                              │
                                    ┌─────────┴──────────┐
                                    │                     │
                               RDS PostgreSQL        SQS Queue
                               db.t4g.micro          (replaces Asynq/Redis)
                                    │
                        EventBridge (scheduler)
                                    │
                            Lambda (job-discovery)
                            Lambda (job-matcher)
                                    │
                              SQS → Lambda (apply-engine coordinator)
                                    │
                              ECS Fargate Spot (browser-pool)
                              (booted on-demand, terminates after job)
```

### Key Changes vs Tier 2

| What | Before | After | Effort |
|:-----|:-------|:-------|:------:|
| HTTP server | ECS Fargate | Lambda + `awslambdagoapi` chi adapter | 1 day |
| Queue | Redis + Asynq | SQS Standard | 2 days |
| Scheduling | Ticker goroutine | EventBridge rules | 1 day |
| Browser pool | Always-on Fargate | On-demand Fargate Spot | None |
| Frontend | Amplify | Vercel (free) | 1 hour |

| Category | Serverless Setup | Est. Cost |
|:---------|:----------------|----------:|
| Lambda (api, discovery, matcher) | ARM64, 128–256 MB | ~$0.50 |
| API Gateway HTTP API | per-request, no idle | ~$0.30 |
| SQS | 1M req/mo (always free) | $0 |
| RDS `db.t4g.micro` (Single-AZ) | Largest fixed cost | ~$12 |
| Browser Pool (Fargate Spot, on-demand) | Avg 30 min/day running | ~$8 |
| S3 + EventBridge + SES | — | ~$2 |
| Vercel frontend | Free tier | $0 |
| CloudWatch (3-day log retention) | — | ~$3 |
| **Hybrid Serverless Total** | | **~$26/mo** |

> **Critical Blocker: Lambda + Playwright.** Chrome binaries exceed Lambda's 50 MB unzipped limit for standard runtimes. Lambda container images (up to 10 GB) work, but cold starts take 15–30 seconds for a Chromium container and are unreliable for ATS sessions. This is why the browser pool stays on Fargate even in this tier.

---

## Tier 4 — Ultra-Lean ⭐ Recommended ($8–10/mo)

Replace managed AWS databases with **free-tier external services** that maintain full compatibility — zero code rewrite for the data layer.

### The Core Insight

| Managed AWS Service | Cost | Replacement | Cost | Compatibility |
|:--------------------|:----:|:------------|:----:|:-------------:|
| RDS PostgreSQL | ~$12/mo | **Neon.tech free tier** | $0 | 100% (same wire protocol, pgvector supported) |
| ElastiCache Redis | ~$12/mo | **Upstash Redis free tier** | $0 | 100% (Redis-compatible, Asynq works natively) |
| Amplify / ALB | ~$17/mo | **Vercel free tier** | $0 | 100% (Next.js SSR supported) |

These three swaps save **$41/month** with zero changes to Go or frontend code.

### Architecture

```
                        ┌─────────────────────────────────────┐
                        │         EXTERNAL FREE SERVICES       │
                        │                                      │
                        │  Neon.tech PostgreSQL (free tier)    │
                        │  ├── pgvector extension ✓            │
                        │  └── auto-suspend when idle          │
                        │                                      │
                        │  Upstash Redis (free 10k cmd/day)    │
                        │  └── Asynq compatible ✓              │
                        │                                      │
                        │  Vercel (free tier)                  │
                        │  └── Next.js 14 SSR ✓                │
                        └─────────────────────────────────────┘
                                         │
                                         │ DATABASE_URL / REDIS_URL
                                         │ (just env vars, no code change)
                                         ▼
Users ──── Vercel ──── API Gateway HTTP API ──── Lambda (api-gateway, arm64 256MB)
                                                       │
                                              ┌────────┴────────┐
                                              │                  │
                                       Upstash Redis        Neon PostgreSQL
                                       (Asynq queue)        (pgvector, all tables)
                                              │
                         ┌────────────────────┼────────────────────┐
                         │                    │                    │
                   EventBridge          EventBridge           Lambda
                   every 2hr            every 1hr           (job-matcher)
                         │                    │
                   Lambda               Lambda
                 (job-discovery)    (followup-scheduler)
                                              │
                                        Upstash Redis
                                    (Asynq TypeBrowserApply task)
                                              │
                                    EC2 t4g.nano ($3.07/mo)
                                    ├── apply-engine worker
                                    ├── browser-pool (Playwright/Chromium)
                                    └── Asynq server (QueueBrowserApply)
                                              │
                                   ┌──────────┴──────────┐
                                   │                     │
                                 ATS                    S3
                                Portals              (resumes, screenshots)

                        Cognito (auth, 50k MAU free)
                        SES     (email, free from EC2)
                        Bedrock (embeddings, ~$0.006/mo)
```

### Why EC2 t4g.nano for Browser Pool (not Lambda)

| Factor | Lambda | EC2 t4g.nano |
|:-------|:-------|:------------|
| Chromium package size | Exceeds 250 MB limit (needs container) | No limit |
| Max execution time | 15 min hard cap | Unlimited |
| ATS session timeout risk | High (complex forms > 15 min possible) | Zero |
| Chromium cold start | ~20s (container image) | Pre-loaded, ~2s |
| Cost | ~$0 but unreliable | $3.07/month, reliable |
| Memory for Chromium | 512 MB–1 GB needed | 512 MB available |

> **t4g.nano memory note:** At 512 MB, nano is tight. Run a single Chromium session at a time. If you need concurrent sessions, upgrade to `t4g.micro` ($6.14/mo, 1 GB RAM) — still well under $10/month total.

### Neon.tech Considerations

- **Free tier includes:** 0.5 GB storage, 1 compute unit (0.25 vCPU, 1 GB RAM), pgvector ✓, branching, auto-suspend
- **Auto-suspend:** Compute suspends after 5 min inactivity → ~1–2s cold start on first query. Acceptable for async workers, API Gateway Lambda handles reconnection automatically via `pgx` connection pool.
- **Connection strings:** Standard `postgres://user:pass@host/db?sslmode=require` — point `DATABASE_URL` at Neon, zero code change.
- **At scale:** Upgrade to Neon Pro ($19/mo) for 10 GB storage and no suspend — still cheaper than RDS.

### Upstash Redis Considerations

- **Free tier:** 10,000 commands/day (300k/month)
- **Asynq usage estimate at 50 jobs/day:** ~25 Redis commands per job cycle = 1,250 commands/day. Headroom: 8,750 commands/day for caching.
- **Connection:** Standard Redis URL (`redis://default:pass@host:port`) — Asynq connects identically.
- **At scale:** Upgrade to Upstash Pay-as-you-go ($0.20 per 100k commands) — ~$2/month at 1M commands.

### Code Changes Required (Medium Effort — ~3 days)

| Change | Effort | Impact |
|:-------|:------:|:-------|
| Add `awslambdagoapi` chi adapter to `cmd/api-gateway/main.go` | 2 hours | API runs as Lambda |
| Refactor FollowUpScheduler goroutine → Lambda handler (EventBridge trigger) | 3 hours | Removes long-running goroutine from API |
| Convert `cmd/job-discovery` → Lambda handler (EventBridge every 2hr) | 4 hours | Saves Fargate cost |
| Convert `cmd/job-matcher` → Lambda handler (EventBridge or SQS trigger) | 3 hours | Saves Fargate cost |
| Point `DATABASE_URL` to Neon, `REDIS_URL` to Upstash | 30 min | DB/cache swap |
| Keep `cmd/apply-engine` as-is on EC2 t4g.nano | 0 | Browser pool intact |
| Migrate frontend to Vercel | 30 min | Free hosting |

### Monthly Cost Breakdown (Post Free Tier)

| Service | Spec | Cost |
|:--------|:-----|-----:|
| EC2 t4g.nano (browser pool + apply-engine) | ARM64, 512 MB, on-demand | $3.07 |
| Lambda (api-gateway, discovery, matcher) | arm64, 128–256 MB, ~1.5M req/mo | $0.30 |
| API Gateway HTTP API | $1.00/1M requests | $0.15 |
| Neon.tech PostgreSQL | Free tier | $0 |
| Upstash Redis | Free tier (10k cmd/day) | $0 |
| S3 (resumes + screenshots) | ~20 GB Standard | $0.50 |
| Bedrock Titan Embeddings V2 | ~300k tokens/mo | $0.006 |
| SES (email notifications) | From EC2, 62k/mo free | $0 |
| EventBridge (schedulers) | 14M events/mo free tier | $0 |
| CloudWatch Logs (3-day retention) | ~3 GB/mo ingested | $1.50 |
| Cognito | 50k MAU free tier | $0 |
| Vercel (Next.js frontend) | Free hobby tier | $0 |
| **Total** | | **~$5.50–8/mo** |

> **With t4g.micro** (1 GB RAM, safer for concurrent Chromium): **~$8.50–10/mo**

---

## Tier 5 — Extreme Rewrite (~$3/mo)

Push cost to near-zero by eliminating every managed database. Requires significant architectural rewriting — not recommended unless DynamoDB + vector DB migration is already planned for other reasons.

### Required Rewrites

**A. Replace PostgreSQL → DynamoDB + Pinecone**
- Rewrite all 6 repository files (`application_repository.go`, `user_repository.go`, etc.) to use AWS SDK DynamoDB client.
- Replace pgvector semantic search with **Pinecone free tier** (1 index, 1M vectors).
- DynamoDB free tier: 25 GB, 25 WCU, 25 RCU — permanently free.
- **Effort:** 2–3 weeks. Every SQL JOIN, ORDER BY, and LIMIT must be redesigned for key-value access patterns.

**B. Event-Driven EC2 Spot for browser (no always-on compute)**
- SQS triggers a tiny Lambda, which calls `ec2:RunInstances` to boot a fresh Spot instance.
- Instance runs User Data script: pulls browser-pool container, processes application, terminates self.
- Adds 60–90 second cold start before browser begins (invisible to users since pipeline is async).
- EC2 Spot `t4g.small` ($0.005/hr): 30 applications/month × 10 min each = 5 hrs = **$0.025/month**.

**C. Static Next.js export → S3 + CloudFront**
- Loses SSR. All API calls must go to Lambda backend.
- Cost: ~$0.50/month.

### Extreme Tier Cost

| Category | Extreme Setup | Cost |
|:---------|:-------------|-----:|
| Lambda (all Go services) | arm64, 128 MB | ~$0.30 |
| API Gateway HTTP API | — | ~$0.10 |
| DynamoDB (On-Demand) | Free tier | $0 |
| Pinecone free tier | 1 index, 1M vectors | $0 |
| Event-Driven EC2 Spot (browser) | ~5 hrs/month at $0.005/hr | ~$0.03 |
| S3 + CloudFront | Static frontend + assets | ~$0.60 |
| SQS + EventBridge | Free tier | $0 |
| CloudWatch (minimal) | — | ~$1 |
| **Total** | | **~$2–3/mo** |

> **Honest assessment:** The $3/month saving over Tier 4 costs 3+ weeks of engineering time and permanent complexity. The DynamoDB data model for complex joins (applications → jobs → users) is non-trivial. Only pursue this if building DynamoDB-native from scratch.

---

## Year 1 Free Tier Analysis

AWS Free Tier (first 12 months) + external free tiers eliminate nearly all cost during the MVP phase.

| Service | Free Tier Allowance | AutoDreamApplier Usage | Cost Y1 |
|:--------|:--------------------|:----------------------|--------:|
| EC2 t2.micro / t3.micro | 750 hrs/month (1 instance) | Browser pool + apply-engine | $0 |
| Lambda | 1M requests + 400k GB-seconds/month (always free) | All Go Lambda services | $0 |
| API Gateway HTTP API | 1M requests/month (12 months) | ~500k req/month MVP | $0 |
| S3 | 5 GB + 20k GETs + 2k PUTs (12 months) | Resume + screenshot storage | $0 |
| CloudWatch | 10 metrics, 10 alarms, 1M API calls (always free) | Basic monitoring | $0 |
| Cognito | 50,000 MAU (always free) | Auth | $0 |
| SES | 62,000 emails/month from EC2 | Notifications | $0 |
| SQS | 1M requests/month (always free) | Optional queuing | $0 |
| EventBridge | 14M events/month (always free) | Schedulers | $0 |
| Bedrock (Titan Embeddings) | 1M tokens free (promo) | ~300k tokens/month | $0 |
| **Neon PostgreSQL** | Free tier (no expiry) | All DB operations | $0 |
| **Upstash Redis** | 10k commands/day free (no expiry) | Asynq queue | $0 |
| **Vercel** | Free hobby tier (no expiry) | Next.js frontend | $0 |
| **Total Year 1** | | | **~$0–2/mo** |

> Year 1 costs are essentially zero. The only possible charges are CloudWatch logs beyond 5 GB/month ingested ($0.50/GB), or S3 egress beyond 15 GB/month.

---

## Hidden Cost Watch-Outs

These costs are frequently missed in initial estimates:

| Hidden Cost | When It Triggers | Mitigation |
|:------------|:-----------------|:-----------|
| **Data transfer out** | EC2 → internet: $0.09/GB after 100 GB/month | Stay under 100 GB; screenshots are small |
| **RDS Proxy** | If Lambda concurrency → DB connection exhaustion | Neon handles connection pooling natively (PgBouncer built-in) |
| **NAT Gateway data processing** | $0.045/GB through NAT | Eliminate NAT entirely (Tier 2+) |
| **CloudWatch log retention** | $0.03/GB/month stored | Set retention to 3 days for dev, 7 days for prod |
| **Lambda provisioned concurrency** | Needed to avoid cold starts | Not needed for Tier 4 — cold starts <200ms for Go |
| **Bedrock per-token** | Claude API calls for cover letters | Claude is already paid via Anthropic API; avoid double-billing via Bedrock |
| **Neon compute beyond free tier** | Lots of concurrent connections | Keep connection pool ≤5 connections from Lambda |
| **Upstash beyond 10k/day** | High Asynq task volume | At >200 applications/day, upgrade to pay-as-you-go ($0.20/100k commands) |
| **EC2 Spot interruption** | AWS reclaims spot instance | Use on-demand t4g.nano for browser pool — spot is only worth it at Fargate scale |

---

## Migration Path: MVP → Growth → Scale

```
Today (MVP, 0–50 users)                    Growth (50–500 users)              Scale (500+ users)
─────────────────────────                  ──────────────────────             ──────────────────
Tier 4 Ultra-Lean                          Tier 3 Hybrid Serverless           Tier 2 Optimized Fargate
                                           + Neon Pro ($19/mo)                + RDS Multi-AZ
Neon free + Upstash free ──────────────►   Neon Pro + Upstash PAYG ────────►  RDS db.t4g.small
EC2 t4g.nano ($3/mo)                       EC2 t4g.micro ($6/mo)              ECS Fargate Spot
Lambda (all stateless APIs)                Lambda (APIs) + Fargate (workers)  ECS (all services)
~$8/month                                  ~$45/month                         ~$120/month
```

**Trigger to upgrade Neon → RDS:** When Neon free storage (0.5 GB) is full — typically at ~500 active users with job history.

**Trigger to upgrade Upstash → ElastiCache:** When commands exceed 10k/day consistently — typically at ~200 auto-applies per day.

**Trigger to upgrade EC2 nano → Fargate:** When browser pool needs concurrent sessions (>3 users applying simultaneously).

---

## ARM64 Dockerfile Requirement

All Go services must build for `linux/arm64` to run on Graviton EC2/Fargate:

```dockerfile
# Multi-arch build (already the case if using Docker Buildx)
FROM --platform=linux/arm64 golang:1.22-alpine AS builder
RUN GOARCH=arm64 GOOS=linux go build -o /app ./cmd/api-gateway

FROM --platform=linux/arm64 gcr.io/distroless/static
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

For CI/CD (GitHub Actions):
```yaml
- uses: docker/setup-buildx-action@v3
- run: docker buildx build --platform linux/arm64 -t autodream/api-gateway:latest .
```
