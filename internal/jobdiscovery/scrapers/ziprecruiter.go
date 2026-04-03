package scrapers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// zipRecruiterScraper scrapes job listings from ZipRecruiter.
//
// Strategy:
//   - Uses ZipRecruiter's public HTML search page
//   - Parses job cards from the results page (article elements with data-job-id)
//   - Follows apply links to detect the underlying ATS
//   - Rotates User-Agent and uses polite delays to avoid blocks
//
// Note: ZipRecruiter's HTML structure may change. If blocking becomes frequent,
// consider switching to their partner API or a third-party scraping service.
type zipRecruiterScraper struct {
	client     *http.Client
	userAgents []string
	uaIndex    int
}

// NewZipRecruiterScraper creates a new ZipRecruiter scraper.
func NewZipRecruiterScraper() Scraper {
	return &zipRecruiterScraper{
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

func (s *zipRecruiterScraper) Source() models.JobSource { return models.SourceZipRecruiter }
func (s *zipRecruiterScraper) Name() string             { return "ZipRecruiter Scraper" }

// Search searches ZipRecruiter for jobs matching the given parameters.
// Results are emitted to the returned channel as they are found.
func (s *zipRecruiterScraper) Search(ctx context.Context, params SearchParams) (<-chan *models.ScrapedJob, <-chan error) {
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
				// Send error but continue — try next page
				errCh <- fmt.Errorf("page %d: %w", page, err)
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

			// Polite delay between pages (1.5–3 seconds)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(1500+page*200) * time.Millisecond):
			}
		}
	}()

	return jobsCh, errCh
}

// buildSearchURL constructs a ZipRecruiter search URL.
// ZipRecruiter uses 1-based page numbers and the `days` param for recency.
func (s *zipRecruiterScraper) buildSearchURL(query, location string, remote bool, page int) string {
	params := url.Values{}
	params.Set("search", query)
	params.Set("days", "14") // Last 14 days

	if remote || strings.EqualFold(location, "remote") {
		params.Set("location", "")
		params.Set("refine_by_tags", "remote") // ZipRecruiter remote filter tag
	} else if location != "" {
		params.Set("location", location)
	}

	if page > 1 {
		params.Set("page", strconv.Itoa(page))
	}

	return "https://www.ziprecruiter.com/jobs-search?" + params.Encode()
}

// scrapePage fetches one search results page and returns the job listings.
func (s *zipRecruiterScraper) scrapePage(ctx context.Context, pageURL string) ([]*models.ScrapedJob, error) {
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
		return nil, fmt.Errorf("rate limited by ZipRecruiter (429) — back off")
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

// parseHTML extracts job cards from the ZipRecruiter results HTML.
// ZipRecruiter renders job cards as <article data-job-id="..."> elements.
func (s *zipRecruiterScraper) parseHTML(htmlContent string) ([]*models.ScrapedJob, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var jobs []*models.ScrapedJob
	s.walkNode(doc, &jobs)
	return jobs, nil
}

// walkNode recursively walks the HTML AST, collecting job cards.
func (s *zipRecruiterScraper) walkNode(n *html.Node, jobs *[]*models.ScrapedJob) {
	if n.Type == html.ElementNode {
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

// isJobCard returns true if the node is a ZipRecruiter job card.
// ZipRecruiter uses <article data-job-id="..."> or divs with class "job_result".
func (s *zipRecruiterScraper) isJobCard(n *html.Node) bool {
	if n.Data != "article" && n.Data != "div" {
		return false
	}
	for _, attr := range n.Attr {
		if attr.Key == "data-job-id" && attr.Val != "" {
			return true
		}
		if attr.Key == "class" && (strings.Contains(attr.Val, "job_result") ||
			strings.Contains(attr.Val, "jobList-item") ||
			strings.Contains(attr.Val, "job-listing")) {
			return true
		}
	}
	return false
}

// extractJobCard extracts a ScrapedJob from a job card node.
func (s *zipRecruiterScraper) extractJobCard(n *html.Node) *models.ScrapedJob {
	job := &models.ScrapedJob{
		Source:         models.SourceZipRecruiter,
		SalaryCurrency: "USD",
	}

	// Extract job ID and construct URLs
	for _, attr := range n.Attr {
		if attr.Key == "data-job-id" && attr.Val != "" {
			job.ExternalID = attr.Val
			job.URL = "https://www.ziprecruiter.com/jobs/" + attr.Val
			job.ApplyURL = "https://www.ziprecruiter.com/jobs/" + attr.Val
		}
	}

	// Walk child nodes to extract fields
	s.extractFromChildren(n, job)

	// Also check for apply URL embedded in an <a> tag
	s.extractApplyURL(n, job)

	if job.ExternalID == "" || job.Title == "" || job.Company == "" {
		return nil
	}

	// Try to resolve the true ATS URL by following the redirect
	s.enrichATSType(job)

	return job
}

// extractFromChildren recursively extracts job fields from child nodes.
func (s *zipRecruiterScraper) extractFromChildren(n *html.Node, job *models.ScrapedJob) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractFieldFromNode(child, job)
		s.extractFromChildren(child, job)
	}
}

// extractFieldFromNode tries to extract a job field from a specific node.
func (s *zipRecruiterScraper) extractFieldFromNode(n *html.Node, job *models.ScrapedJob) {
	if n.Type != html.ElementNode {
		return
	}

	classes := getAttr(n, "class")
	itemprop := getAttr(n, "itemprop")
	dataTest := getAttr(n, "data-testid")

	switch {
	// Title: various selectors ZipRecruiter uses
	case containsAny(classes, "job_title", "jobList-title", "t_good") ||
		itemprop == "title" ||
		dataTest == "job-title":
		if job.Title == "" {
			job.Title = strings.TrimSpace(textContent(n))
		}

	// Company name
	case containsAny(classes, "hiring_company_text", "jobList-company", "t_org_name") ||
		itemprop == "hiringOrganization" ||
		dataTest == "job-company":
		if job.Company == "" {
			job.Company = strings.TrimSpace(textContent(n))
		}

	// Location
	case containsAny(classes, "location", "jobList-location", "t_city") ||
		itemprop == "jobLocation" ||
		dataTest == "job-location":
		if job.Location == "" {
			loc := strings.TrimSpace(textContent(n))
			job.Location = loc
			job.IsRemote = strings.Contains(strings.ToLower(loc), "remote")
		}

	// Salary
	case containsAny(classes, "actual_minimum_salary", "salary", "compensation") ||
		itemprop == "baseSalary" ||
		dataTest == "job-salary":
		if salary := strings.TrimSpace(textContent(n)); salary != "" {
			job.SalaryMin, job.SalaryMax = parseSalary(salary)
		}

	// Description snippet
	case containsAny(classes, "job_description", "jobList-description", "t_job_desc"):
		if job.Description == "" {
			job.Description = strings.TrimSpace(textContent(n))
		}
	}
}

// extractApplyURL looks for the apply/job link within a job card node.
func (s *zipRecruiterScraper) extractApplyURL(n *html.Node, job *models.ScrapedJob) {
	if n.Type == html.ElementNode && n.Data == "a" {
		href := getAttr(n, "href")
		classes := getAttr(n, "class")
		if href != "" && containsAny(classes, "job_link", "jobList-title", "t_good") {
			// ZipRecruiter apply links are typically relative paths
			if strings.HasPrefix(href, "/") {
				href = "https://www.ziprecruiter.com" + href
			}
			if strings.Contains(href, "ziprecruiter.com") {
				job.ApplyURL = href
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		s.extractApplyURL(child, job)
	}
}

// enrichATSType follows the ZipRecruiter apply link redirect to find the true ApplyURL
// and infers the ATSType from it.
func (s *zipRecruiterScraper) enrichATSType(job *models.ScrapedJob) {
	if job.ApplyURL == "" {
		return
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
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

	finalURL := resp.Request.URL.String()

	// Update if we successfully departed the ZipRecruiter domain
	if finalURL != "" && finalURL != job.ApplyURL &&
		!strings.Contains(finalURL, "ziprecruiter.com") {
		job.ApplyURL = finalURL
	}
}
