package scrapers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// loadFixture reads a file from the testdata directory relative to this package.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to load fixture %q: %v", name, err)
	}
	return string(data)
}

// drainZR reads all jobs and errors from Search() channels with a timeout.
func drainZR(jobsCh <-chan *models.ScrapedJob, errCh <-chan error) ([]*models.ScrapedJob, []error) {
	var jobs []*models.ScrapedJob
	var errs []error

	timeout := time.After(5 * time.Second)
	jobsDone, errsDone := false, false

	for !jobsDone || !errsDone {
		select {
		case j, ok := <-jobsCh:
			if !ok {
				jobsDone = true
			} else if j != nil {
				jobs = append(jobs, j)
			}
		case e, ok := <-errCh:
			if !ok {
				errsDone = true
			} else if e != nil {
				errs = append(errs, e)
			}
		case <-timeout:
			return jobs, errs
		}
	}
	return jobs, errs
}

// newTestZipScraper returns a zipRecruiterScraper configured for testing.
func newTestZipScraper() *zipRecruiterScraper {
	return &zipRecruiterScraper{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		userAgents: []string{"TestAgent/1.0"},
	}
}

// zipJobIDs returns a slice of ExternalIDs for display in test failures.
func zipJobIDs(jobs []*models.ScrapedJob) []string {
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ExternalID
	}
	return ids
}

// mustParseHTMLNode parses an HTML string and fatals on error.
func mustParseHTMLNode(t *testing.T, htmlStr string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}
	return doc
}

// findFirstHTMLElement does a depth-first search and returns the first element
// whose tag name matches the given name.
func findFirstHTMLElement(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstHTMLElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

// ── unit tests for parseHTML ──────────────────────────────────────────────────

func TestZipRecruiterScraper_ParseHTML_ParsesJobs(t *testing.T) {
	fixture := loadFixture(t, "ziprecruiter_search.html")
	s := newTestZipScraper()

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job, got 0")
	}
}

func TestZipRecruiterScraper_ParseHTML_JobFields(t *testing.T) {
	fixture := loadFixture(t, "ziprecruiter_search.html")
	s := newTestZipScraper()

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed from fixture")
	}

	job := jobs[0]

	if job.ExternalID == "" {
		t.Error("job.ExternalID must not be empty")
	}
	if job.Title == "" {
		t.Error("job.Title must not be empty")
	}
	if job.Company == "" {
		t.Error("job.Company must not be empty")
	}
	if job.URL == "" {
		t.Error("job.URL must not be empty")
	}
	if job.Source != models.SourceZipRecruiter {
		t.Errorf("job.Source = %q, want %q", job.Source, models.SourceZipRecruiter)
	}
}

func TestZipRecruiterScraper_ParseHTML_ExternalIDFormat(t *testing.T) {
	// Fixture job card 1 has data-job-id="zr_abc123".
	fixture := loadFixture(t, "ziprecruiter_search.html")
	s := newTestZipScraper()

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed")
	}

	found := false
	for _, job := range jobs {
		if job.ExternalID == "zr_abc123" {
			found = true
			expectedURL := "https://www.ziprecruiter.com/jobs/zr_abc123"
			if job.URL != expectedURL {
				t.Errorf("job.URL = %q, want %q", job.URL, expectedURL)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected a job with ExternalID %q, got IDs: %v", "zr_abc123", zipJobIDs(jobs))
	}
}

func TestZipRecruiterScraper_ParseHTML_RemoteFlag(t *testing.T) {
	// Fixture job card 2 (zr_def456) has location "Remote" — IsRemote should be true.
	fixture := loadFixture(t, "ziprecruiter_search.html")
	s := newTestZipScraper()

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	found := false
	for _, job := range jobs {
		if job.ExternalID == "zr_def456" {
			found = true
			if !job.IsRemote {
				t.Errorf("expected IsRemote=true for job with location %q", job.Location)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected job with ExternalID %q, got: %v", "zr_def456", zipJobIDs(jobs))
	}
}

func TestZipRecruiterScraper_ParseHTML_EmptyPage(t *testing.T) {
	s := newTestZipScraper()

	jobs, err := s.parseHTML(`<html><body><div id="no-results">No jobs found</div></body></html>`)
	if err != nil {
		t.Fatalf("parseHTML on empty page returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs on empty page, got %d", len(jobs))
	}
}

func TestZipRecruiterScraper_ParseHTML_InvalidHTML(t *testing.T) {
	// golang.org/x/net/html is lenient — malformed HTML should not error.
	s := newTestZipScraper()

	jobs, err := s.parseHTML(`<<<<<not valid html>`)
	if err != nil {
		t.Fatalf("parseHTML on invalid HTML returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs from garbage HTML, got %d", len(jobs))
	}
}

// ── isJobCard ─────────────────────────────────────────────────────────────────

func TestZipRecruiterScraper_IsJobCard_ArticleWithDataJobID(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<article data-job-id="123">title</article>`)
	article := findFirstHTMLElement(doc, "article")
	if article == nil {
		t.Fatal("could not find article node in test HTML")
	}
	if !s.isJobCard(article) {
		t.Error("expected isJobCard=true for article with data-job-id")
	}
}

func TestZipRecruiterScraper_IsJobCard_DivWithJobResultClass(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<div class="job_result other-class">title</div>`)
	div := findFirstHTMLElement(doc, "div")
	if div == nil {
		t.Fatal("could not find div node in test HTML")
	}
	if !s.isJobCard(div) {
		t.Error("expected isJobCard=true for div with job_result class")
	}
}

func TestZipRecruiterScraper_IsJobCard_DivWithJobListItemClass(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<div class="jobList-item">title</div>`)
	div := findFirstHTMLElement(doc, "div")
	if div == nil {
		t.Fatal("could not find div node")
	}
	if !s.isJobCard(div) {
		t.Error("expected isJobCard=true for div with jobList-item class")
	}
}

func TestZipRecruiterScraper_IsJobCard_SpanReturnsFalse(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<span data-job-id="123">title</span>`)
	span := findFirstHTMLElement(doc, "span")
	if span == nil {
		t.Fatal("could not find span node")
	}
	if s.isJobCard(span) {
		t.Error("expected isJobCard=false for span element")
	}
}

func TestZipRecruiterScraper_IsJobCard_ArticleNoDataJobIDNoClass(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<article class="unrelated">hello</article>`)
	article := findFirstHTMLElement(doc, "article")
	if article == nil {
		t.Fatal("could not find article node")
	}
	if s.isJobCard(article) {
		t.Error("expected isJobCard=false for article without data-job-id or job_result class")
	}
}

// ── extractJobCard — missing required fields ──────────────────────────────────

func TestZipRecruiterScraper_ExtractJobCard_MissingTitle(t *testing.T) {
	// A card with data-job-id and company but NO title should return nil.
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `
		<article data-job-id="missing_title">
			<span class="hiring_company_text">Acme</span>
			<span class="location">Remote</span>
		</article>`)
	article := findFirstHTMLElement(doc, "article")
	if article == nil {
		t.Fatal("could not find article node")
	}
	job := s.extractJobCard(article)
	if job != nil {
		t.Errorf("expected nil for card missing title, got %+v", job)
	}
}

func TestZipRecruiterScraper_ExtractJobCard_MissingCompany(t *testing.T) {
	// A card with data-job-id and title but NO company should return nil.
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `
		<article data-job-id="missing_company">
			<span class="job_title">Engineer</span>
			<span class="location">New York, NY</span>
		</article>`)
	article := findFirstHTMLElement(doc, "article")
	if article == nil {
		t.Fatal("could not find article node")
	}
	job := s.extractJobCard(article)
	if job != nil {
		t.Errorf("expected nil for card missing company, got %+v", job)
	}
}

func TestZipRecruiterScraper_ExtractJobCard_MissingExternalID(t *testing.T) {
	// A div with class job_result but no data-job-id should return nil.
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `
		<div class="job_result">
			<span class="job_title">Engineer</span>
			<span class="hiring_company_text">Acme</span>
		</div>`)
	div := findFirstHTMLElement(doc, "div")
	if div == nil {
		t.Fatal("could not find div node")
	}
	job := s.extractJobCard(div)
	if job != nil {
		t.Errorf("expected nil for card without data-job-id, got %+v", job)
	}
}

// ── extractFieldFromNode — data-testid selectors ──────────────────────────────

func TestZipRecruiterScraper_ExtractFieldFromNode_DataTestIDTitle(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<a data-testid="job-title" href="/job/1">Senior Engineer</a>`)
	a := findFirstHTMLElement(doc, "a")
	if a == nil {
		t.Fatal("could not find anchor node")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(a, job)
	if job.Title != "Senior Engineer" {
		t.Errorf("expected Title=%q, got %q", "Senior Engineer", job.Title)
	}
}

func TestZipRecruiterScraper_ExtractFieldFromNode_DataTestIDCompany(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<span data-testid="job-company">Acme Corp</span>`)
	span := findFirstHTMLElement(doc, "span")
	if span == nil {
		t.Fatal("could not find span node")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(span, job)
	if job.Company != "Acme Corp" {
		t.Errorf("expected Company=%q, got %q", "Acme Corp", job.Company)
	}
}

func TestZipRecruiterScraper_ExtractFieldFromNode_DataTestIDLocation(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<span data-testid="job-location">Remote</span>`)
	span := findFirstHTMLElement(doc, "span")
	if span == nil {
		t.Fatal("could not find span node")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(span, job)
	if job.Location != "Remote" {
		t.Errorf("expected Location=%q, got %q", "Remote", job.Location)
	}
	if !job.IsRemote {
		t.Error("expected IsRemote=true for location 'Remote'")
	}
}

func TestZipRecruiterScraper_ExtractFieldFromNode_DataTestIDSalary(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<span data-testid="job-salary">$120,000 - $160,000 a year</span>`)
	span := findFirstHTMLElement(doc, "span")
	if span == nil {
		t.Fatal("could not find span node")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(span, job)
	if job.SalaryMin == nil {
		t.Error("expected SalaryMin to be populated")
	}
}

func TestZipRecruiterScraper_ExtractFieldFromNode_NoMatchIsNoop(t *testing.T) {
	s := newTestZipScraper()
	doc := mustParseHTMLNode(t, `<div class="totally-unknown-class">value</div>`)
	div := findFirstHTMLElement(doc, "div")
	if div == nil {
		t.Fatal("could not find div node")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(div, job)
	// Nothing should be set.
	if job.Title != "" || job.Company != "" || job.Location != "" {
		t.Error("expected no fields populated for unknown class")
	}
}

// ── scrapePage integration tests ──────────────────────────────────────────────

func TestZipRecruiterScraper_ScrapePage_ParsesJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	fixture := loadFixture(t, "ziprecruiter_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fixture))
	}))
	defer srv.Close()

	s := &zipRecruiterScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	jobs, err := s.scrapePage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("scrapePage returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected ≥1 job from fixture page, got 0")
	}
}

func TestZipRecruiterScraper_ScrapePage_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := &zipRecruiterScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestZipRecruiterScraper_ScrapePage_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := &zipRecruiterScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") && !strings.Contains(err.Error(), "429") {
		t.Errorf("expected rate-limit message, got: %v", err)
	}
}

func TestZipRecruiterScraper_Search_ContextCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := newTestZipScraper()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the goroutine hits the ctx.Done() check.
	cancel()

	params := SearchParams{Keywords: []string{"go"}, MaxPages: 3}
	jobsCh, errCh := s.Search(ctx, params)

	done := make(chan struct{})
	go func() {
		drainZR(jobsCh, errCh)
		close(done)
	}()

	select {
	case <-done:
		// Good — channels closed after context cancel.
	case <-time.After(3 * time.Second):
		t.Error("channels did not close after context cancellation within 3s")
	}
}

// ── enrichATSType ─────────────────────────────────────────────────────────────

func TestZipRecruiterScraper_EnrichATSType_FollowsRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Final ATS destination (not ziprecruiter.com domain).
	atsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("apply page"))
	}))
	defer atsSrv.Close()

	// Intermediate redirect pointing to the ATS server.
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, atsSrv.URL, http.StatusFound)
	}))
	defer redirectSrv.Close()

	s := newTestZipScraper()
	job := &models.ScrapedJob{
		Title:    "Engineer",
		ApplyURL: redirectSrv.URL,
	}

	s.enrichATSType(job)

	// ApplyURL should be updated to the ATS destination because it differs
	// from the original URL and is not a ziprecruiter.com domain.
	if job.ApplyURL == redirectSrv.URL {
		t.Errorf("expected ApplyURL to be updated from %q after redirect", redirectSrv.URL)
	}
}

func TestZipRecruiterScraper_EnrichATSType_EmptyURL(t *testing.T) {
	s := newTestZipScraper()
	job := &models.ScrapedJob{Title: "Engineer", ApplyURL: ""}

	// Should not panic or error.
	s.enrichATSType(job)

	if job.ApplyURL != "" {
		t.Errorf("expected empty ApplyURL unchanged, got %q", job.ApplyURL)
	}
}

func TestZipRecruiterScraper_EnrichATSType_NoRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Server that responds 200 without redirecting.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("job page"))
	}))
	defer srv.Close()

	s := newTestZipScraper()
	originalURL := srv.URL
	job := &models.ScrapedJob{
		Title:    "Engineer",
		ApplyURL: originalURL,
	}

	s.enrichATSType(job)

	// URL didn't change and is same as original — no update expected.
	// (The scraper only updates if finalURL != job.ApplyURL)
	t.Logf("ApplyURL after no-redirect: %q (original: %q)", job.ApplyURL, originalURL)
}

// ── source / name ──────────────────────────────────────────────────────────────

func TestZipRecruiterScraper_Source(t *testing.T) {
	s := NewZipRecruiterScraper()
	if got := s.Source(); got != models.SourceZipRecruiter {
		t.Errorf("Source() = %q, want %q", got, models.SourceZipRecruiter)
	}
}

func TestZipRecruiterScraper_Name(t *testing.T) {
	s := NewZipRecruiterScraper()
	if s.Name() == "" {
		t.Error("Name() returned empty string")
	}
}

// ── buildSearchURL ─────────────────────────────────────────────────────────────

func TestZipRecruiterScraper_BuildSearchURL_Remote(t *testing.T) {
	s := newTestZipScraper()
	u := s.buildSearchURL("go developer", "", true, 1)
	if !strings.Contains(u, "refine_by_tags=remote") {
		t.Errorf("expected remote filter in URL, got: %s", u)
	}
}

func TestZipRecruiterScraper_BuildSearchURL_RemoteLocation(t *testing.T) {
	// When location is "remote" (case-insensitive), remote filter is applied.
	s := newTestZipScraper()
	u := s.buildSearchURL("go developer", "remote", false, 1)
	if !strings.Contains(u, "refine_by_tags=remote") {
		t.Errorf("expected remote filter in URL for location=remote, got: %s", u)
	}
}

func TestZipRecruiterScraper_BuildSearchURL_WithLocation(t *testing.T) {
	s := newTestZipScraper()
	u := s.buildSearchURL("go developer", "New York, NY", false, 1)
	if !strings.Contains(u, "location=") {
		t.Errorf("expected location in URL, got: %s", u)
	}
}

func TestZipRecruiterScraper_BuildSearchURL_Page2HasPageParam(t *testing.T) {
	s := newTestZipScraper()
	u := s.buildSearchURL("go developer", "Remote", false, 2)
	if !strings.Contains(u, "page=2") {
		t.Errorf("expected page=2 in URL, got: %s", u)
	}
}

func TestZipRecruiterScraper_BuildSearchURL_Page1HasNoPageParam(t *testing.T) {
	s := newTestZipScraper()
	u := s.buildSearchURL("go developer", "Remote", false, 1)
	if strings.Contains(u, "page=") {
		t.Errorf("expected no page param for page 1, got: %s", u)
	}
}

func TestZipRecruiterScraper_BuildSearchURL_Days14(t *testing.T) {
	s := newTestZipScraper()
	u := s.buildSearchURL("engineer", "", false, 1)
	if !strings.Contains(u, "days=14") {
		t.Errorf("expected days=14 in URL, got: %s", u)
	}
}
