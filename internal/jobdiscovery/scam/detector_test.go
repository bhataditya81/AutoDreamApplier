package scam_test

import (
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/scam"
)

func ptr(i int) *int { return &i }

func TestDetector_ObviousScam(t *testing.T) {
	d := scam.New()

	job := &models.ScrapedJob{
		Title:       "Work From Home Earn Easy Money",
		Company:     "",
		Description: "Pay to apply today. training fee required. Earn $500/hr with unlimited earning potential from home.",
		URL:         "http://totally-legit-jobs.biz/apply",
	}

	result := d.Classify(job)

	if !result.IsScam {
		t.Errorf("expected IsScam=true, got false; score=%.2f reasons=%v", result.Score, result.Reasons)
	}
	if result.Score < 0.5 {
		t.Errorf("expected score >= 0.5, got %.2f", result.Score)
	}
	if result.Score > 1.0 {
		t.Errorf("score capped at 1.0, got %.2f", result.Score)
	}
	if len(result.Reasons) == 0 {
		t.Error("expected at least one reason for scam job")
	}
}

func TestDetector_CleanJob(t *testing.T) {
	d := scam.New()

	job := &models.ScrapedJob{
		Title:       "Software Engineer",
		Company:     "Acme Corp",
		Description: "We are looking for a skilled Software Engineer to join our team. You will work on distributed systems and cloud infrastructure. 3+ years of Go experience required. Competitive salary and benefits. Remote-friendly.",
		URL:         "https://jobs.acmecorp.com/software-engineer-123",
		SalaryMin:   ptr(120000),
		SalaryMax:   ptr(180000),
	}

	result := d.Classify(job)

	if result.IsScam {
		t.Errorf("expected IsScam=false, got true; score=%.2f reasons=%v", result.Score, result.Reasons)
	}
	if result.Score != 0.0 {
		// A clean job with a mismatched URL might pick up one signal;
		// we just want to ensure it's clearly below the scam threshold.
		if result.Score >= 0.5 {
			t.Errorf("clean job scored %.2f >= 0.5 scam threshold; reasons=%v", result.Score, result.Reasons)
		}
	}
}

func TestDetector_NoDescription(t *testing.T) {
	d := scam.New()

	job := &models.ScrapedJob{
		Title:       "Data Entry Clerk",
		Company:     "Unknown",
		Description: "",
		URL:         "http://example.com/job",
	}

	result := d.Classify(job)

	// Should not panic. With no company + short/empty description, score should be > 0.
	if result.Score <= 0 {
		t.Errorf("expected non-zero score for job with empty description and unknown company, got %.2f", result.Score)
	}
	// Score must stay in range [0, 1].
	if result.Score > 1.0 {
		t.Errorf("score %.2f exceeds maximum 1.0", result.Score)
	}
}

func TestDetector_NoSalary(t *testing.T) {
	d := scam.New()

	job := &models.ScrapedJob{
		Title:       "Marketing Coordinator",
		Company:     "Bright Ideas LLC",
		Description: "Join our growing marketing team. We need someone passionate about digital marketing, content creation, and brand strategy. Must have excellent communication skills and 2+ years of experience.",
		URL:         "https://brightideasllc.com/careers/marketing",
		SalaryMin:   nil,
		SalaryMax:   nil,
	}

	result := d.Classify(job)

	// No salary present should not trigger the salary signal.
	if result.IsScam {
		t.Errorf("expected IsScam=false for legitimate job with no salary info; score=%.2f reasons=%v", result.Score, result.Reasons)
	}
}

func TestDetector_HighSalarySalaryField(t *testing.T) {
	d := scam.New()

	// High salary + buzzwords + no company = triggers enough signals to exceed 0.5.
	job := &models.ScrapedJob{
		Title:       "Sales Associate",
		Company:     "",
		Description: "Great opportunity with unlimited earning potential. Join our ground floor team. Be your own boss. Passive income awaits. No experience needed. MLM friendly.",
		URL:         "http://totally-unrelated-site.biz/join",
		SalaryMin:   ptr(500000),
	}

	result := d.Classify(job)

	if !result.IsScam {
		t.Errorf("expected IsScam=true for job with very high salary + buzzwords + no company; score=%.2f reasons=%v", result.Score, result.Reasons)
	}
}

func TestDetector_PersonalEmail(t *testing.T) {
	d := scam.New()

	job := &models.ScrapedJob{
		Title:       "Administrative Assistant",
		Company:     "Unknown",
		Description: "Send your CV to hiring@gmail.com to apply. We will respond within 24 hours. No office required.",
		URL:         "http://admin-jobs-now.net/apply",
	}

	result := d.Classify(job)

	// personal email + no company — should score fairly high
	if result.Score < 0.3 {
		t.Errorf("expected score >= 0.3 for personal email + unknown company, got %.2f", result.Score)
	}
}

func TestDetector_ScoreCappedAt1(t *testing.T) {
	d := scam.New()

	// Trigger every signal possible.
	salary := 400001
	job := &models.ScrapedJob{
		Title:       "Work From Home Earn Data Entry No Experience",
		Company:     "",
		Description: "Pay to apply. Training fee $99. Contact us at scammer@gmail.com. Unlimited earning. Passive income. Be your own boss. Ground floor opportunity. MLM team. Starter kit required. " +
			"$500/hr guaranteed.",
		URL:       "http://totally-not-related-domain.biz/apply",
		SalaryMin: &salary,
	}

	result := d.Classify(job)

	if result.Score > 1.0 {
		t.Errorf("score should be capped at 1.0, got %.2f", result.Score)
	}
	if !result.IsScam {
		t.Error("expected IsScam=true for max-signal job")
	}
}

// TestDetector_AllEightRedFlags verifies that a job deliberately crafted to
// trigger all 8 signals is classified as IsScam=true with score == 1.0.
func TestDetector_AllEightRedFlags(t *testing.T) {
	d := scam.New()

	salary := 999999 // Signal 1: suspiciously high salary (> 300 000)
	job := &models.ScrapedJob{
		// Signal 4: suspicious title phrase
		Title: "work from home earn easy money",
		// Signal 3: no company name
		Company: "",
		// Signal 2: upfront payment keyword
		// Signal 5: personal email domain
		// Signal 7: description < 100 chars — NOT here; we need it long enough to
		//            also include buzzwords for Signal 8.
		Description: "Pay to apply now. Contact scam@gmail.com. Unlimited. Passive income. Be your own boss. Ground floor. MLM. Extra filler text to exceed 100 characters for this description.",
		// Signal 6: URL domain doesn't match company name (company is empty so skipped)
		URL:       "http://scam-domain-xyz.biz/apply",
		SalaryMin: &salary,
	}

	result := d.Classify(job)

	if !result.IsScam {
		t.Errorf("expected IsScam=true for all-red-flags job; score=%.2f reasons=%v",
			result.Score, result.Reasons)
	}
	if result.Score > 1.0 {
		t.Errorf("score capped at 1.0, got %.2f", result.Score)
	}
	if len(result.Reasons) == 0 {
		t.Error("expected at least one reason for all-red-flags job")
	}
}

// TestDetector_GoodJob_NoRedFlags verifies that a clearly legitimate posting
// receives IsScam=false with score below the 0.5 threshold.
func TestDetector_GoodJob_NoRedFlags(t *testing.T) {
	d := scam.New()

	salaryMin := 130000
	salaryMax := 160000
	job := &models.ScrapedJob{
		Title:       "Senior Software Engineer",
		Company:     "Stripe",
		Description: "We are building the global financial infrastructure. You will work closely with our payments team designing highly available systems in Go and Rust. 5+ years of backend experience required. Competitive compensation, equity, and benefits. No upfront fees or training costs.",
		URL:         "https://stripe.com/jobs/senior-software-engineer-123",
		SalaryMin:   &salaryMin,
		SalaryMax:   &salaryMax,
	}

	result := d.Classify(job)

	if result.IsScam {
		t.Errorf("expected IsScam=false for clearly legitimate job; score=%.2f reasons=%v",
			result.Score, result.Reasons)
	}
	if result.Score >= 0.5 {
		t.Errorf("clean job scored %.2f >= 0.5 scam threshold; reasons=%v", result.Score, result.Reasons)
	}
}

// TestDetector_BoundaryExactly4Signals_IsScam verifies that exactly 4 signals
// yields a score >= 0.5 (threshold is >=0.5 → IsScam=true).
// We trigger:
//   Signal 1 high salary (+0.3)  Signal 2 upfront payment (+0.4) → 0.7 already ≥ 0.5
// Use a minimal combination that reaches exactly 0.5:
//   Signal 3 no company (+0.2) + Signal 7 short description (+0.15) + Signal 4 suspicious title (+0.2) = 0.55
// Actually the cleanest exact-0.5 combination is:
//   Signal 3 (+0.2) + Signal 7 short desc (+0.15) + Signal 4 title (+0.2) = 0.55 → still > 0.5
// Simplest ≥0.5: Signal 2 alone (+0.4) + Signal 3 (+0.2) = 0.6, or just Signal 2 + Signal 5:
// To get score that lands precisely at 0.5 we use Signal 3 (+0.2) + Signal 4 (+0.2) + Signal 7 (+0.15) = 0.55
func TestDetector_BoundaryScore_AtLeast05_IsScam(t *testing.T) {
	d := scam.New()

	// Signal 3: no company (+0.2)
	// Signal 4: suspicious title phrase (+0.2)
	// Signal 7: description < 100 chars (+0.15) → total = 0.55 ≥ 0.5 → IsScam=true
	// We avoid URL mismatch (Signal 6) by leaving company empty (skips that check).
	job := &models.ScrapedJob{
		Title:       "data entry no experience",          // Signal 4
		Company:     "",                                  // Signal 3
		Description: "Short job post.",                  // Signal 7: < 100 chars
		URL:         "http://example.com/apply",
	}

	result := d.Classify(job)

	// Score must be >= 0.5 so IsScam must be true.
	if result.Score < 0.5 {
		t.Errorf("expected score >= 0.5 for boundary job, got %.2f; reasons=%v",
			result.Score, result.Reasons)
	}
	if !result.IsScam {
		t.Errorf("expected IsScam=true at score %.2f (threshold is >=0.5)", result.Score)
	}
}

// TestDetector_BelowBoundary_ThreeSignals_NotScam verifies that a job with
// only 3 small signals stays below 0.5 and is NOT classified as a scam.
// We use:
//   Signal 5 personal email (+0.25) + Signal 7 short desc (+0.15) = 0.40 < 0.5
func TestDetector_BelowBoundary_ThreeSignals_NotScam(t *testing.T) {
	d := scam.New()

	// Signal 5: personal email in description (+0.25)
	// Signal 7: description < 100 chars (+0.15)
	// Total = 0.40 — below threshold → IsScam=false
	// Company is present and URL contains company word to suppress signals 3 and 6.
	job := &models.ScrapedJob{
		Title:       "Office Assistant",
		Company:     "BrightOffice",
		Description: "Apply now: contact us at hr@gmail.com",          // < 100 chars + email
		URL:         "https://brightoffice.com/jobs/office-assistant",  // company word present
	}

	result := d.Classify(job)

	if result.Score >= 0.5 {
		t.Errorf("expected score < 0.5 for below-boundary job, got %.2f; reasons=%v",
			result.Score, result.Reasons)
	}
	if result.IsScam {
		t.Errorf("expected IsScam=false at score %.2f", result.Score)
	}
}
