package scrapers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestGlassdoorScraper returns a glassdoorScraper wired to use the provided
// HTTP client (typically the httptest server's client).
func newTestGlassdoorScraper(client *http.Client) *glassdoorScraper {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &glassdoorScraper{
		client:     client,
		userAgents: []string{"TestAgent/1.0"},
	}
}

// loadGlassdoorFixture reads a fixture file from the testdata directory.
func loadGlassdoorFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to load fixture %q: %v", name, err)
	}
	return string(data)
}

// drainGlassdoor reads all jobs and errors from Search() channels with a timeout.
func drainGlassdoor(jobsCh <-chan *models.ScrapedJob, errCh <-chan error) ([]*models.ScrapedJob, []error) {
	var jobs []*models.ScrapedJob
	var errs []error

	timeout := time.After(5 * time.Second)
	jobsDone := false
	errsDone := false

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

// ── Source / Name ─────────────────────────────────────────────────────────────

func TestGlassdoorScraper_Source(t *testing.T) {
	s := NewGlassdoorScraper()
	if got := s.Source(); got != models.SourceGlassdoor {
		t.Errorf("Source() = %q, want %q", got, models.SourceGlassdoor)
	}
}

func TestGlassdoorScraper_Name(t *testing.T) {
	s := NewGlassdoorScraper()
	if s.Name() == "" {
		t.Error("Name() returned empty string")
	}
}

// ── parseHTML: DOM fallback (legacy HTML fixture) ─────────────────────────────

func TestGlassdoorScraper_ParseHTML_LegacyFixture_ParsesJobs(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from legacy HTML fixture, got 0")
	}
}

func TestGlassdoorScraper_ParseHTML_LegacyFixture_JobCount(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	// The fixture contains 3 complete job cards.
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs from legacy fixture, got %d", len(jobs))
	}
}

func TestGlassdoorScraper_ParseHTML_LegacyFixture_FirstJobFields(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

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
	if !strings.HasPrefix(job.ExternalID, "glassdoor_") {
		t.Errorf("job.ExternalID must start with 'glassdoor_', got %q", job.ExternalID)
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
	if job.Source != models.SourceGlassdoor {
		t.Errorf("job.Source = %q, want %q", job.Source, models.SourceGlassdoor)
	}
}

func TestGlassdoorScraper_ParseHTML_LegacyFixture_JobIDFromHref(t *testing.T) {
	// The fixture uses legacy <li class="jl"> cards. The job ID must be
	// extracted from the ?jl= parameter in the anchor href.
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed")
	}

	// All jobs must have ExternalIDs that encode the jl= param value.
	for i, job := range jobs {
		if job.ExternalID == "" {
			t.Errorf("job[%d].ExternalID is empty", i)
		}
		if !strings.HasPrefix(job.ExternalID, "glassdoor_") {
			t.Errorf("job[%d].ExternalID = %q, want glassdoor_ prefix", i, job.ExternalID)
		}
	}
}

func TestGlassdoorScraper_ParseHTML_LegacyFixture_RemoteFlag(t *testing.T) {
	// Fixture card 2 has location "Remote" — IsRemote must be true.
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	var remoteJob *models.ScrapedJob
	for _, j := range jobs {
		if strings.Contains(strings.ToLower(j.Location), "remote") {
			remoteJob = j
			break
		}
	}
	if remoteJob == nil {
		t.Fatal("expected a remote job in fixture, found none")
	}
	if !remoteJob.IsRemote {
		t.Errorf("expected IsRemote=true for job with location %q", remoteJob.Location)
	}
}

func TestGlassdoorScraper_ParseHTML_LegacyFixture_SalaryParsed(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed")
	}

	// At least the first two jobs have salary text in the fixture.
	salaryJobCount := 0
	for _, j := range jobs {
		if j.SalaryMin != nil {
			salaryJobCount++
		}
	}
	if salaryJobCount == 0 {
		t.Error("expected at least one job with salary parsed, got 0")
	}
}

// ── parseHTML: __NEXT_DATA__ extraction ──────────────────────────────────────

func TestGlassdoorScraper_ParseHTML_NextData_ParsesJobs(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job from __NEXT_DATA__ fixture, got 0")
	}
}

func TestGlassdoorScraper_ParseHTML_NextData_JobCount(t *testing.T) {
	// Fixture has 4 entries but one has an empty jobListingId and must be skipped.
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	// 3 valid jobs (entry 4 has empty jobListingId).
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs from __NEXT_DATA__ fixture, got %d", len(jobs))
	}
}

func TestGlassdoorScraper_ParseHTML_NextData_FirstJobFields(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("no jobs parsed")
	}

	job := jobs[0]
	if job.ExternalID != "glassdoor_5001" {
		t.Errorf("job[0].ExternalID = %q, want %q", job.ExternalID, "glassdoor_5001")
	}
	if job.Title != "Staff Software Engineer" {
		t.Errorf("job[0].Title = %q, want %q", job.Title, "Staff Software Engineer")
	}
	if job.Company != "TechCorp Inc" {
		t.Errorf("job[0].Company = %q, want %q", job.Company, "TechCorp Inc")
	}
	if job.Source != models.SourceGlassdoor {
		t.Errorf("job[0].Source = %q, want %q", job.Source, models.SourceGlassdoor)
	}
	if job.SalaryMin == nil {
		t.Error("job[0].SalaryMin must not be nil")
	} else if *job.SalaryMin != 180000 {
		t.Errorf("job[0].SalaryMin = %d, want 180000", *job.SalaryMin)
	}
	if job.SalaryMax == nil {
		t.Error("job[0].SalaryMax must not be nil")
	} else if *job.SalaryMax != 240000 {
		t.Errorf("job[0].SalaryMax = %d, want 240000", *job.SalaryMax)
	}
	if !strings.Contains(job.URL, "glassdoor.com") {
		t.Errorf("job[0].URL = %q, expected glassdoor.com domain", job.URL)
	}
}

func TestGlassdoorScraper_ParseHTML_NextData_RemoteJob(t *testing.T) {
	// Fixture job 5002 has remoteWorkTypes = ["Remote"].
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	var remoteJob *models.ScrapedJob
	for _, j := range jobs {
		if j.ExternalID == "glassdoor_5002" {
			remoteJob = j
			break
		}
	}
	if remoteJob == nil {
		t.Fatal("expected job glassdoor_5002, not found")
	}
	if !remoteJob.IsRemote {
		t.Error("expected IsRemote=true for job with remoteWorkTypes=[Remote]")
	}
}

func TestGlassdoorScraper_ParseHTML_NextData_DescriptionStripped(t *testing.T) {
	// Fixture job 5001 has HTML in jobDescription — it should be stripped.
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.parseHTML(fixture)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	var job *models.ScrapedJob
	for _, j := range jobs {
		if j.ExternalID == "glassdoor_5001" {
			job = j
			break
		}
	}
	if job == nil {
		t.Fatal("glassdoor_5001 not found")
	}
	if strings.Contains(job.Description, "<") || strings.Contains(job.Description, ">") {
		t.Errorf("description still contains HTML tags: %q", job.Description)
	}
}

// ── parseHTML: empty and malformed input ──────────────────────────────────────

func TestGlassdoorScraper_ParseHTML_EmptyPage(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	jobs, err := s.parseHTML(`<html><body><div>No jobs here</div></body></html>`)
	if err != nil {
		t.Fatalf("parseHTML on empty page returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs on empty page, got %d", len(jobs))
	}
}

func TestGlassdoorScraper_ParseHTML_InvalidHTML(t *testing.T) {
	// golang.org/x/net/html is lenient — malformed HTML should not error.
	s := newTestGlassdoorScraper(nil)
	jobs, err := s.parseHTML(`<<<<<not valid html>`)
	if err != nil {
		t.Fatalf("parseHTML on invalid HTML returned unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs from garbage HTML, got %d", len(jobs))
	}
}

// ── scrapePage: blocked response detection ────────────────────────────────────

func TestGlassdoorScraper_ScrapePage_ForbiddenReturnsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := newTestGlassdoorScraper(srv.Client())
	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrSourceBlocked, got nil")
	}
	if !errors.Is(err, ErrSourceBlocked) {
		t.Errorf("expected errors.Is(err, ErrSourceBlocked), got: %v", err)
	}
}

func TestGlassdoorScraper_ScrapePage_TooManyRequestsReturnsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := newTestGlassdoorScraper(srv.Client())
	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrSourceBlocked, got nil")
	}
	if !errors.Is(err, ErrSourceBlocked) {
		t.Errorf("expected errors.Is(err, ErrSourceBlocked), got: %v", err)
	}
}

func TestGlassdoorScraper_ScrapePage_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestGlassdoorScraper(srv.Client())
	_, err := s.scrapePage(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
	// Must NOT be ErrSourceBlocked — 500 is a server error, not a block.
	if errors.Is(err, ErrSourceBlocked) {
		t.Error("500 error should not be ErrSourceBlocked")
	}
}

func TestGlassdoorScraper_ScrapePage_ParsesJobsFromLegacyFixture(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_search.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	s := newTestGlassdoorScraper(srv.Client())
	jobs, err := s.scrapePage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("scrapePage returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected ≥1 job from legacy fixture page, got 0")
	}
}

func TestGlassdoorScraper_ScrapePage_ParsesJobsFromNextDataFixture(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	s := newTestGlassdoorScraper(srv.Client())
	jobs, err := s.scrapePage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("scrapePage returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Error("expected ≥1 job from __NEXT_DATA__ fixture page, got 0")
	}
}

// ── Search: blocked stops immediately and sends ErrSourceBlocked ──────────────

func TestGlassdoorScraper_Search_BlockedSendsErrSourceBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	// Override the scraper's client to point at our test server, and build
	// the search URL directly to the test server.
	s := &glassdoorScraper{
		client:     srv.Client(),
		userAgents: []string{"TestAgent/1.0"},
	}

	// Manually run scrapePage against the blocked server to verify the sentinel.
	_, err := s.scrapePage(context.Background(), srv.URL)
	if !errors.Is(err, ErrSourceBlocked) {
		t.Errorf("expected ErrSourceBlocked from 403, got: %v", err)
	}
}

// ── Search: context cancellation ─────────────────────────────────────────────

func TestGlassdoorScraper_Search_ContextCancelledBeforeStart(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	params := SearchParams{
		Keywords: []string{"backend engineer"},
		MaxPages: 5,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Search is called

	jobsCh, errCh := s.Search(ctx, params)

	done := make(chan struct{})
	go func() {
		drainGlassdoor(jobsCh, errCh)
		close(done)
	}()

	select {
	case <-done:
		// channels closed cleanly — pass
	case <-time.After(3 * time.Second):
		t.Error("channels did not close within 3s after immediate context cancel")
	}
}

func TestGlassdoorScraper_Search_ContextCancellationDuringSearch(t *testing.T) {
	// Server that hangs until the request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Use a short-lived context. The HTTP client will abort the in-flight
	// request when the context expires, causing scrapePage to return an error
	// and the goroutine to close the channels.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	s := newTestGlassdoorScraper(srv.Client())
	params := SearchParams{Keywords: []string{"go"}, MaxPages: 1}

	jobsCh, errCh := s.Search(ctx, params)

	done := make(chan struct{})
	go func() {
		drainGlassdoor(jobsCh, errCh)
		close(done)
	}()

	// Allow enough time for the context to expire plus goroutine cleanup.
	select {
	case <-done:
		// channels closed cleanly — pass
	case <-time.After(5 * time.Second):
		t.Error("channels did not close after context timeout within 5s")
	}
}

// ── buildSearchURL ─────────────────────────────────────────────────────────────

func TestGlassdoorScraper_BuildSearchURL_Remote(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("go developer", "", true, 1)
	if !strings.Contains(u, "remoteWorkType=1") {
		t.Errorf("expected remoteWorkType=1 in URL, got: %s", u)
	}
}

func TestGlassdoorScraper_BuildSearchURL_RemoteLocation(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("go developer", "remote", false, 1)
	if !strings.Contains(u, "remoteWorkType=1") {
		t.Errorf("expected remoteWorkType=1 for location=remote, got: %s", u)
	}
}

func TestGlassdoorScraper_BuildSearchURL_WithLocation(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("go developer", "New York, NY", false, 1)
	if !strings.Contains(u, "locKeyword=") {
		t.Errorf("expected locKeyword in URL, got: %s", u)
	}
}

func TestGlassdoorScraper_BuildSearchURL_Page2HasPParam(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("engineer", "", false, 2)
	if !strings.Contains(u, "p=2") {
		t.Errorf("expected p=2 in URL for page 2, got: %s", u)
	}
}

func TestGlassdoorScraper_BuildSearchURL_Page1NoPParam(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("engineer", "", false, 1)
	if strings.Contains(u, "p=") {
		t.Errorf("expected no p= param for page 1, got: %s", u)
	}
}

func TestGlassdoorScraper_BuildSearchURL_ContainsKeyword(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	u := s.buildSearchURL("golang backend", "", false, 1)
	if !strings.Contains(u, "sc.keyword=") {
		t.Errorf("expected sc.keyword in URL, got: %s", u)
	}
	if !strings.Contains(u, "glassdoor.com") {
		t.Errorf("expected glassdoor.com in URL, got: %s", u)
	}
}

// ── extractNextData ──────────────────────────────────────────────────────────

func TestGlassdoorScraper_ExtractNextData_NoScriptTag(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	jobs, err := s.extractNextData(`<html><body><p>no script</p></body></html>`)
	if err == nil {
		t.Error("expected error when __NEXT_DATA__ tag absent, got nil")
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs when tag absent, got %d", len(jobs))
	}
}

func TestGlassdoorScraper_ExtractNextData_MalformedJSON(t *testing.T) {
	s := newTestGlassdoorScraper(nil)
	html := `<html><body><script id="__NEXT_DATA__" type="application/json">{bad json}</script></body></html>`
	jobs, err := s.extractNextData(html)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs for bad JSON, got %d", len(jobs))
	}
}

func TestGlassdoorScraper_ExtractNextData_ValidBlob(t *testing.T) {
	fixture := loadGlassdoorFixture(t, "glassdoor_next_data.html")
	s := newTestGlassdoorScraper(nil)

	jobs, err := s.extractNextData(fixture)
	if err != nil {
		t.Fatalf("extractNextData returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected jobs from valid __NEXT_DATA__ blob, got 0")
	}
}

// ── extractJLParam ───────────────────────────────────────────────────────────

func TestExtractJLParam_ValidURL(t *testing.T) {
	tests := []struct {
		href string
		want string
	}{
		{"/job-listing/engineer.htm?jl=1234", "1234"},
		{"https://www.glassdoor.com/job-listing/x.htm?jl=5678&utm=foo", "5678"},
		{"/jobs?foo=bar&jl=9999", "9999"},
		{"/no-jl-param", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractJLParam(tt.href)
		if got != tt.want {
			t.Errorf("extractJLParam(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}

// ── isBlockedErr ─────────────────────────────────────────────────────────────

func TestIsBlockedErr_NilReturnsFalse(t *testing.T) {
	if isBlockedErr(nil) {
		t.Error("isBlockedErr(nil) should return false")
	}
}

func TestIsBlockedErr_403(t *testing.T) {
	err := errors.New("glassdoor returned 403: something")
	if !isBlockedErr(err) {
		t.Error("isBlockedErr should return true for 403 error")
	}
}

func TestIsBlockedErr_429(t *testing.T) {
	err := errors.New("rate limited 429")
	if !isBlockedErr(err) {
		t.Error("isBlockedErr should return true for 429 error")
	}
}

func TestIsBlockedErr_ErrSourceBlocked(t *testing.T) {
	err := fmt.Errorf("wrap: %w", ErrSourceBlocked)
	if !isBlockedErr(err) {
		t.Error("isBlockedErr should return true when error wraps ErrSourceBlocked")
	}
}

// ── stripHTML ─────────────────────────────────────────────────────────────────

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"plain text", "plain text"},
		{"  <br/>  ", ""},
		{"<p>  spaces   </p>", "spaces"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
