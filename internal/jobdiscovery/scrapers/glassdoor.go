package scrapers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// glassdoorScraper scrapes Glassdoor job listings.
//
// Glassdoor blocks direct HTTP scrapers aggressively. Strategy:
//   - Use Glassdoor's public job search HTML endpoint
//   - Conservative rate: max 5 req/min (1 page per 12 seconds)
//   - Real User-Agent rotation with modern browser strings
//   - Returns ErrSourceBlocked if 403/429 received (caller should back off)
//
// Note: Glassdoor's HTML structure changes frequently and they use heavy
// JavaScript rendering. If blocking persists, consider switching to a
// headless browser via the browser pool, or a third-party scraping service
// (e.g. ScraperAPI, Apify Glassdoor actor).
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
				if isBlockedErr(err) {
					// Glassdoor is actively blocking — stop immediately, do not retry
					errCh <- fmt.Errorf("glassdoor scraper blocked on page %d: %w", page, ErrSourceBlocked)
					return
				}
				// Non-fatal — send error and try next page
				errCh <- fmt.Errorf("page %d: %w", page, err)
				time.Sleep(5 * time.Second) // back-off on error
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

// parseHTML extracts job cards from a Glassdoor search results page.
// Glassdoor renders job cards with class "react-job-listing" or
// "JobsList_jobListItem__*" in the newer React-based layout.
func (s *glassdoorScraper) parseHTML(htmlContent string) ([]*models.ScrapedJob, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var jobs []*models.ScrapedJob
	s.walkNode(doc, &jobs)
	return jobs, nil
}

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
	return containsAny(classes,
		"react-job-listing",
		"JobsList_jobListItem",
		"jobCard",
		"job-listing",
	)
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

	// Try to find a better URL from an anchor link if we have one
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
func (s *glassdoorScraper) extractJobLink(n *html.Node, job *models.ScrapedJob) {
	if n.Type == html.ElementNode && n.Data == "a" {
		href := getAttr(n, "href")
		if href != "" && containsAny(href, "glassdoor.com/job", "/job-listing/", "/partner/") {
			if strings.HasPrefix(href, "/") {
				href = "https://www.glassdoor.com" + href
			}
			if job.URL == "" || containsAny(href, "/job-listing/", "/partner/jobListing") {
				job.URL = href
				job.ApplyURL = href
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractJobLink(child, job)
	}
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
			"css-1m4cuuf",
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
			"css-87uc0g",
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
			"css-1buaf54",
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
			"css-1xe2xww",
		):
		if salary := strings.TrimSpace(textContent(n)); salary != "" {
			job.SalaryMin, job.SalaryMax = parseSalary(salary)
		}

	// Description snippet
	case containsAny(classes, "job-description", "JobCard_jobDescriptionSnippet", "desc"):
		if job.Description == "" {
			job.Description = strings.TrimSpace(textContent(n))
		}
	}
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
