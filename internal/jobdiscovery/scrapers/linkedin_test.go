package scrapers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// linkedInJobCardHTML is a minimal fixture that mimics LinkedIn's public job
// search results HTML (logged-out / public view).
const linkedInJobCardHTML = `<!DOCTYPE html>
<html>
<body>
<ul class="jobs-search__results-list">
  <li>
    <div class="base-card job-search-card" data-entity-urn="urn:li:jobPosting:9876543210">
      <a class="base-card__full-link" href="/jobs/view/9876543210?refId=test" aria-label="Senior Go Engineer">
        Senior Go Engineer
      </a>
      <h3 class="base-search-card__title">Senior Go Engineer</h3>
      <h4 class="base-search-card__subtitle">Acme Corp</h4>
      <span class="job-search-card__location">San Francisco, CA (Remote)</span>
      <p class="job-search-card__snippet">Build scalable backend services in Go.</p>
    </div>
  </li>
  <li>
    <div class="base-card job-search-card" data-job-id="1111111111">
      <a class="base-card__full-link" href="https://www.linkedin.com/jobs/view/1111111111">
        Staff Software Engineer
      </a>
      <h3 class="base-search-card__title">Staff Software Engineer</h3>
      <h4 class="base-search-card__subtitle">StartupXYZ</h4>
      <span class="job-search-card__location">New York, NY</span>
    </div>
  </li>
  <li>
    <!-- Incomplete card — should be skipped (missing title and company) -->
    <div class="base-card job-search-card" data-job-id="9999999999">
    </div>
  </li>
</ul>
</body>
</html>`

func TestLinkedInScraper_ParseHTML(t *testing.T) {
	s := NewLinkedInScraper("").(*linkedInScraper)

	jobs, err := s.parseHTML(linkedInJobCardHTML)
	if err != nil {
		t.Fatalf("parseHTML returned error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// First job — extracted via data-entity-urn
	j0 := jobs[0]
	if j0.ExternalID != "linkedin_9876543210" {
		t.Errorf("job[0].ExternalID = %q, want %q", j0.ExternalID, "linkedin_9876543210")
	}
	if j0.Title != "Senior Go Engineer" {
		t.Errorf("job[0].Title = %q, want %q", j0.Title, "Senior Go Engineer")
	}
	if j0.Company != "Acme Corp" {
		t.Errorf("job[0].Company = %q, want %q", j0.Company, "Acme Corp")
	}
	if !j0.IsRemote {
		t.Errorf("job[0].IsRemote should be true (location contains 'Remote')")
	}
	if !strings.HasPrefix(j0.URL, "https://www.linkedin.com/jobs/view/9876543210") {
		t.Errorf("job[0].URL = %q, expected prefix https://www.linkedin.com/jobs/view/9876543210", j0.URL)
	}

	// Second job — extracted via data-job-id
	j1 := jobs[1]
	if j1.ExternalID != "linkedin_1111111111" {
		t.Errorf("job[1].ExternalID = %q, want %q", j1.ExternalID, "linkedin_1111111111")
	}
	if j1.Title != "Staff Software Engineer" {
		t.Errorf("job[1].Title = %q, want %q", j1.Title, "Staff Software Engineer")
	}
	if j1.Company != "StartupXYZ" {
		t.Errorf("job[1].Company = %q, want %q", j1.Company, "StartupXYZ")
	}
	if j1.IsRemote {
		t.Errorf("job[1].IsRemote should be false")
	}
}

func TestLinkedInScraper_BlockedResponse(t *testing.T) {
	// Simulate a 429 response from LinkedIn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	s := NewLinkedInScraper("").(*linkedInScraper)
	// Point the scraper at our test server by temporarily overriding the URL
	// via scrapePage directly.
	_, err := s.scrapePage(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected ErrSourceBlocked, got nil")
	}
	if !strings.Contains(err.Error(), ErrSourceBlocked.Error()) {
		t.Errorf("expected ErrSourceBlocked in error chain, got: %v", err)
	}
}

func TestLinkedInScraper_ForbiddenResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	s := NewLinkedInScraper("").(*linkedInScraper)
	_, err := s.scrapePage(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected ErrSourceBlocked, got nil")
	}
	if !strings.Contains(err.Error(), ErrSourceBlocked.Error()) {
		t.Errorf("expected ErrSourceBlocked in error chain, got: %v", err)
	}
}

func TestLinkedInScraper_Source(t *testing.T) {
	s := NewLinkedInScraper("")
	if s.Source() != "linkedin" {
		t.Errorf("Source() = %q, want %q", s.Source(), "linkedin")
	}
	if s.Name() != "LinkedIn (Easy Apply)" {
		t.Errorf("Name() = %q, want %q", s.Name(), "LinkedIn (Easy Apply)")
	}
}

func TestLinkedInScraper_BuildSearchURL(t *testing.T) {
	s := NewLinkedInScraper("").(*linkedInScraper)

	tests := []struct {
		query    string
		location string
		remote   bool
		start    int
		wantSubs []string
	}{
		{
			query:    "golang engineer",
			location: "San Francisco",
			remote:   false,
			start:    0,
			wantSubs: []string{"keywords=golang+engineer", "f_LF=f_AL", "location=San+Francisco"},
		},
		{
			query:    "software engineer",
			location: "",
			remote:   true,
			start:    25,
			wantSubs: []string{"f_WT=2", "start=25"},
		},
		{
			query:    "product manager",
			location: "Remote",
			remote:   false,
			start:    0,
			wantSubs: []string{"f_WT=2"},
		},
	}

	for _, tt := range tests {
		u := s.buildSearchURL(tt.query, tt.location, tt.remote, tt.start)
		for _, sub := range tt.wantSubs {
			if !strings.Contains(u, sub) {
				t.Errorf("buildSearchURL(%q, %q, %v, %d) = %q, want substring %q",
					tt.query, tt.location, tt.remote, tt.start, u, sub)
			}
		}
	}
}

func TestLinkedInScraper_ContextCancellation(t *testing.T) {
	// Server that hangs forever — context cancel should terminate Search.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	s := NewLinkedInScraper("")
	jobsCh, errCh := s.Search(ctx, SearchParams{Keywords: []string{"go"}, MaxPages: 1})

	// Drain channels — must not block forever
	for range jobsCh {
	}
	for range errCh {
	}
	// If we reach here without deadlock, the test passes.
}
