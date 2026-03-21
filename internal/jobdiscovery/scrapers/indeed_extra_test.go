package scrapers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// ── shared HTML helpers for test files in this package ────────────────────────

// parseHTMLString is a convenience wrapper around html.Parse.
func parseHTMLString(s string) (*html.Node, error) {
	return html.Parse(strings.NewReader(s))
}

// findElem does a depth-first search and returns the first html.ElementNode
// whose tag name matches the given tag.
func findElem(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findElem(child, tag); found != nil {
			return found
		}
	}
	return nil
}

// ── parseHTML unit tests ──────────────────────────────────────────────────────

const indeedJobHTML = `<!DOCTYPE html>
<html>
<body>
  <div data-jk="abc123">
    <h2 class="jobTitle"><span>Software Engineer</span></h2>
    <span class="companyName">Acme Corp</span>
    <div class="companyLocation">San Francisco, CA</div>
    <div class="salary-snippet">$120K - $160K a year</div>
  </div>
  <div data-jk="def456">
    <h2 class="jobTitle"><span>Senior Go Developer</span></h2>
    <span class="companyName">BetaCorp</span>
    <div class="companyLocation">Remote</div>
  </div>
</body>
</html>`

func TestIndeedScraper_ParseHTML_ParsesJobs(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)

	jobs, err := s.parseHTML(indeedJobHTML)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job, got 0")
	}
}

func TestIndeedScraper_ParseHTML_JobFields(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)

	jobs, err := s.parseHTML(indeedJobHTML)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed")
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
	if job.Source != models.SourceIndeed {
		t.Errorf("job.Source = %q, want %q", job.Source, models.SourceIndeed)
	}
	if !strings.HasPrefix(job.URL, "https://www.indeed.com/viewjob?jk=") {
		t.Errorf("job.URL = %q, expected indeed viewjob URL", job.URL)
	}
}

func TestIndeedScraper_ParseHTML_RemoteFlag(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)

	jobs, err := s.parseHTML(indeedJobHTML)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	found := false
	for _, job := range jobs {
		if job.ExternalID == "def456" {
			found = true
			if !job.IsRemote {
				t.Errorf("expected IsRemote=true for location %q", job.Location)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected job with ExternalID %q", "def456")
	}
}

func TestIndeedScraper_ParseHTML_EmptyPage(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)

	jobs, err := s.parseHTML(`<html><body><div id="empty">No results</div></body></html>`)
	if err != nil {
		t.Fatalf("parseHTML on empty page returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs on empty page, got %d", len(jobs))
	}
}

func TestIndeedScraper_ParseHTML_InvalidHTML(t *testing.T) {
	// golang.org/x/net/html is lenient and won't error on garbage.
	s := NewIndeedScraper().(*indeedScraper)

	jobs, err := s.parseHTML(`<<not valid>>`)
	if err != nil {
		t.Fatalf("parseHTML on invalid HTML returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs from garbage HTML, got %d", len(jobs))
	}
}

func TestIndeedScraper_ParseHTML_MissingFieldsReturnsNil(t *testing.T) {
	// A card with data-jk but no title/company should be skipped.
	const h = `<html><body>
		<div data-jk="nofields">
			<div class="companyLocation">Remote</div>
		</div>
	</body></html>`

	s := NewIndeedScraper().(*indeedScraper)
	jobs, err := s.parseHTML(h)
	if err != nil {
		t.Fatalf("parseHTML returned unexpected error: %v", err)
	}
	// Incomplete card (no title, no company) must be dropped.
	for _, job := range jobs {
		if job.ExternalID == "nofields" {
			t.Errorf("expected incomplete card to be skipped, but got job: %+v", job)
		}
	}
}

// ── enrichATSType — ATS URL recognition ──────────────────────────────────────

// TestEnrichATSType_GreenhouseURL verifies that when the redirect lands on a
// greenhouse.io-like URL, that URL is captured in ApplyURL.
func TestEnrichATSType_GreenhouseURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Simulate Greenhouse redirect target.
	greenhouseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Greenhouse Apply Page"))
	}))
	defer greenhouseSrv.Close()

	// Indeed wrapper redirects to greenhouse.
	indeedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, greenhouseSrv.URL, http.StatusFound)
	}))
	defer indeedSrv.Close()

	s := NewIndeedScraper().(*indeedScraper)
	job := &models.ScrapedJob{
		Title:    "Software Engineer",
		ApplyURL: indeedSrv.URL,
	}

	s.enrichATSType(job)

	// ApplyURL should now point to the greenhouse server.
	if job.ApplyURL != greenhouseSrv.URL {
		t.Errorf("expected ApplyURL=%q (greenhouse), got %q", greenhouseSrv.URL, job.ApplyURL)
	}
}

// TestEnrichATSType_LeverURL verifies that when the redirect lands on a
// lever.co-like URL, that URL is captured in ApplyURL.
func TestEnrichATSType_LeverURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Simulate Lever redirect target.
	leverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Lever Apply Page"))
	}))
	defer leverSrv.Close()

	// Indeed wrapper redirects to Lever.
	indeedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, leverSrv.URL, http.StatusFound)
	}))
	defer indeedSrv.Close()

	s := NewIndeedScraper().(*indeedScraper)
	job := &models.ScrapedJob{
		Title:    "Backend Engineer",
		ApplyURL: indeedSrv.URL,
	}

	s.enrichATSType(job)

	if job.ApplyURL != leverSrv.URL {
		t.Errorf("expected ApplyURL=%q (lever), got %q", leverSrv.URL, job.ApplyURL)
	}
}

// TestEnrichATSType_UnknownURL verifies that when the apply URL stays on the
// same server without redirecting (unknown domain), no URL update is made.
func TestEnrichATSType_UnknownURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Server that always returns 200 without redirecting.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Unknown Apply Page"))
	}))
	defer srv.Close()

	s := NewIndeedScraper().(*indeedScraper)
	originalURL := srv.URL
	job := &models.ScrapedJob{
		Title:    "Designer",
		ApplyURL: originalURL,
	}

	s.enrichATSType(job)

	// When finalURL == initialURL, no update should happen.
	if job.ApplyURL != originalURL {
		t.Errorf("expected ApplyURL to remain %q for no-redirect case, got %q", originalURL, job.ApplyURL)
	}
}

// TestEnrichATSType_BadURL verifies that a malformed URL does not panic.
func TestEnrichATSType_BadURL(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	job := &models.ScrapedJob{
		Title:    "Test",
		ApplyURL: "not-a-valid-url-%%%",
	}
	// Should not panic.
	s.enrichATSType(job)
}

// ── scrapePage error paths ────────────────────────────────────────────────────

func TestIndeedScraper_ScrapePage_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := &indeedScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention 500, got: %v", err)
	}
}

func TestIndeedScraper_ScrapePage_RateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := &indeedScraper{
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

func TestIndeedScraper_ScrapePage_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(indeedJobHTML))
	}))
	defer srv.Close()

	s := &indeedScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	jobs, err := s.scrapePage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("scrapePage returned unexpected error: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected ≥1 job from fixture HTML, got 0")
	}
}

// ── Search — context cancellation during multi-page scrape ────────────────────

func TestIndeedScraper_Search_ContextCancelMultiPage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := NewIndeedScraper().(*indeedScraper)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the goroutine hits the ctx.Done() check before
	// making any real HTTP call.
	cancel()

	params := SearchParams{
		Keywords: []string{"golang", "backend"},
		Location: "Remote",
		MaxPages: 5,
	}

	jobsCh, errCh := s.Search(ctx, params)

	done := make(chan struct{})
	go func() {
		timeout := time.After(3 * time.Second)
		jDone, eDone := false, false
		for !jDone || !eDone {
			select {
			case _, ok := <-jobsCh:
				if !ok {
					jDone = true
				}
			case _, ok := <-errCh:
				if !ok {
					eDone = true
				}
			case <-timeout:
				jDone, eDone = true, true
			}
		}
		close(done)
	}()

	select {
	case <-done:
		// Good — channels closed cleanly after context cancel.
	case <-time.After(4 * time.Second):
		t.Error("channels did not close after context cancellation within 4s")
	}
}

// ── buildSearchURL ────────────────────────────────────────────────────────────

func TestIndeedScraper_BuildSearchURL_Remote(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	u := s.buildSearchURL("software engineer", "", true, 0)
	if !strings.Contains(u, "remotejob=") {
		t.Errorf("expected remotejob param in URL, got: %s", u)
	}
}

func TestIndeedScraper_BuildSearchURL_RemoteLocation(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	u := s.buildSearchURL("software engineer", "remote", false, 0)
	if !strings.Contains(u, "remotejob=") {
		t.Errorf("expected remotejob param for location=remote, got: %s", u)
	}
}

func TestIndeedScraper_BuildSearchURL_WithLocation(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	u := s.buildSearchURL("developer", "New York, NY", false, 0)
	if !strings.Contains(u, "l=") {
		t.Errorf("expected l= (location) param in URL, got: %s", u)
	}
}

func TestIndeedScraper_BuildSearchURL_PaginationOffset(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	u := s.buildSearchURL("engineer", "Remote", false, 20)
	if !strings.Contains(u, "start=20") {
		t.Errorf("expected start=20 in URL, got: %s", u)
	}
}

func TestIndeedScraper_BuildSearchURL_ContainsQuery(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	u := s.buildSearchURL("golang developer", "Austin, TX", false, 0)
	if !strings.Contains(u, "golang") {
		t.Errorf("expected query in URL, got: %s", u)
	}
}

// ── Source and Name ──────────────────────────────────────────────────────────

func TestIndeedScraper_Source(t *testing.T) {
	s := NewIndeedScraper()
	if got := s.Source(); got != models.SourceIndeed {
		t.Errorf("Source() = %q, want %q", got, models.SourceIndeed)
	}
}

func TestIndeedScraper_Name(t *testing.T) {
	s := NewIndeedScraper()
	if s.Name() == "" {
		t.Error("Name() returned empty string")
	}
}

// ── isJobCard ────────────────────────────────────────────────────────────────

func TestIndeedScraper_IsJobCard_DataJK(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<div data-jk="abc123"><span>job</span></div>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	div := findElem(doc, "div")
	if div == nil {
		t.Fatal("could not find div")
	}
	if !s.isJobCard(div) {
		t.Error("expected isJobCard=true for div with data-jk")
	}
}

func TestIndeedScraper_IsJobCard_JobSeenBeaconClass(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<div class="job_seen_beacon extra">content</div>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	div := findElem(doc, "div")
	if div == nil {
		t.Fatal("could not find div")
	}
	if !s.isJobCard(div) {
		t.Error("expected isJobCard=true for div with job_seen_beacon class")
	}
}

func TestIndeedScraper_IsJobCard_UnrelatedNode(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<div class="unrelated">content</div>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	div := findElem(doc, "div")
	if div == nil {
		t.Fatal("could not find div")
	}
	if s.isJobCard(div) {
		t.Error("expected isJobCard=false for unrelated div")
	}
}

// ── extractFieldFromNode ─────────────────────────────────────────────────────

func TestIndeedScraper_ExtractFieldFromNode_Title(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<h2 class="jobTitle"><span>Go Engineer</span></h2>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	h2 := findElem(doc, "h2")
	if h2 == nil {
		t.Fatal("could not find h2")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(h2, job)
	if job.Title != "Go Engineer" {
		t.Errorf("expected Title=%q, got %q", "Go Engineer", job.Title)
	}
}

func TestIndeedScraper_ExtractFieldFromNode_Company(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<span class="companyName">Acme Corp</span>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	span := findElem(doc, "span")
	if span == nil {
		t.Fatal("could not find span")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(span, job)
	if job.Company != "Acme Corp" {
		t.Errorf("expected Company=%q, got %q", "Acme Corp", job.Company)
	}
}

func TestIndeedScraper_ExtractFieldFromNode_Location(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<div class="companyLocation">Remote</div>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	div := findElem(doc, "div")
	if div == nil {
		t.Fatal("could not find div")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(div, job)
	if job.Location != "Remote" {
		t.Errorf("expected Location=%q, got %q", "Remote", job.Location)
	}
	if !job.IsRemote {
		t.Error("expected IsRemote=true for location 'Remote'")
	}
}

func TestIndeedScraper_ExtractFieldFromNode_Salary(t *testing.T) {
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<div class="salary-snippet">$120K - $160K a year</div>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	div := findElem(doc, "div")
	if div == nil {
		t.Fatal("could not find div")
	}
	job := &models.ScrapedJob{}
	s.extractFieldFromNode(div, job)
	if job.SalaryMin == nil {
		t.Error("expected SalaryMin to be populated")
	}
}

func TestIndeedScraper_ExtractFieldFromNode_TitleNotOverwritten(t *testing.T) {
	// If Title is already set, a second matching node should not overwrite it.
	s := NewIndeedScraper().(*indeedScraper)
	doc, err := parseHTMLString(`<h2 class="jobTitle"><span>New Title</span></h2>`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	h2 := findElem(doc, "h2")
	if h2 == nil {
		t.Fatal("could not find h2")
	}
	job := &models.ScrapedJob{Title: "Original Title"}
	s.extractFieldFromNode(h2, job)
	if job.Title != "Original Title" {
		t.Errorf("expected Title to remain %q, got %q", "Original Title", job.Title)
	}
}

// ── parseSalary helper ────────────────────────────────────────────────────────

func TestParseSalary_KNotation(t *testing.T) {
	min, max := parseSalary("$120K - $160K a year")
	if min == nil {
		t.Fatal("expected non-nil min")
	}
	if *min != 120000 {
		t.Errorf("expected min=120000, got %d", *min)
	}
	if max == nil {
		t.Fatal("expected non-nil max")
	}
	if *max != 160000 {
		t.Errorf("expected max=160000, got %d", *max)
	}
}

func TestParseSalary_NoSalary(t *testing.T) {
	min, max := parseSalary("No salary information")
	if min != nil {
		t.Errorf("expected nil min, got %d", *min)
	}
	if max != nil {
		t.Errorf("expected nil max, got %d", *max)
	}
}

func TestParseSalary_SingleValue(t *testing.T) {
	min, max := parseSalary("$90K a year")
	if min == nil {
		t.Fatal("expected non-nil min")
	}
	if *min != 90000 {
		t.Errorf("expected min=90000, got %d", *min)
	}
	if max != nil {
		t.Errorf("expected nil max for single value, got %d", *max)
	}
}
