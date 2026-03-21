package plugins_test

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/ats"
	"github.com/bhata/AutoDreamApplier/internal/ats/plugins"
	"github.com/bhata/AutoDreamApplier/internal/browser"
)

// newBrowserClient returns a browser.Client pointing at a non-existent server.
// Passing nil panics in plugin constructors, so we use a dummy URL.
// No actual network calls are made in these tests (Name/Detect/ValidateURL only).
func newBrowserClient() *browser.Client {
	return browser.New("http://localhost:19999", zerolog.Nop())
}

// ── GreenhousePlugin ──────────────────────────────────────────────────────────

func TestGreenhousePlugin_Name(t *testing.T) {
	t.Parallel()
	p := plugins.NewGreenhousePlugin(newBrowserClient(), zerolog.Nop())
	if got := p.Name(); got != "greenhouse" {
		t.Errorf("Name() = %q; want %q", got, "greenhouse")
	}
}

func TestGreenhousePlugin_Detect(t *testing.T) {
	t.Parallel()
	p := plugins.NewGreenhousePlugin(newBrowserClient(), zerolog.Nop())

	cases := []struct {
		url  string
		want bool
	}{
		{"https://boards.greenhouse.io/stripe/jobs/123", true},
		{"https://grnh.se/abc123", true},
		{"https://greenhouse.io/embed/job_app?for=company", true},
		{"https://BOARDS.GREENHOUSE.IO/jobs/1", true},
		{"https://jobs.lever.co/company/abc", false},
		{"https://unknown.example.com/jobs/1", false},
		{"", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			got := p.Detect(tc.url)
			if got != tc.want {
				t.Errorf("Detect(%q) = %v; want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestGreenhousePlugin_ValidateURL(t *testing.T) {
	t.Parallel()
	p := plugins.NewGreenhousePlugin(newBrowserClient(), zerolog.Nop())

	if !p.ValidateURL("https://boards.greenhouse.io/company/jobs/1") {
		t.Error("ValidateURL should return true for valid greenhouse URL")
	}
	if p.ValidateURL("https://unknown.example.com") {
		t.Error("ValidateURL should return false for non-greenhouse URL")
	}
}

// ── LeverPlugin ───────────────────────────────────────────────────────────────

func TestLeverPlugin_Name(t *testing.T) {
	t.Parallel()
	p := plugins.NewLeverPlugin(newBrowserClient(), zerolog.Nop())
	if got := p.Name(); got != "lever" {
		t.Errorf("Name() = %q; want %q", got, "lever")
	}
}

func TestLeverPlugin_Detect(t *testing.T) {
	t.Parallel()
	p := plugins.NewLeverPlugin(newBrowserClient(), zerolog.Nop())

	cases := []struct {
		url  string
		want bool
	}{
		{"https://jobs.lever.co/airbnb/abc-def", true},
		{"https://jobs.lever.co/airbnb/abc-def/apply", true},
		{"https://apply.lever.co/jobs", true},
		{"HTTPS://JOBS.LEVER.CO/COMPANY/123", true},
		{"https://boards.greenhouse.io/jobs/1", false},
		{"https://unknown.example.com/jobs/1", false},
		{"", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			got := p.Detect(tc.url)
			if got != tc.want {
				t.Errorf("Detect(%q) = %v; want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestLeverPlugin_ValidateURL(t *testing.T) {
	t.Parallel()
	p := plugins.NewLeverPlugin(newBrowserClient(), zerolog.Nop())

	if !p.ValidateURL("https://jobs.lever.co/company/123") {
		t.Error("ValidateURL should return true for valid lever URL")
	}
	if p.ValidateURL("https://boards.greenhouse.io/company/jobs/1") {
		t.Error("ValidateURL should return false for non-lever URL")
	}
}

// ── WorkdayPlugin ─────────────────────────────────────────────────────────────

func TestWorkdayPlugin_Name(t *testing.T) {
	t.Parallel()
	p := plugins.NewWorkdayPlugin(newBrowserClient(), zerolog.Nop())
	if got := p.Name(); got != "workday" {
		t.Errorf("Name() = %q; want %q", got, "workday")
	}
}

func TestWorkdayPlugin_Detect(t *testing.T) {
	t.Parallel()
	p := plugins.NewWorkdayPlugin(newBrowserClient(), zerolog.Nop())

	cases := []struct {
		url  string
		want bool
	}{
		{"https://company.myworkdayjobs.com/en-US/external", true},
		{"https://microsoft.wd1.myworkdayjobs.com/en-US/jobs/123", true},
		{"https://ibm.wd3.myworkdayjobs.com/en-US/jobs/456", true},
		{"https://amazon.wd5.myworkdayjobs.com/en-US/jobs/789", true},
		{"https://workday.com/en-us/find-jobs/engineering", true},
		{"HTTPS://COMPANY.MYWORKDAYJOBS.COM/JOBS", true},
		{"https://boards.greenhouse.io/jobs/1", false},
		{"https://unknown.example.com/jobs/1", false},
		{"", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(strings.ReplaceAll(tc.url, "/", "_"), func(t *testing.T) {
			t.Parallel()
			got := p.Detect(tc.url)
			if got != tc.want {
				t.Errorf("Detect(%q) = %v; want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestWorkdayPlugin_ValidateURL(t *testing.T) {
	t.Parallel()
	p := plugins.NewWorkdayPlugin(newBrowserClient(), zerolog.Nop())

	if !p.ValidateURL("https://company.wd1.myworkdayjobs.com/en-US/jobs/1") {
		t.Error("ValidateURL should return true for valid workday URL")
	}
	if p.ValidateURL("https://boards.greenhouse.io/company/jobs/1") {
		t.Error("ValidateURL should return false for non-workday URL")
	}
}

// ── Apply: invalid URL path (no network call required) ────────────────────────

func TestGreenhousePlugin_Apply_InvalidURL_ReturnsError(t *testing.T) {
	t.Parallel()
	p := plugins.NewGreenhousePlugin(newBrowserClient(), zerolog.Nop())
	ctx := t.Context()

	data := &ats.ApplicationData{
		FullName: "Test User",
		Email:    "test@example.com",
	}

	result, err := p.Apply(ctx, "https://unknown.example.com/jobs/1", data)
	if err == nil {
		t.Fatal("Apply with invalid URL: expected error, got nil")
	}
	if result == nil {
		t.Fatal("Apply with invalid URL: expected non-nil result")
	}
	if result.ATSType != "greenhouse" {
		t.Errorf("result.ATSType = %q; want 'greenhouse'", result.ATSType)
	}
	if result.ErrorMessage == "" {
		t.Error("result.ErrorMessage should not be empty for invalid URL")
	}
}

func TestLeverPlugin_Apply_InvalidURL_ReturnsError(t *testing.T) {
	t.Parallel()
	p := plugins.NewLeverPlugin(newBrowserClient(), zerolog.Nop())
	ctx := t.Context()

	data := &ats.ApplicationData{
		FullName: "Test User",
		Email:    "test@example.com",
	}

	result, err := p.Apply(ctx, "https://unknown.example.com/jobs/1", data)
	if err == nil {
		t.Fatal("Apply with invalid URL: expected error, got nil")
	}
	if result == nil {
		t.Fatal("Apply with invalid URL: expected non-nil result")
	}
	if result.ATSType != "lever" {
		t.Errorf("result.ATSType = %q; want 'lever'", result.ATSType)
	}
	if result.ErrorMessage == "" {
		t.Error("result.ErrorMessage should not be empty for invalid URL")
	}
}

func TestWorkdayPlugin_Apply_InvalidURL_ReturnsError(t *testing.T) {
	t.Parallel()
	p := plugins.NewWorkdayPlugin(newBrowserClient(), zerolog.Nop())
	ctx := t.Context()

	data := &ats.ApplicationData{
		FullName: "Test User",
		Email:    "test@example.com",
	}

	result, err := p.Apply(ctx, "https://unknown.example.com/jobs/1", data)
	if err == nil {
		t.Fatal("Apply with invalid URL: expected error, got nil")
	}
	if result == nil {
		t.Fatal("Apply with invalid URL: expected non-nil result")
	}
	if result.ATSType != "workday" {
		t.Errorf("result.ATSType = %q; want 'workday'", result.ATSType)
	}
	if result.ErrorMessage == "" {
		t.Error("result.ErrorMessage should not be empty for invalid URL")
	}
}
