#!/usr/bin/env python3
# AutoDreamApplier - Lambda + DB Cost Analysis
# Data collected: 2026-03-27

PRICE_PER_REQ = 0.20 / 1_000_000
PRICE_PER_GB_SEC = 0.0000166667

functions = {
    "api-gateway": {
        "memory_mb": 256,
        "timeout_s": 30,
        "invocations_24h": 476,
        "errors_24h": 0,
        "avg_duration_ms": 281.3,
        "max_duration_ms": 6831.4,
        "throttles": 0,
        "max_concurrent": 4,
        "schedule": "On-demand (API Gateway HTTP proxy)",
        "trigger": "API Gateway HTTP - $default catch-all route",
        "cold_starts_sample": 5,
        "sample_inv": 18,
        "billed_durations_ms": [1539, 171, 275, 152, 2, 3, 25, 62, 6, 728, 789, 3, 2, 2, 2, 6, 12, 504]
    },
    "job-discovery": {
        "memory_mb": 256,
        "timeout_s": 840,
        "invocations_24h": 8,
        "errors_24h": 0,
        "avg_duration_ms": 10451.2,
        "max_duration_ms": 13298.6,
        "throttles": 0,
        "max_concurrent": 1,
        "schedule": "rate(2 hours)",
        "trigger": "EventBridge rule autodreamApplier-job-discovery",
        "cold_starts_sample": 2,
        "sample_inv": 2,
        "billed_durations_ms": [10474, 10201]
    },
    "job-matcher": {
        "memory_mb": 256,
        "timeout_s": 300,
        "invocations_24h": 7,
        "errors_24h": 0,
        "avg_duration_ms": 1587.9,
        "max_duration_ms": 4430.5,
        "throttles": 0,
        "max_concurrent": 1,
        "schedule": "cron(30 */2 * * ? *)",
        "trigger": "EventBridge rule autodreamApplier-job-matcher",
        "cold_starts_sample": 2,
        "sample_inv": 2,
        "billed_durations_ms": [4431, 579]
    },
    "followup-scheduler": {
        "memory_mb": 128,
        "timeout_s": 120,
        "invocations_24h": 13,
        "errors_24h": 0,
        "avg_duration_ms": 1015.3,
        "max_duration_ms": 3800.8,
        "throttles": 0,
        "max_concurrent": 1,
        "schedule": "rate(1 hour)",
        "trigger": "EventBridge rule autodreamApplier-followup-scheduler",
        "cold_starts_sample": 3,
        "sample_inv": 3,
        "billed_durations_ms": [1927, 761, 588]
    }
}

monthly_factor = 30
free_tier_req = 1_000_000
free_tier_gb_sec = 400_000

print("=" * 72)
print("AUTODREAMAPPLIER - AWS LAMBDA & DATABASE USAGE REPORT")
print("Account: 346992621600  Region: us-east-1")
print("Period:  2026-03-26 15:12Z to 2026-03-27 15:12Z (24 hours)")
print("=" * 72)

print("\n=== 1. EVENTBRIDGE SCHEDULE EXPRESSIONS ===\n")
print(f"  {'Rule Name':<42} {'Schedule Expression':<30} {'Theoretical/24h'}")
print("-" * 90)
eb_data = [
    ("autodreamApplier-job-discovery",       "rate(2 hours)",          "12", "ENABLED"),
    ("autodreamApplier-job-matcher",         "cron(30 */2 * * ? *)",   "12", "ENABLED"),
    ("autodreamApplier-followup-scheduler",  "rate(1 hour)",           "24", "ENABLED"),
]
for name, expr, theo, state in eb_data:
    print(f"  {name:<42} {expr:<30} {theo}/24h ({state})")
print("  autodreamApplier-api-gateway: No EventBridge rule - triggered by API Gateway HTTP")

print("\n=== 2. LAMBDA INVOCATIONS - LAST 24 HOURS ===\n")
print(f"  {'Function':<35} {'Inv/24h':>8} {'Errors':>7} {'Throttles':>10} {'Avg ms':>8} {'Max ms':>8} {'MaxConc':>8} {'ColdStarts*':>12}")
print("-" * 102)
total_inv = 0
for name, d in functions.items():
    total_inv += d['invocations_24h']
    cs_pct = d['cold_starts_sample'] / d['sample_inv'] * 100 if d['sample_inv'] > 0 else 0
    print(f"  autodreamApplier-{name:<18} {d['invocations_24h']:>8.0f} {d['errors_24h']:>7.0f} {d['throttles']:>10.0f} {d['avg_duration_ms']:>8.1f} {d['max_duration_ms']:>8.1f} {d['max_concurrent']:>8} {cs_pct:>10.0f}%")
print("-" * 102)
print(f"  {'TOTAL':<35} {total_inv:>8.0f}")
print("  * Cold start % from log sample (not complete invocation set)")

print("\n=== 3. HOURLY BREAKDOWN - api-gateway (last 24h) ===\n")
hourly_apigw = [
    ("2026-03-26 16:xx UTC", 5,   "Small test/dev burst"),
    ("2026-03-26 17:xx UTC", 31,  "Development session"),
    ("2026-03-26 18:xx UTC", 233, "Large burst: testing / load test / onboarding"),
    ("2026-03-26 19:xx - 03:xx", 0, "Idle (8+ hour gap)"),
    ("2026-03-27 04:xx UTC", 1,   "Health check (curl/8.17.0 user-agent)"),
    ("2026-03-27 06:xx UTC", 1,   "Health check"),
    ("2026-03-27 08:xx UTC", 1,   "Health check (+ 1 extra req in period)"),
    ("2026-03-27 09:xx UTC", 198, "Auth testing: 106 x 4xx malformed-token requests"),
    ("2026-03-27 10:xx UTC", 1,   "Health check"),
    ("2026-03-27 14:xx UTC", 5,   "Recent session (20 API GW hits, 18 x 4xx)"),
]
print(f"  {'Hour (UTC)':<30} {'Invoc':>6}  Note")
print("-" * 75)
for ts, inv, note in hourly_apigw:
    print(f"  {ts:<30} {inv:>6}  {note}")

print("\n  API Gateway HTTP API request count (partial window, ~13.5h): 222")
print("  API Gateway 4xx total: 125 (56% of recorded GW requests)")
print("  API Gateway 5xx total: 0")
print("  API Gateway avg latency: 290.4ms  max: 7,272ms")
print("  Dominant 4xx cause: malformed JWT tokens (token contains invalid number of segments)")

print("\n=== 4. DATABASE CONNECTION & QUERY ANALYSIS ===\n")
print("  Database: NeonDB PostgreSQL (external, not AWS RDS)")
print("  Host:     ep-noisy-wave-am6vdr32-pooler.c-5.us-east-1.aws.neon.tech")
print("  Pool:     PgBouncer pooler, max_conns=25 per Lambda instance")
print("  Users in DB: 6 registered accounts")
print()
print("  Connection pattern (from logs):")
print("  - Every Lambda invocation opens a fresh TCP connection to NeonDB pooler")
print("  - NeonDB pooler multiplexes these to actual Postgres instances")
print("  - No persistent connection possible in Lambda (stateless execution)")
print()
print("  DB queries per function invocation (observed from logs):")
db_queries = [
    ("api-gateway",        "~1-5",  "Auth JWT validation + profile read per request"),
    ("job-discovery",      "~3",    "Connect + keyword config read + job upserts (0 new today)"),
    ("job-matcher",        "~8",    "User list (1) + prefs per user (6) + job fetch (1)"),
    ("followup-scheduler", "~8",    "User list (1) + pending follow-up query per user (6) + 1"),
]
print(f"  {'Function':<25} {'Queries/inv':>12}  Breakdown")
print("-" * 75)
for fn, q, desc in db_queries:
    print(f"  {fn:<25} {q:>12}  {desc}")

print()
print("  Estimated DB queries per 24h:")
db_24h = [
    ("api-gateway",        476, 3,  1428),
    ("job-discovery",      8,   3,  24),
    ("job-matcher",        7,   8,  56),
    ("followup-scheduler", 13,  8,  104),
]
total_queries = 0
for fn, inv, qpi, total in db_24h:
    total_queries += total
    print(f"    {fn:<25}: {inv} invoc x {qpi} queries = {total} queries")
print(f"    {'TOTAL':<25}:                      {total_queries} queries/24h (~{total_queries//24}/hour avg)")

print()
print("  NeonDB auto-suspend behavior:")
print("  - NeonDB suspends compute after ~5min of inactivity")
print("  - followup-scheduler fires every hour -> keeps compute alive during active hours")
print("  - job-discovery/matcher fire every 2h -> may trigger auto-suspend wakeup between runs")
print("  - Wakeup penalty: ~500ms to 1500ms cold-start (visible in Init Duration in logs)")

print("\n=== 5. COST ESTIMATION ===\n")

print("  5a. AWS Lambda")
print()

monthly_data = []
for name, d in functions.items():
    inv_mo = d['invocations_24h'] * monthly_factor
    avg_billed = sum(d['billed_durations_ms']) / len(d['billed_durations_ms'])
    gb = d['memory_mb'] / 1024
    gb_sec_inv = gb * (avg_billed / 1000)
    gb_sec_mo = gb_sec_inv * inv_mo
    monthly_data.append((name, inv_mo, avg_billed, gb_sec_mo))

total_inv_mo = sum(x[1] for x in monthly_data)
total_gb_sec_mo = sum(x[3] for x in monthly_data)

print(f"  {'Function':<35} {'Inv/mo':>10} {'Avg billed ms':>14} {'GB-sec/mo':>12}")
print("-" * 78)
for name, inv_mo, avg_billed, gb_sec_mo in monthly_data:
    print(f"  autodreamApplier-{name:<18} {inv_mo:>10,d} {avg_billed:>14.1f} {gb_sec_mo:>12.2f}")
print("-" * 78)
print(f"  {'TOTAL':<35} {total_inv_mo:>10,d} {'':>14} {total_gb_sec_mo:>12.2f}")
print()
print(f"  AWS Lambda Free Tier: 1,000,000 requests/month + 400,000 GB-seconds/month")
print(f"  Monthly invocations: {total_inv_mo:,}  (free tier: {free_tier_req:,})")
print(f"  Monthly GB-seconds:  {total_gb_sec_mo:.1f}  (free tier: {free_tier_gb_sec:,})")

billable_req = max(0, total_inv_mo - free_tier_req)
billable_gb = max(0, total_gb_sec_mo - free_tier_gb_sec)
lambda_cost = billable_req * PRICE_PER_REQ + billable_gb * PRICE_PER_GB_SEC

print()
print(f"  Billable requests:   {billable_req:,}")
print(f"  Billable GB-seconds: {billable_gb:.1f}")
print(f"  Lambda monthly cost: ${lambda_cost:.4f}")
print()

print("  5b. API Gateway HTTP API")
apigw_24h = 222
apigw_mo = apigw_24h * monthly_factor
apigw_cost = apigw_mo / 1_000_000 * 1.00
print(f"  Requests/month: {apigw_mo:,} (at $1.00/million)")
print(f"  Free tier:      300 million requests/month for first 12 months")
print(f"  Monthly cost:   ${apigw_cost:.4f} (well within free tier -> $0.00)")
print()

print("  5c. NeonDB")
neon_compute_hours = (
    (8 * 30 * 10.5) +    # job-discovery: 240 runs x 10.5s
    (7 * 30 * 1.6) +     # job-matcher: 210 runs x 1.6s
    (13 * 30 * 1.0) +    # followup: 390 runs x 1.0s
    (476 * 30 * 0.3)     # api-gateway: 14280 invoc x 0.3s (shared CU with pooler)
) / 3600
print(f"  Compute hours estimate/month: {neon_compute_hours:.1f} CU-hours")
print(f"  NeonDB Free Tier: 191.9 CU-hours/month")
print(f"  Usage vs free tier: {neon_compute_hours/191.9*100:.1f}%")
print(f"  NeonDB cost: $0.00/month (within free tier)")
print()

print("  5d. CloudWatch Logs")
print("  - 5 log groups, all 0 KB stored (3-day retention)")
print("  - Ingestion: ~50 log lines/invocation x 504 invocations = ~25,200 lines/day")
print("  - At ~200 bytes/line: ~5 MB/day = ~150 MB/month")
print("  - CW free tier: 5 GB ingest/month -> cost $0.00")
print()

print("  TOTAL MONTHLY COST ESTIMATE: < $0.01 (essentially $0.00)")
print("  All services are within free tier at current MVP scale.")

print("\n=== 6. JUSTIFIABILITY ANALYSIS ===\n")

print("  6a. autodreamApplier-api-gateway")
print("  Verdict: JUSTIFIED but has anomalous traffic patterns")
print()
print("  Observations:")
print("  - Normal baseline: ~5-10 requests/hour (health checks + occasional user activity)")
print("  - Burst at 18:xx UTC (233 invocations): likely development load testing or onboarding")
print("    walkthrough. No 4xx spike in that window -> legitimate requests.")
print("  - Burst at 09:xx UTC (198 invocations, 106 x 4xx): malformed JWT tokens.")
print("    Error message: 'token contains an invalid number of segments'")
print("    This suggests a client sending a raw cookie/session string instead of a Bearer token,")
print("    or a misconfigured frontend auth header. Needs investigation.")
print("  - 4xx rate: 125/222 API GW requests = 56% error rate during recorded window")
print("  - No 5xx errors, no Lambda errors, no throttles -> backend is healthy")
print("  - Cold start rate: 5/18 sampled = 28% (expected for infrequent Lambda)")
print("  - Max concurrent: 4 (well within account limit of 10)")
print("  - Duration healthy: avg 281ms, max 6.8s (p99 likely around 1-2s)")
print()
print("  Recommendation:")
print("  - Investigate the malformed token 4xx burst (09:xx). Add rate limiting (WAF or")
print("    API Gateway usage plan) to prevent token-probing attacks.")
print("  - Consider Lambda function URL instead of API Gateway to reduce latency/cost if")
print("    custom domain is not needed yet.")
print()

print("  6b. autodreamApplier-job-discovery")
print("  Verdict: OVER-SCHEDULED — 100% failure rate, wasting compute")
print()
print("  Observations:")
print("  - Fires every 2 hours = 12 invocations/day = 360/month")
print("  - EVERY invocation fails to scrape: 403 (Indeed, ZipRecruiter) + blocked (Glassdoor)")
print("  - total_found: 0, total_new: 0 across all observed runs")
print("  - Each run: 10-13 seconds of compute, 256MB memory, just to retry blocked scrapers")
print("  - 3 scrapers are all blocked: Indeed (403), ZipRecruiter (403), Glassdoor (403/429)")
print("  - Monthly wasted compute: ~360 runs x 10.5s x 0.25 GB = ~945 GB-seconds")
print()
print("  Recommendation:")
print("  - IMMEDIATE: Disable or pause job-discovery rule until scraper strategy is fixed.")
print("    Use: aws events disable-rule --name autodreamApplier-job-discovery")
print("  - Implement proper scraper rotation: residential proxies, browser automation,")
print("    or switch to official job board APIs (Indeed Publisher API, Glassdoor API)")
print("  - Consider rate-limiting to 1x/day or manual trigger until proxy solution is in place")
print("  - Add circuit breaker: if all scrapers fail N consecutive times, disable self")
print()

print("  6c. autodreamApplier-job-matcher")
print("  Verdict: OVER-SCHEDULED relative to zero data, but acceptable schedule")
print()
print("  Observations:")
print("  - Fires every 2h offset by :30 (smart: interleaved with job-discovery)")
print("  - All 6 users have no preferences set -> 0 matches scored every run")
print("  - Duration: avg 1.6s (mostly NeonDB wakeup + user list query)")
print("  - schedule is correct for production; current uselessness is due to no job data")
print("    (job-discovery blocked) and no user preferences configured")
print("  - No errors, clean execution")
print()
print("  Recommendation:")
print("  - Keep schedule but add dependency check: if job_listings table is empty or")
print("    all users have no preferences, exit early with a log line instead of querying each user")
print("  - This would reduce the 8 DB queries to 1-2 per run")
print()

print("  6d. autodreamApplier-followup-scheduler")
print("  Verdict: JUSTIFIED schedule, but currently idle work")
print()
print("  Observations:")
print("  - Rate: 1 per hour = 24/day = 720/month (highest frequency scheduled function)")
print("  - Every run: queries all 6 users for pending follow-ups, finds 0 each time")
print("  - Duration: avg 1.0s (mostly NeonDB connection + 6 follow-up queries)")
print("  - Cold starts in 3/3 sampled runs = function always cold (not warm enough)")
print("  - This is expected: 1-hour interval is too long for Lambda to stay warm")
print("  - At 128MB, cheap compute; monthly GB-seconds: ~0.12 GB x 1.0s x 720 = ~86 GB-sec")
print()
print("  Recommendation:")
print("  - Hourly follow-up checking is the right cadence for a job-application SaaS")
print("  - Add early-exit: if 0 users have any applied applications, skip follow-up queries")
print("  - Consider moving user list to a cached/config source to avoid DB query every run")
print()

print("=== 7. SUMMARY TABLE ===\n")
print(f"  {'Function':<35} {'Inv/24h':>8} {'Inv/mo*':>8} {'$/mo':>8} {'Status':<12} {'Verdict'}")
print("-" * 100)
verdicts = [
    ("api-gateway",        476, 14280, 0.00, "Active",    "Justified - monitor 4xx burst"),
    ("job-discovery",      8,   240,   0.00, "FAILING",   "OVER-SCHEDULED - all scrapers blocked 403"),
    ("job-matcher",        7,   210,   0.00, "Idle",      "OK - no data to match yet"),
    ("followup-scheduler", 13,  390,   0.00, "Active",    "Justified - no follow-ups due yet"),
]
for fn, inv, inv_mo, cost, status, verdict in verdicts:
    print(f"  autodreamApplier-{fn:<18} {inv:>8} {inv_mo:>8} ${cost:>6.2f}  {status:<12} {verdict}")
print()
print("  * Monthly estimate = 24h actuals x 30")
print()
print("  Total Lambda:    $0.00/month (within free tier)")
print("  Total NeonDB:    $0.00/month (within free tier)")
print("  Total API GW:    $0.00/month (within free tier)")
print("  TOTAL AWS BILL:  ~$0.00/month at current scale")
print()
print("  Scale-to-paid threshold:")
print("  - Lambda: ~2,000 active users with 10 req/day each = 600K invoc/month (still free)")
print("  - Lambda becomes paid at: >1M requests or >400K GB-sec per month")
print("    => roughly 3,500+ active users before Lambda charges apply")
print("  - NeonDB becomes paid at: >0.5 GB storage or >191.9 CU-hours/month")
print("    => hit CU limit at ~17 Lambda invocations running concurrently (not an issue)")
print()
print("=== END OF REPORT ===")
