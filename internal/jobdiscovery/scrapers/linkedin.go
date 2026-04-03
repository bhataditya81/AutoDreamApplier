package scrapers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// ErrSourceBlocked is returned when a job board actively blocks the scraper (403/429).
var ErrSourceBlocked = errors.New("source blocked scraper (403/429)")

// linkedInScraper scrapes LinkedIn Easy Apply job listings.
//
// LinkedIn is heavily bot-protected. Strategy:
//   - Uses LinkedIn public job search (no auth required for listing page)
//   - Only "Easy Apply" jobs (f_LF=f_AL filter)
//   - Conservative rate limiting: 1 req/5s, max 3 pages per search
//   - Rotates User-Agent strings
//   - Optional residential proxy via LINKEDIN_PROXY_URL env var
//
// Note: LinkedIn frequently updates their HTML. DOM selectors may need
// updating. Consider switching to Playwright (browser pool) if blocking
// becomes frequent.
type linkedInScraper struct {
	client     *http.Client
	userAgents []string
	proxyURL   string
}

// NewLinkedInScraper creates a new LinkedIn Easy Apply scraper.
// proxyURL is optional — pass "" to use a direct connection.
// If proxyURL is non-empty it must be a valid http/https/socks5 URL
// (e.g. "http://user:pass@host:port").
func NewLinkedInScraper(proxyURL string) Scraper {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}

	return &linkedInScraper{
		client: &http.Client{
			Timeout:   20 * time.Second,
			Transport: transport,
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
		},
		proxyURL: proxyURL,
	}
}

func (s *linkedInScraper) Source() models.JobSource { return models.SourceLinkedIn }
func (s *linkedInScraper) Name() string             { return "LinkedIn (Easy Apply)" }

// Search scrapes LinkedIn Easy Apply job listings matching the given parameters.
// Results are emitted to the returned channel as they are found.
// LinkedIn is capped at 3 pages (25 results/page) to stay below their blocking
// threshold — that is ~75 listings per search invocation.
func (s *linkedInScraper) Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error) {
	jobsCh := make(chan *models.ScrapedJob, 50)
	errCh := make(chan error, 1)

	go func() {
		defer close(jobsCh)
		defer close(errCh)

		// LinkedIn blocks aggressively beyond 3 pages; hard-cap regardless
		// of what the caller requested.
		maxPages := params.MaxPages
		if maxPages <= 0 || maxPages > 3 {
			maxPages = 3
		}

		query := strings.Join(params.Keywords, " ")

		for page := 0; page < maxPages; page++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			searchURL := s.buildSearchURL(query, params.Location, params.Remote, page*25)

			jobs, err := s.scrapePage(ctx, searchURL)
			if err != nil {
				if errors.Is(err, ErrSourceBlocked) {
					// LinkedIn is blocking us — abort immediately
					errCh <- fmt.Errorf("linkedin blocked on page %d: %w", page+1, err)
					return
				}
				// Non-fatal — log and try next page
				errCh <- fmt.Errorf("page %d: %w", page+1, err)
				time.Sleep(5 * time.Second)
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

			// Conservative delay between pages: 2–5 seconds (random jitter)
			if page < maxPages-1 {
				delay := 2000 + rand.Intn(3000) //nolint:gosec // not crypto
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(delay) * time.Millisecond):
				}
			}
		}
	}()

	return jobsCh, errCh
}

// buildSearchURL constructs a LinkedIn job search URL.
// f_LF=f_AL filters for Easy Apply jobs only.
func (s *linkedInScraper) buildSearchURL(query, location string, remote bool, start int) string {
	params := url.Values{}
	params.Set("keywords", query)
	params.Set("f_LF", "f_AL") // Easy Apply filter
	params.Set("sortBy", "DD") // Date descending (most recent)
	params.Set("start", fmt.Sprintf("%d", start))

	if remote || strings.EqualFold(location, "remote") {
		params.Set("f_WT", "2") // LinkedIn remote work type
	} else if location != "" {
		params.Set("location", location)
	}

	return "https://www.linkedin.com/jobs/search/?" + params.Encode()
}

// scrapePage fetches one LinkedIn search results page and returns job listings.
func (s *linkedInScraper) scrapePage(ctx context.Context, pageURL string) ([]*models.ScrapedJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
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
		return nil, ErrSourceBlocked
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

// setHeaders sets realistic browser headers on the request and rotates User-Agent.
func (s *linkedInScraper) setHeaders(req *http.Request) {
	ua := s.userAgents[rand.Intn(len(s.userAgents))] //nolint:gosec
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", "https://www.linkedin.com/")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// parseHTML extracts job listings from a LinkedIn search results page.
// LinkedIn renders job cards as:
//   - <div class="job-search-card"> (public/logged-out view)
//   - <li class="jobs-search-results__list-item"> (authenticated view)
func (s *linkedInScraper) parseHTML(htmlContent string) ([]*models.ScrapedJob, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var jobs []*models.ScrapedJob
	s.walkNode(doc, &jobs)
	return jobs, nil
}

// walkNode recursively walks the HTML AST collecting LinkedIn job cards.
func (s *linkedInScraper) walkNode(n *html.Node, jobs *[]*models.ScrapedJob) {
	if n.Type == html.ElementNode && s.isJobCard(n) {
		if job := s.extractJobCard(n); job != nil {
			*jobs = append(*jobs, job)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.walkNode(child, jobs)
	}
}

// isJobCard returns true if the node looks like a LinkedIn job card.
func (s *linkedInScraper) isJobCard(n *html.Node) bool {
	classes := getAttr(n, "class")
	return containsAny(classes,
		"job-search-card",
		"jobs-search-results__list-item",
		"base-card",
		"job-card-container",
	)
}

// extractJobCard extracts a ScrapedJob from a LinkedIn job card node.
func (s *linkedInScraper) extractJobCard(n *html.Node) *models.ScrapedJob {
	job := &models.ScrapedJob{
		Source:         models.SourceLinkedIn,
		SalaryCurrency: "USD",
	}

	// Try to find the job ID and URL from an anchor tag within the card
	s.extractJobLink(n, job)

	if job.ExternalID == "" {
		// Fallback: look for data-entity-urn or data-job-id attributes
		for _, attr := range n.Attr {
			switch attr.Key {
			case "data-entity-urn":
				// e.g. "urn:li:jobPosting:1234567890"
				parts := strings.Split(attr.Val, ":")
				if len(parts) > 0 {
					id := parts[len(parts)-1]
					job.ExternalID = "linkedin_" + id
					if job.URL == "" {
						job.URL = "https://www.linkedin.com/jobs/view/" + id
						job.ApplyURL = job.URL
					}
				}
			case "data-job-id":
				if attr.Val != "" {
					job.ExternalID = "linkedin_" + attr.Val
					if job.URL == "" {
						job.URL = "https://www.linkedin.com/jobs/view/" + attr.Val
						job.ApplyURL = job.URL
					}
				}
			}
		}
	}

	if job.ExternalID == "" {
		return nil
	}

	// Extract text fields from child nodes
	s.extractFromChildren(n, job)

	if job.Title == "" || job.Company == "" {
		return nil
	}

	return job
}

// extractJobLink finds the primary <a> anchor within a job card and extracts
// the job ID and canonical URL from it.
func (s *linkedInScraper) extractJobLink(n *html.Node, job *models.ScrapedJob) {
	if n.Type == html.ElementNode && n.Data == "a" {
		href := getAttr(n, "href")
		classes := getAttr(n, "class")
		if href != "" && containsAny(href, "/jobs/view/", "linkedin.com/jobs/") {
			// Normalize URL (strip query params for the canonical URL)
			if idx := strings.Index(href, "?"); idx != -1 {
				href = href[:idx]
			}
			// Make absolute if relative
			if strings.HasPrefix(href, "/") {
				href = "https://www.linkedin.com" + href
			}

			if job.URL == "" {
				job.URL = href
				job.ApplyURL = href
			}

			// Extract job ID from URL path: /jobs/view/1234567890
			if parts := strings.Split(href, "/jobs/view/"); len(parts) == 2 {
				id := strings.TrimSuffix(parts[1], "/")
				if id != "" && job.ExternalID == "" {
					job.ExternalID = "linkedin_" + id
				}
			}
		}

		// Check aria-label for title if not yet set (often on the link itself)
		if ariaLabel := getAttr(n, "aria-label"); ariaLabel != "" && job.Title == "" &&
			containsAny(classes, "base-card__full-link", "job-card-list__title") {
			job.Title = strings.TrimSpace(ariaLabel)
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractJobLink(child, job)
	}
}

// extractFromChildren recursively extracts job fields from child nodes.
func (s *linkedInScraper) extractFromChildren(n *html.Node, job *models.ScrapedJob) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractFieldFromNode(child, job)
		s.extractFromChildren(child, job)
	}
}

// extractFieldFromNode tries to extract a job field from a specific node.
func (s *linkedInScraper) extractFieldFromNode(n *html.Node, job *models.ScrapedJob) {
	if n.Type != html.ElementNode {
		return
	}

	classes := getAttr(n, "class")

	switch {
	// Job title
	case containsAny(classes,
		"base-search-card__title",
		"job-card-list__title",
		"job-card-container__link",
		"jobs-unified-top-card__job-title",
	):
		if job.Title == "" {
			job.Title = strings.TrimSpace(textContent(n))
		}

	// Company name
	case containsAny(classes,
		"base-search-card__subtitle",
		"job-card-container__company-name",
		"jobs-unified-top-card__company-name",
		"hidden-nested-link",
	):
		if job.Company == "" {
			job.Company = strings.TrimSpace(textContent(n))
		}

	// Location
	case containsAny(classes,
		"job-search-card__location",
		"job-card-container__metadata-item",
		"jobs-unified-top-card__bullet",
	):
		if job.Location == "" {
			loc := strings.TrimSpace(textContent(n))
			if loc != "" {
				job.Location = loc
				job.IsRemote = strings.Contains(strings.ToLower(loc), "remote")
			}
		}

	// Salary / compensation (LinkedIn sometimes shows it in metadata)
	case containsAny(classes, "compensation", "salary", "job-card-container__salary-info"):
		if salary := strings.TrimSpace(textContent(n)); salary != "" {
			job.SalaryMin, job.SalaryMax = parseSalary(salary)
		}

	// Description snippet
	case containsAny(classes, "job-search-card__snippet", "jobs-description__content"):
		if job.Description == "" {
			job.Description = strings.TrimSpace(textContent(n))
		}
	}
}
