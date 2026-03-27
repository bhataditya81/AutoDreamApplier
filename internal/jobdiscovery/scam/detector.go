// Package scam provides a lightweight heuristic classifier for scam job listings.
package scam

import (
	"errors"
	"regexp"
	"strings"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

// errNotInt is a package-local sentinel returned by parseLeadingSalary when the
// string does not start with a valid positive integer.
var errNotInt = errors.New("not an integer")

// Result holds scam detection output.
type Result struct {
	IsScam  bool
	Score   float64  // 0.0 (clean) to 1.0 (definite scam)
	Reasons []string // human-readable flags triggered
}

// Detector classifies job listings for scam signals.
type Detector struct{}

// New returns a new Detector.
func New() *Detector { return &Detector{} }

// pre-compiled patterns used across signals.
var (
	// salary in text: "$500/hr", "$300/hr", etc.
	salaryTextRe = regexp.MustCompile(`\$\s*(\d+)\s*/\s*hr`)

	// personal / free email domains in the description.
	personalEmailRe = regexp.MustCompile(`(?i)[\w.+-]+@(gmail|yahoo|hotmail|outlook|aol|icloud|mail|protonmail)\.com`)

	// upfront payment keywords.
	upfrontKeywords = []string{
		"pay to apply", "training fee", "starter kit",
		"application fee", "registration fee",
	}

	// suspicious title fragments.
	suspiciousTitlePhrases = []string{
		"work from home earn",
		"data entry no experience",
		"make money fast",
		"easy money",
		"work at home earn",
	}

	// scam buzzwords.
	buzzwords = []string{
		"unlimited", "passive income", "be your own boss",
		"ground floor", "mlm",
	}
)

// Classify scores a job for scam signals.
func (d *Detector) Classify(job *models.ScrapedJob) Result {
	var score float64
	var reasons []string

	descLower := strings.ToLower(job.Description)
	titleLower := strings.ToLower(job.Title)
	companyLower := strings.ToLower(job.Company)

	// ── Signal 1: Suspiciously high salary (+0.3) ─────────────────────────────
	highSalaryTriggered := false
	if job.SalaryMin != nil && *job.SalaryMin > 300000 {
		highSalaryTriggered = true
	}
	if !highSalaryTriggered {
		if matches := salaryTextRe.FindAllStringSubmatch(descLower, -1); len(matches) > 0 {
			for _, m := range matches {
				if len(m) >= 2 {
					// parse the number to check if >= 200/hr (very high)
					var rate int
					if n, err := parseSimpleInt(m[1]); err == nil && n >= 200 {
						rate = n
						_ = rate
						highSalaryTriggered = true
						break
					}
				}
			}
		}
	}
	if !highSalaryTriggered && (strings.Contains(descLower, "unlimited earning") ||
		strings.Contains(descLower, "unlimited income") ||
		strings.Contains(descLower, "$500/hr") ||
		strings.Contains(descLower, "$400/hr") ||
		strings.Contains(descLower, "$300/hr")) {
		highSalaryTriggered = true
	}
	if highSalaryTriggered {
		score += 0.3
		reasons = append(reasons, "suspiciously high salary")
	}

	// ── Signal 2: Requires upfront payment (+0.4) ─────────────────────────────
	for _, kw := range upfrontKeywords {
		if strings.Contains(descLower, kw) {
			score += 0.4
			reasons = append(reasons, "requires upfront payment: "+kw)
			break
		}
	}
	// also check for "$" in description combined with "apply" nearby
	if !containsReason(reasons, "requires upfront payment") &&
		strings.Contains(descLower, "$ to apply") {
		score += 0.4
		reasons = append(reasons, "requires upfront payment: pay to apply")
	}

	// ── Signal 3: No company name (+0.2) ──────────────────────────────────────
	trimmed := strings.TrimSpace(job.Company)
	if trimmed == "" || strings.EqualFold(trimmed, "unknown") {
		score += 0.2
		reasons = append(reasons, "no company name")
	}

	// ── Signal 4: Suspicious title (+0.2) ─────────────────────────────────────
	for _, phrase := range suspiciousTitlePhrases {
		if strings.Contains(titleLower, phrase) {
			score += 0.2
			reasons = append(reasons, "suspicious title phrase: "+phrase)
			break
		}
	}

	// ── Signal 5: Personal email domain in description (+0.25) ────────────────
	if personalEmailRe.MatchString(job.Description) {
		score += 0.25
		reasons = append(reasons, "personal email domain found in description")
	}

	// ── Signal 6: URL mismatch (+0.15) ────────────────────────────────────────
	if job.URL != "" && trimmed != "" && !strings.EqualFold(trimmed, "unknown") {
		urlLower := strings.ToLower(job.URL)
		// Simplistic check: company name words not present in the URL domain
		companyWords := splitWords(companyLower)
		if len(companyWords) > 0 && !anyWordInURL(companyWords, urlLower) {
			// Only flag if company name looks like a real company (>= 3 chars)
			if len(companyWords[0]) >= 3 {
				score += 0.15
				reasons = append(reasons, "job URL domain doesn't match company name")
			}
		}
	}

	// ── Signal 7: Too vague description (+0.15) ───────────────────────────────
	if len(strings.TrimSpace(job.Description)) < 100 {
		score += 0.15
		reasons = append(reasons, "description too short (< 100 chars)")
	}

	// ── Signal 8: Buzzword overload (+0.1) ────────────────────────────────────
	buzzCount := 0
	for _, bw := range buzzwords {
		if strings.Contains(descLower, bw) {
			buzzCount++
		}
	}
	if buzzCount > 3 {
		score += 0.1
		reasons = append(reasons, "buzzword overload")
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return Result{
		IsScam:  score >= 0.5,
		Score:   score,
		Reasons: reasons,
	}
}

// ── private helpers ───────────────────────────────────────────────────────────

func containsReason(reasons []string, prefix string) bool {
	for _, r := range reasons {
		if strings.HasPrefix(r, prefix) {
			return true
		}
	}
	return false
}

// splitWords splits a company name into lowercase words, dropping short ones.
func splitWords(s string) []string {
	raw := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == ',' || r == '.'
	})
	var words []string
	for _, w := range raw {
		if len(w) >= 3 {
			words = append(words, w)
		}
	}
	return words
}

// anyWordInURL returns true if any word from companyWords appears in urlStr.
func anyWordInURL(companyWords []string, urlStr string) bool {
	for _, w := range companyWords {
		if strings.Contains(urlStr, w) {
			return true
		}
	}
	return false
}

// parseSimpleInt parses a decimal integer from s.
func parseSimpleInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 && len(s) > 0 {
		return 0, errNotInt // not actually an int; use sentinel
	}
	return n, nil
}
