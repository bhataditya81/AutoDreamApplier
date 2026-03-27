package scrapers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// glassdoorScraper scrapes Glassdoor job listings.
//
// Strategy (in priority order):
//  1. Extract the embedded __NEXT_DATA__ JSON blob that Glassdoor's Next.js
//     server-side render injects on every page. This is stable across CSS
//     class renames and does not require JS execution.
//  2. Fall back to DOM walking with known stable selectors
//     (data-test, itemprop, legacy class names) when __NEXT_DATA__ is absent.
//
// Blocking / rate-limiting:
//   - Returns ErrSourceBlocked immediately on 403/429 — caller should back off
//   - Conservative rate: max 5 req/min (1 page per 12 seconds)
//   - Real User-Agent rotation with modern browser strings
//
// Note: If Glassdoor's Next.js structure changes significantly, or if a
// Cloudflare challenge page is returned, consider routing via the browser
// pool (headless Chromium) or a third-party service (ScraperAPI, Apify).
type glassdoorScraper struct {
	client     *http.Client
	userAgents []string
	uaIndex    int
}

// NewGlassdoorScraper creates a new Glassdoor scraper.
func NewGlassdoorScraper() Scraper {
	return &glassdoorScraper{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
		},
	}
}

func (s *glassdoorScraper) Source() models.JobSource { return models.SourceGlassdoor }
func (s *glassdoorScraper) Name() string              { return "Glassdoor Scraper" }

// Search scrapes Glassdoor for jobs matching the given parameters.
// Results are emitted to the returned channel as they are found.
// If Glassdoor returns 403 or 429, both channels are closed immediately
// and ErrSourceBlocked is sent on the error channel before closing.
func (s *glassdoorScraper) Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error) {
	jobsCh := make(chan *models.ScrapedJob, 50)
	errCh := make(chan error, 1)

	go func() {
		defer close(jobsCh)
		defer close(errCh)

		maxPages := params.MaxPages
		if maxPages <= 0 {
			maxPages = 5
		}

		query := strings.Join(params.Keywords, " ")

		for page := 1; page <= maxPages; page++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			searchURL := s.buildSearchURL(query, params.Location, params.Remote, page)

			jobs, err := s.scrapePage(ctx, searchURL)
			if err != nil {
				if errors.Is(err, ErrSourceBlocked) || isBlockedErr(err) {
					// Glassdoor is actively blocking — stop immediately, do not retry
					errCh <- fmt.Errorf("glassdoor scraper blocked on page %d: %w", page, ErrSourceBlocked)
					return
				}
				// If context was cancelled, exit immediately without sleeping.
				if ctx.Err() != nil {
					return
				}
				// Non-fatal — send error and try next page with context-aware back-off.
				select {
				case errCh <- fmt.Errorf("page %d: %w", page, err):
				default: // buffer full — skip
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}

			if len(jobs) == 0 {
				// No more results
				return
			}

			for _, job := range jobs {
				select {
				case <-ctx.Done():
					return
				case jobsCh <- job:
				}
			}

			// Conservative delay: 12s between pages to stay under 5 req/min
			if page < maxPages {
				select {
				case <-ctx.Done():
					return
				case <-time.After(12 * time.Second):
				}
			}
		}
	}()

	return jobsCh, errCh
}

// buildSearchURL constructs a Glassdoor job search URL.
func (s *glassdoorScraper) buildSearchURL(query, location string, remote bool, page int) string {
	params := url.Values{}
	params.Set("sc.keyword", query)
	params.Set("locT", "N") // N = National / country-wide
	params.Set("jobType", "all")
	params.Set("fromAge", "14") // Last 14 days
	params.Set("sort.sortType", "date")
	params.Set("sort.descending", "true")

	if remote || strings.EqualFold(location, "remote") {
		params.Set("remoteWorkType", "1") // Glassdoor remote work filter
	} else if location != "" {
		// Glassdoor accepts free-text location via locKeyword
		params.Set("locT", "C")
		params.Set("locKeyword", location)
	}

	if page > 1 {
		params.Set("p", fmt.Sprintf("%d", page))
	}

	return "https://www.glassdoor.com/Job/jobs.htm?" + params.Encode()
}

// scrapePage fetches one Glassdoor search results page and returns job listings.
func (s *glassdoorScraper) scrapePage(ctx context.Context, pageURL string) ([]*models.ScrapedJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests:
		// Return a sentinel error so the caller knows this is a block, not a
		// transient network error.
		return nil, fmt.Errorf("glassdoor returned %d: %w", resp.StatusCode, ErrSourceBlocked)
	case http.StatusOK:
		// continue
	default:
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return s.parseHTML(string(body))
}

// setHeaders sets realistic browser-like headers and rotates User-Agent.
func (s *glassdoorScraper) setHeaders(req *http.Request) {
	s.uaIndex = (s.uaIndex + 1) % len(s.userAgents)
	req.Header.Set("User-Agent", s.userAgents[s.uaIndex])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", "https://www.glassdoor.com/")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// parseHTML first tries to extract job data from Glassdoor's embedded
// __NEXT_DATA__ JSON (server-side rendered by Next.js), then falls back to
// DOM walking when that tag is absent.
func (s *glassdoorScraper) parseHTML(htmlContent string) ([]*models.ScrapedJob, error) {
	// ── Strategy 1: __NEXT_DATA__ JSON extraction ─────────────────────────────
	// Glassdoor embeds all SSR data in a <script id="__NEXT_DATA__"> tag.
	// This is stable across CSS class renames and does not require JS execution.
	if jobs, err := s.extractNextData(htmlContent); err == nil && len(jobs) > 0 {
		return jobs, nil
	}

	// ── Strategy 2: DOM walking (fallback) ────────────────────────────────────
	// Used when __NEXT_DATA__ is absent (older page format, CDN-cached shell).
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var jobs []*models.ScrapedJob
	s.walkNode(doc, &jobs)
	return jobs, nil
}

// ── __NEXT_DATA__ extraction ──────────────────────────────────────────────────

// nextDataJobListing mirrors the shape of a single job entry inside the
// Glassdoor Next.js page payload. Field names are best-effort; the scraper
// is tolerant of missing fields.
type nextDataJobListing struct {
	JobListingID string `json:"jobListingId"`
	JobTitleText string `json:"jobTitleText"`
	// Employer info may be nested differently across page versions.
	EmployerNameFromSearch string `json:"employerNameFromSearch"`
	LocationName           string `json:"locationName"`
	RemoteWorkTypes        []string `json:"remoteWorkTypes"`
	PayPeriodAdjustedPay   struct {
		P10 *float64 `json:"p10"`
		P90 *float64 `json:"p90"`
	} `json:"payPeriodAdjustedPay"`
	JobDescription string `json:"jobDescription"`
	Header         struct {
		JobLink string `json:"jobLink"`
	} `json:"header"`
}

// extractNextData finds <script id="__NEXT_DATA__"> in the HTML, parses it as
// JSON, and navigates the data tree to find job listings.
//
// Glassdoor's data path (as of early 2024):
//
//	props.pageProps.jobListings[].jobview
//
// This path changes occasionally; the function tries several known paths.
func (s *glassdoorScraper) extractNextData(htmlContent string) ([]*models.ScrapedJob, error) {
	jsonBlob, err := extractScriptTagContent(htmlContent, "__NEXT_DATA__")
	if err != nil {
		return nil, err
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonBlob), &root); err != nil {
		return nil, fmt.Errorf("unmarshal __NEXT_DATA__: %w", err)
	}

	// Navigate: root["props"]["pageProps"]
	propsRaw, ok := root["props"]
	if !ok {
		return nil, errors.New("__NEXT_DATA__: no props key")
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(propsRaw, &props); err != nil {
		return nil, fmt.Errorf("unmarshal props: %w", err)
	}

	pagePropsRaw, ok := props["pageProps"]
	if !ok {
		return nil, errors.New("__NEXT_DATA__: no pageProps key")
	}
	var pageProps map[string]json.RawMessage
	if err := json.Unmarshal(pagePropsRaw, &pageProps); err != nil {
		return nil, fmt.Errorf("unmarshal pageProps: %w", err)
	}

	// Try the known paths for job listings data.
	return s.parseJobListingsFromPageProps(pageProps)
}

// parseJobListingsFromPageProps tries multiple known data paths within pageProps.
func (s *glassdoorScraper) parseJobListingsFromPageProps(pageProps map[string]json.RawMessage) ([]*models.ScrapedJob, error) {
	// Path 1: pageProps.jobListings (most common as of 2024)
	if raw, ok := pageProps["jobListings"]; ok {
		if jobs := s.parseJobListingsArray(raw); len(jobs) > 0 {
			return jobs, nil
		}
	}

	// Path 2: pageProps.initialData.jobListings
	if initialDataRaw, ok := pageProps["initialData"]; ok {
		var initialData map[string]json.RawMessage
		if err := json.Unmarshal(initialDataRaw, &initialData); err == nil {
			if raw, ok := initialData["jobListings"]; ok {
				if jobs := s.parseJobListingsArray(raw); len(jobs) > 0 {
					return jobs, nil
				}
			}
		}
	}

	// Path 3: pageProps.jobSearchResult.jobListings
	if resultRaw, ok := pageProps["jobSearchResult"]; ok {
		var result map[string]json.RawMessage
		if err := json.Unmarshal(resultRaw, &result); err == nil {
			if raw, ok := result["jobListings"]; ok {
				if jobs := s.parseJobListingsArray(raw); len(jobs) > 0 {
					return jobs, nil
				}
			}
		}
	}

	return nil, errors.New("__NEXT_DATA__: no jobListings found in any known path")
}

// parseJobListingsArray parses a JSON array of job listing objects.
// Each element may be a jobview wrapper or the listing directly.
func (s *glassdoorScraper) parseJobListingsArray(raw json.RawMessage) []*models.ScrapedJob {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil
	}

	var jobs []*models.ScrapedJob
	for _, item := range rawItems {
		// Try direct unmarshal as a listing
		var listing nextDataJobListing
		if err := json.Unmarshal(item, &listing); err == nil && listing.JobListingID != "" {
			if job := s.nextDataListingToJob(listing); job != nil {
				jobs = append(jobs, job)
				continue
			}
		}

		// Try wrapper: {"jobview": {...}} or {"jobListing": {...}}
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(item, &wrapper); err != nil {
			continue
		}
		for _, key := range []string{"jobview", "jobListing", "listing"} {
			if inner, ok := wrapper[key]; ok {
				var innerListing nextDataJobListing
				if err := json.Unmarshal(inner, &innerListing); err == nil && innerListing.JobListingID != "" {
					if job := s.nextDataListingToJob(innerListing); job != nil {
						jobs = append(jobs, job)
					}
					break
				}
			}
		}
	}
	return jobs
}

// nextDataListingToJob converts a parsed Next.js job listing to a ScrapedJob.
func (s *glassdoorScraper) nextDataListingToJob(l nextDataJobListing) *models.ScrapedJob {
	if l.JobListingID == "" || l.JobTitleText == "" {
		return nil
	}

	job := &models.ScrapedJob{
		Source:         models.SourceGlassdoor,
		SalaryCurrency: "USD",
		ExternalID:     "glassdoor_" + l.JobListingID,
		Title:          strings.TrimSpace(l.JobTitleText),
		Company:        strings.TrimSpace(l.EmployerNameFromSearch),
		Location:       strings.TrimSpace(l.LocationName),
	}

	// URL construction
	if l.Header.JobLink != "" {
		job.URL = "https://www.glassdoor.com" + l.Header.JobLink
	} else {
		job.URL = "https://www.glassdoor.com/job-listing/x-jv?jl=" + l.JobListingID
	}
	job.ApplyURL = job.URL

	// Remote detection
	for _, rwt := range l.RemoteWorkTypes {
		if strings.EqualFold(rwt, "remote") || strings.EqualFold(rwt, "fully remote") {
			job.IsRemote = true
			break
		}
	}
	if !job.IsRemote {
		job.IsRemote = strings.Contains(strings.ToLower(job.Location), "remote")
	}

	// Salary
	if l.PayPeriodAdjustedPay.P10 != nil {
		v := int(*l.PayPeriodAdjustedPay.P10)
		job.SalaryMin = &v
	}
	if l.PayPeriodAdjustedPay.P90 != nil {
		v := int(*l.PayPeriodAdjustedPay.P90)
		job.SalaryMax = &v
	}

	if l.JobDescription != "" {
		// Strip basic HTML tags from description snippet
		job.Description = stripHTML(l.JobDescription)
	}

	if job.Company == "" {
		return nil // must have company
	}

	return job
}

// ── DOM walking fallback ──────────────────────────────────────────────────────

// walkNode recursively walks the HTML AST collecting Glassdoor job cards.
func (s *glassdoorScraper) walkNode(n *html.Node, jobs *[]*models.ScrapedJob) {
	if n.Type == html.ElementNode && s.isJobCard(n) {
		if job := s.extractJobCard(n); job != nil {
			*jobs = append(*jobs, job)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.walkNode(child, jobs)
	}
}

// isJobCard returns true if the node looks like a Glassdoor job card.
// Supports both the legacy "jl" list-item format and the newer React layouts.
func (s *glassdoorScraper) isJobCard(n *html.Node) bool {
	classes := getAttr(n, "class")
	dataTestID := getAttr(n, "data-test")
	dataJobID := getAttr(n, "data-id")

	if dataJobID != "" {
		return true
	}
	if dataTestID == "jobListing" {
		return true
	}

	// Legacy format: <li class="jl" data-brand-employer-id="...">
	// The job ID must be extracted from the anchor href (?jl=NNN).
	if n.Data == "li" && containsAny(classes, "jl") {
		// Only treat as job card if it contains a jobLink anchor with ?jl=
		return s.hasJobLink(n)
	}

	return containsAny(classes,
		"react-job-listing",
		"JobsList_jobListItem",
		"jobCard",
		"job-listing",
	)
}

// hasJobLink returns true if the node subtree contains an anchor with a ?jl= param.
func (s *glassdoorScraper) hasJobLink(n *html.Node) bool {
	if n.Type == html.ElementNode && n.Data == "a" {
		href := getAttr(n, "href")
		if strings.Contains(href, "jl=") || strings.Contains(href, "/job-listing/") {
			return true
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if s.hasJobLink(child) {
			return true
		}
	}
	return false
}

// extractJobCard extracts a ScrapedJob from a Glassdoor job card node.
func (s *glassdoorScraper) extractJobCard(n *html.Node) *models.ScrapedJob {
	job := &models.ScrapedJob{
		Source:         models.SourceGlassdoor,
		SalaryCurrency: "USD",
	}

	// Try to get job ID from data-id or data-job-listing-id attributes
	for _, attr := range n.Attr {
		switch attr.Key {
		case "data-id", "data-job-listing-id", "data-jobid":
			if attr.Val != "" {
				job.ExternalID = "glassdoor_" + attr.Val
				job.URL = "https://www.glassdoor.com/job-listing/x-jv?jl=" + attr.Val
				job.ApplyURL = job.URL
			}
		}
	}

	// Extract text fields from child nodes
	s.extractFromChildren(n, job)

	// Try to find a better URL from an anchor link (also extracts job ID for legacy cards)
	s.extractJobLink(n, job)

	// ExternalID is required; fall back to the URL if needed
	if job.ExternalID == "" {
		return nil
	}
	if job.Title == "" || job.Company == "" {
		return nil
	}

	return job
}

// extractJobLink finds the primary <a> tag within the job card and extracts
// the canonical URL and (if possible) the job ID from it.
// For legacy cards (<li class="jl">) the job ID lives in the href ?jl= param.
func (s *glassdoorScraper) extractJobLink(n *html.Node, job *models.ScrapedJob) {
	if n.Type == html.ElementNode && n.Data == "a" {
		href := getAttr(n, "href")
		if href != "" && (containsAny(href, "glassdoor.com/job", "/job-listing/", "/partner/", "jl=")) {
			if strings.HasPrefix(href, "/") {
				href = "https://www.glassdoor.com" + href
			}
			if job.URL == "" || containsAny(href, "/job-listing/", "/partner/jobListing") {
				job.URL = href
				job.ApplyURL = href
			}

			// Extract job ID from ?jl= query parameter (legacy cards)
			if job.ExternalID == "" {
				if id := extractJLParam(href); id != "" {
					job.ExternalID = "glassdoor_" + id
				}
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractJobLink(child, job)
	}
}

// jlParamRe matches the jl= query parameter in Glassdoor job URLs.
var jlParamRe = regexp.MustCompile(`[?&]jl=(\d+)`)

// extractJLParam extracts the job listing ID from a Glassdoor URL's jl= param.
func extractJLParam(href string) string {
	m := jlParamRe.FindStringSubmatch(href)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// extractFromChildren recursively extracts job fields from child nodes.
func (s *glassdoorScraper) extractFromChildren(n *html.Node, job *models.ScrapedJob) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractFieldFromNode(child, job)
		s.extractFromChildren(child, job)
	}
}

// extractFieldFromNode tries to extract a job field from a specific node.
func (s *glassdoorScraper) extractFieldFromNode(n *html.Node, job *models.ScrapedJob) {
	if n.Type != html.ElementNode {
		return
	}

	classes := getAttr(n, "class")
	dataTest := getAttr(n, "data-test")
	itemprop := getAttr(n, "itemprop")

	switch {
	// Job title
	case dataTest == "job-title" ||
		itemprop == "title" ||
		containsAny(classes,
			"job-title",
			"JobCard_jobTitle",
			"jobLink",
			"jobInfoItem jobTitle", // legacy: "jobLink jobInfoItem jobTitle"
		):
		if job.Title == "" {
			job.Title = strings.TrimSpace(textContent(n))
		}

	// Company name
	case dataTest == "employer-name" ||
		itemprop == "name" ||
		containsAny(classes,
			"employer-name",
			"JobCard_employerName",
			"EmployerProfile_employerName",
			"jobEmpolyerName", // legacy typo in Glassdoor HTML
			"employerName",
		):
		if job.Company == "" {
			job.Company = strings.TrimSpace(textContent(n))
		}

	// Location
	case dataTest == "emp-location" ||
		itemprop == "addressLocality" ||
		containsAny(classes,
			"location",
			"JobCard_location",
			"jobLocation",  // legacy: <span class="jobLocation">
		):
		if job.Location == "" {
			loc := strings.TrimSpace(textContent(n))
			if loc != "" {
				job.Location = loc
				job.IsRemote = strings.Contains(strings.ToLower(loc), "remote")
			}
		}

	// Salary
	case dataTest == "detailSalary" ||
		containsAny(classes,
			"salary",
			"JobCard_salaryEstimate",
			"salaryText", // legacy
		):
		if salary := strings.TrimSpace(textContent(n)); salary != "" {
			job.SalaryMin, job.SalaryMax = parseSalary(salary)
		}

	// Description snippet
	case containsAny(classes,
		"job-description",
		"JobCard_jobDescriptionSnippet",
		"jobDescriptionSnippet", // legacy
		"desc",
	):
		if job.Description == "" {
			job.Description = strings.TrimSpace(textContent(n))
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractScriptTagContent finds a <script id="<id>"> tag and returns its text content.
func extractScriptTagContent(htmlContent, id string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("parse HTML for script tag: %w", err)
	}

	var content string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if content != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" && getAttr(n, "id") == id {
			// Collect text children
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					content += child.Data
				}
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	if content == "" {
		return "", fmt.Errorf("script tag with id=%q not found", id)
	}
	return content, nil
}

// stripHTML removes HTML tags from a string, collapsing whitespace.
var stripHTMLRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = stripHTMLRe.ReplaceAllString(s, " ")
	// Collapse multiple spaces/newlines
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// isBlockedErr returns true if the error indicates the source blocked us.
func isBlockedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "403") ||
		strings.Contains(err.Error(), "429") ||
		strings.Contains(err.Error(), ErrSourceBlocked.Error())
}
