# Job Discovery Engine (`cmd/job-discovery`)

The Job Discovery service acts as the core data ingestion engine. It continuously polls configured job board definitions and scrapes new openings into the AutoDreamApplier ecosystem.

## 1. Overview
Scraping job boards is an inherently volatile process. Websites like Indeed or Glassdoor change DOM structures and employ rate limiting. 
To shield the Matcher and the Apply Engine from failing due to bad scrapes, the Discovery Engine handles ingestion, data sanitization, ATS link resolution, and robust conflict resolution when pushing to Postgraduate.

## 2. Cron Scheduler & Orchestration
The root of the Discovery Engine is a lightweight interval-based scheduler.

1. **Scraper Initialization:** Every X hours, the scheduler boots up instances of registered scraper concrete types (e.g. `NewIndeedScraper()`).
2. **Channel-Based Data Flow:** Scrapers implement the interface:
```go
type Scraper interface {
	Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error)
	Source() models.JobSource
	Name() string
}
```
   The engine reads from the returned `jobsCh` and immediately persists jobs individually to keep memory low.

## 3. Data Sanitization & Persistance
When a ScrapedJob is received off the channel, it goes through an `Upsert` conflict process.

- **External IDs:** The ID supplied by the source board (`data-jk` on Indeed, `jobListingId` on Glassdoor) is used as the primary tracking key.
- **ON CONFLICT Rules**: If an ID already exists in the Postgres `jobs` table:
  - The `title` and `description` are updated (as jobs are frequently edited by recruiters).
  - The `is_active` flag is set to `true`.
  - The `updated_at` timestamp is refreshed.
- **Garbage Collection (Stale Jobs)**: At the end of a full scrape cycle for a specific source board, any job that *was not* seen during that scan has its `is_active` flag set to `false`, effectively removing it from Matcher consideration.

## 4. ATS Tracing & Auto-Detection
The Apply Engine requires a precise ATS to execute automation. Default wrapper links (`indeed.com/viewjob`) are unusable.

### The Redirect Tracing Heuristics:
1. **Initial Output:** Scrapers pull the raw "Apply" link.
2. **`enrichATSType()` execution:** If the Apply URL is an Indeed domain, the scraper builds a custom `http.Client`.
3. **Follow-Through:** The client sends a GET request to the wrapper link and records the `Location` header provided in `HTTP 302 Found` responses without rendering the DOM.
4. **Link Extraction:** Once the `indeed.com` domain is exited, the final URL destination is captured resulting in the true ATS domain (e.g., `boards.greenhouse.io`).
5. **Pattern Matching:** The `JobRepository` reads the true URL and maps it against a dictionary of valid Application Tracking Systems (`internal/ats/detector.go`):
```go
	case strings.Contains(url, "jobs.lever.co"), strings.Contains(url, "lever.co/"):
		return models.ATSLever
```

With the ATS precisely classified (or marked `ATSUnknown`), the database `jobs` payload is ready for the `job-matcher`.
