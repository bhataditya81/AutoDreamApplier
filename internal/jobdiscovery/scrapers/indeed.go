package scrapers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// indeedScraper scrapes job listings from Indeed.
//
// Strategy:
//   - Uses Indeed's public HTML (no JS required for search results page)
//   - Parses structured <article> job cards from the results page
//   - Follows "apply" links to detect the underlying ATS
//   - Rotates User-Agent and respects rate limits to avoid blocks
//
// Note: Indeed's HTML structure changes frequently. Selectors may need
// updating. Consider switching to ScraperAPI or Apify's Indeed actor
// if blocking becomes frequent.
type indeedScraper struct {
	client     *http.Client
	userAgents []string
	uaIndex    int
}

// NewIndeedScraper creates a new Indeed scraper.
func NewIndeedScraper() Scraper {
	return &indeedScraper{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
		},
	}
}

func (s *indeedScraper) Source() models.JobSource { return models.SourceIndeed }
func (s *indeedScraper) Name() string              { return "Indeed Scraper" }

// Search searches Indeed for jobs matching the given parameters.
// Results are emitted to the returned channel as they are found.
func (s *indeedScraper) Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error) {
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

		for page := 0; page < maxPages; page++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			searchURL := s.buildSearchURL(query, params.Location, params.Remote, page*10)

			jobs, err := s.scrapePage(ctx, searchURL)
			if err != nil {
				// Send error but don't stop — try next page
				errCh <- fmt.Errorf("page %d: %w", page+1, err)
				time.Sleep(3 * time.Second) // back-off on error
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

			// Polite delay between pages (1-3 seconds)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(1500+page*200) * time.Millisecond):
			}
		}
	}()

	return jobsCh, errCh
}

// buildSearchURL constructs an Indeed search URL.
func (s *indeedScraper) buildSearchURL(query, location string, remote bool, start int) string {
	params := url.Values{}
	params.Set("q", query)

	if remote || strings.EqualFold(location, "remote") {
		params.Set("remotejob", "032b3046-06a3-4876-8dfd-474eb5e7ed11")
		params.Set("l", "")
	} else if location != "" {
		params.Set("l", location)
	}

	params.Set("start", strconv.Itoa(start))
	params.Set("sort", "date")      // Most recent first
	params.Set("fromage", "14")     // Last 14 days

	return "https://www.indeed.com/jobs?" + params.Encode()
}

// scrapePage fetches one search results page and returns the job listings.
func (s *indeedScraper) scrapePage(ctx context.Context, pageURL string) ([]*models.ScrapedJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Rotate User-Agent
	s.uaIndex = (s.uaIndex + 1) % len(s.userAgents)
	req.Header.Set("User-Agent", s.userAgents[s.uaIndex])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by Indeed (429) — back off")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return s.parseHTML(string(body))
}

// parseHTML extracts job cards from the Indeed results HTML.
// Indeed renders job cards as <div data-jk="..."> or <article> elements.
func (s *indeedScraper) parseHTML(htmlContent string) ([]*models.ScrapedJob, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var jobs []*models.ScrapedJob
	s.walkNode(doc, &jobs)
	return jobs, nil
}

// walkNode recursively walks the HTML AST, collecting job cards.
func (s *indeedScraper) walkNode(n *html.Node, jobs *[]*models.ScrapedJob) {
	if n.Type == html.ElementNode {
		// Indeed job cards: <div class="job_seen_beacon"> or data-jk attribute
		if s.isJobCard(n) {
			if job := s.extractJobCard(n); job != nil {
				*jobs = append(*jobs, job)
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.walkNode(child, jobs)
	}
}

// isJobCard returns true if the node looks like an Indeed job card.
func (s *indeedScraper) isJobCard(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "data-jk" && attr.Val != "" {
			return true
		}
		if attr.Key == "class" && strings.Contains(attr.Val, "job_seen_beacon") {
			return true
		}
	}
	return false
}

// extractJobCard extracts a ScrapedJob from a job card node.
func (s *indeedScraper) extractJobCard(n *html.Node) *models.ScrapedJob {
	job := &models.ScrapedJob{
		Source:         models.SourceIndeed,
		SalaryCurrency: "USD",
	}

	// Extract job key (external ID)
	for _, attr := range n.Attr {
		if attr.Key == "data-jk" {
			job.ExternalID = attr.Val
			job.URL = "https://www.indeed.com/viewjob?jk=" + attr.Val
			// Start with the indeed apply URL pattern
			job.ApplyURL = "https://www.indeed.com/applystart?jk=" + attr.Val
		}
	}

	if job.ExternalID == "" {
		return nil
	}

	// Walk child nodes to extract fields
	s.extractFromChildren(n, job)

	if job.Title == "" || job.Company == "" {
		return nil // Skip incomplete cards
	}

	// Try to resolve the true ATS URL by following the redirect
	s.enrichATSType(job)

	return job
}

// extractFromChildren recursively extracts job fields from child nodes.
func (s *indeedScraper) extractFromChildren(n *html.Node, job *models.ScrapedJob) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractFieldFromNode(child, job)
		s.extractFromChildren(child, job)
	}
}

// extractFieldFromNode tries to extract a job field from a specific node.
func (s *indeedScraper) extractFieldFromNode(n *html.Node, job *models.ScrapedJob) {
	if n.Type != html.ElementNode {
		return
	}

	classes := getAttr(n, "class")
	id := getAttr(n, "id")

	switch {
	case containsAny(classes, "jobTitle", "jcs-JobTitle"):
		if job.Title == "" {
			job.Title = strings.TrimSpace(textContent(n))
		}
	case containsAny(classes, "companyName", "css-1ioi40n"):
		if job.Company == "" {
			job.Company = strings.TrimSpace(textContent(n))
		}
	case containsAny(classes, "companyLocation", "css-1m4cuuf"):
		if job.Location == "" {
			loc := strings.TrimSpace(textContent(n))
			job.Location = loc
			job.IsRemote = strings.Contains(strings.ToLower(loc), "remote")
		}
	case containsAny(classes, "salary-snippet", "estimated-salary"):
		if salary := strings.TrimSpace(textContent(n)); salary != "" {
			job.SalaryMin, job.SalaryMax = parseSalary(salary)
		}
	case strings.HasPrefix(id, "jobDescriptionText"):
		if job.Description == "" {
			job.Description = strings.TrimSpace(textContent(n))
		}
	}
}

// enrichATSType attempts to follow the Indeed apply link redirect to find the true ApplyURL
// and infers the ATSType from it.
func (s *indeedScraper) enrichATSType(job *models.ScrapedJob) {
	if job.ApplyURL == "" {
		return
	}

	// Create a custom client that doesn't strictly follow redirects but captures them.
	// Often, the first or second redirect lands on the ATS domain.
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop after 3 redirects to prevent infinite loops,
			// or stop early if we hit a known ATS.
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, job.ApplyURL, nil)
	if err != nil {
		return
	}
	s.uaIndex = (s.uaIndex + 1) % len(s.userAgents)
	req.Header.Set("User-Agent", s.userAgents[s.uaIndex])

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// The final URL after our limited redirect tracing.
	finalURL := resp.Request.URL.String()
	
	// If it's still an Indeed URL, try to parse the actual URL from Query Params
	// (Sometimes Indeed embeds the target inside `continue` or similar params)
	if strings.Contains(finalURL, "indeed.com") {
		if continueURL := resp.Request.URL.Query().Get("continue"); continueURL != "" {
			finalURL = continueURL
		}
	}

	// Update if we successfully departed the indeed domain
	if finalURL != "" && finalURL != job.ApplyURL {
		job.ApplyURL = finalURL
	}
}

// ── Helper functions ──────────────────────────────────────────────────────────

var salaryRegex = regexp.MustCompile(`\$?([\d,]+)(?:K)?`)

// parseSalary extracts min/max integers from a salary string like "$120K - $180K a year".
func parseSalary(s string) (*int, *int) {
	matches := salaryRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	parse := func(raw string) int {
		raw = strings.ReplaceAll(raw, ",", "")
		v, _ := strconv.Atoi(raw)
		// Convert K notation
		if strings.Contains(strings.ToUpper(s), "K") {
			v *= 1000
		}
		return v
	}

	min := parse(matches[0][1])
	if min == 0 {
		return nil, nil
	}
	minVal := min

	var maxVal *int
	if len(matches) > 1 {
		max := parse(matches[1][1])
		maxVal = &max
	}
	return &minVal, maxVal
}

// textContent returns the concatenated text content of a node and its children.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return sb.String()
}

// getAttr returns the value of an HTML attribute, or empty string.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
