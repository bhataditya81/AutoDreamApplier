package scorer

import (
	"math"
	"strings"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	matchmodels "github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	usermodels "github.com/bhata/AutoDreamApplier/internal/user/models"
)

// Scorer calculates how well a job matches a user's preferences.
type Scorer struct{}

// New creates a new Scorer.
func New() *Scorer { return &Scorer{} }

// Score returns a breakdown of how well the job matches the user's preferences.
func (s *Scorer) Score(job *discmodels.Job, user *usermodels.User, prefs *usermodels.UserPreferences, resume *usermodels.Resume) matchmodels.ScoreBreakdown {
	bd := matchmodels.ScoreBreakdown{}

	bd.TitleScore = s.scoreTitle(job.Title, prefs.TargetTitles)
	bd.LocationScore = s.scoreLocation(job, prefs)
	bd.SalaryScore = s.scoreSalary(job, prefs)
	bd.SkillsScore = s.scoreSkills(job.Description, resume.RawText)
	bd.SeniorityScore = s.scoreSeniority(job.Title, resume.RawText)

	return bd
}

// scoreTitle checks if the job title matches any of the user's target titles.
// Returns 1.0 for exact match, 0.7 for partial match, 0.3 for adjacent role, 0 for no match.
func (s *Scorer) scoreTitle(jobTitle string, targetTitles []string) float64 {
	if len(targetTitles) == 0 {
		return 0.5 // neutral if no preference
	}

	jobTitleLower := strings.ToLower(jobTitle)
	bestScore := 0.0

	for _, target := range targetTitles {
		targetLower := strings.ToLower(target)

		// Exact match
		if jobTitleLower == targetLower {
			return 1.0
		}

		// Partial match — both contain each other's words
		targetWords := tokenize(targetLower)
		jobWords := tokenize(jobTitleLower)
		overlap := wordOverlap(jobWords, targetWords)

		if overlap > bestScore {
			bestScore = overlap
		}
	}

	// Penalty for senior/junior mismatch (handled separately in seniority score)
	return bestScore
}

// scoreLocation returns 1.0 for remote/matching, 0.0 for excluded location.
func (s *Scorer) scoreLocation(job *discmodels.Job, prefs *usermodels.UserPreferences) float64 {
	switch prefs.RemotePref {
	case "remote_only":
		if job.IsRemote {
			return 1.0
		}
		return 0.0
	case "hybrid_ok":
		if job.IsRemote {
			return 1.0
		}
		// Check if location matches preferences
		return s.locationMatch(job.Location, prefs.Locations)
	default: // "any" or "on_site"
		return s.locationMatch(job.Location, prefs.Locations)
	}
}

// locationMatch returns the fraction of matching locations.
func (s *Scorer) locationMatch(jobLocation string, prefLocations []string) float64 {
	if len(prefLocations) == 0 {
		return 0.8 // no location preference — assume mostly ok
	}

	jobLocLower := strings.ToLower(jobLocation)
	for _, pref := range prefLocations {
		prefLower := strings.ToLower(pref)
		if strings.EqualFold(prefLower, "remote") && strings.Contains(jobLocLower, "remote") {
			return 1.0
		}
		if strings.Contains(jobLocLower, prefLower) || strings.Contains(prefLower, jobLocLower) {
			return 1.0
		}
	}
	return 0.2 // location not in prefs but not excluded
}

// scoreSalary returns 1.0 if the job salary overlaps well with user's range.
func (s *Scorer) scoreSalary(job *discmodels.Job, prefs *usermodels.UserPreferences) float64 {
	// If no salary info in either direction, neutral score
	if job.SalaryMin == nil && job.SalaryMax == nil {
		return 0.5
	}
	if prefs.SalaryMin == nil && prefs.SalaryMax == nil {
		return 0.5
	}

	jobMin := 0
	if job.SalaryMin != nil {
		jobMin = *job.SalaryMin
	}
	jobMax := jobMin
	if job.SalaryMax != nil {
		jobMax = *job.SalaryMax
	}

	userMin := 0
	if prefs.SalaryMin != nil {
		userMin = *prefs.SalaryMin
	}
	userMax := userMin + 50000 // assume +50k if no max provided
	if prefs.SalaryMax != nil {
		userMax = *prefs.SalaryMax
	}

	// Check overlap between [jobMin, jobMax] and [userMin, userMax]
	overlapStart := math.Max(float64(jobMin), float64(userMin))
	overlapEnd := math.Min(float64(jobMax), float64(userMax))

	if overlapEnd < overlapStart {
		// No overlap — check how far off
		if jobMax < userMin {
			gap := float64(userMin-jobMax) / float64(userMin)
			return math.Max(0, 1.0-gap)
		}
		return 0.1 // job pays way more than user max — probably senior role
	}

	// Calculate overlap ratio
	userRange := float64(userMax - userMin)
	if userRange == 0 {
		return 1.0
	}
	overlapRatio := (overlapEnd - overlapStart) / userRange
	return math.Min(1.0, overlapRatio)
}

// scoreSkills computes keyword overlap between the job description and resume.
func (s *Scorer) scoreSkills(jobDesc, resumeText string) float64 {
	if jobDesc == "" || resumeText == "" {
		return 0.3
	}

	// Extract meaningful tech keywords from job description
	techKeywords := extractTechKeywords(strings.ToLower(jobDesc))
	if len(techKeywords) == 0 {
		return 0.5
	}

	resumeLower := strings.ToLower(resumeText)
	matchCount := 0
	for _, kw := range techKeywords {
		if strings.Contains(resumeLower, kw) {
			matchCount++
		}
	}

	return math.Min(1.0, float64(matchCount)/float64(len(techKeywords)))
}

// scoreSeniority checks if the job seniority level matches the resume.
func (s *Scorer) scoreSeniority(jobTitle, resumeText string) float64 {
	jobLower := strings.ToLower(jobTitle)
	resumeLower := strings.ToLower(resumeText)

	jobLevel := detectSeniority(jobLower)
	resumeLevel := detectSeniority(resumeLower)

	if jobLevel == resumeLevel {
		return 1.0
	}
	// One level difference — acceptable
	if math.Abs(float64(jobLevel-resumeLevel)) == 1 {
		return 0.7
	}
	return 0.3
}

// ── Keyword extraction ──────────────────────────────────────────────────────

// techKeywordList is a curated list of common tech keywords for matching.
var techKeywordList = []string{
	// Languages
	"go", "golang", "python", "typescript", "javascript", "java", "kotlin",
	"rust", "ruby", "scala", "c++", "c#", "swift", "php", "r",
	// Frameworks
	"react", "next.js", "vue", "angular", "node.js", "fastapi", "django",
	"flask", "spring", "rails", "laravel", "gin", "echo", "chi",
	// Databases
	"postgresql", "postgres", "mysql", "mongodb", "redis", "cassandra",
	"dynamodb", "elasticsearch", "sqlite", "snowflake", "bigquery",
	// Cloud
	"aws", "gcp", "azure", "kubernetes", "k8s", "docker", "terraform",
	"pulumi", "ecs", "eks", "lambda", "s3", "sqs", "rds",
	// Practices
	"microservices", "rest", "grpc", "graphql", "event-driven", "kafka",
	"rabbitmq", "ci/cd", "devops", "agile", "tdd", "scrum",
	// Tools
	"git", "github", "gitlab", "jira", "datadog", "prometheus", "grafana",
}

// extractTechKeywords finds tech keywords in a job description.
func extractTechKeywords(text string) []string {
	var found []string
	for _, kw := range techKeywordList {
		if strings.Contains(text, kw) {
			found = append(found, kw)
		}
	}
	return found
}

// ── Seniority detection ────────────────────────────────────────────────────

type seniorityLevel int

const (
	levelIntern seniorityLevel = iota
	levelJunior
	levelMid
	levelSenior
	levelStaff
	levelPrincipal
)

// detectSeniority infers a seniority level from text.
func detectSeniority(text string) seniorityLevel {
	switch {
	case contains(text, "intern", "internship", "graduate"):
		return levelIntern
	case contains(text, "junior", "associate", "entry"):
		return levelJunior
	case contains(text, "senior", "sr."):
		return levelSenior
	case contains(text, "staff", "lead"):
		return levelStaff
	case contains(text, "principal", "distinguished"):
		return levelPrincipal
	default:
		return levelMid
	}
}

// ── String helpers ─────────────────────────────────────────────────────────

// tokenize splits text into lowercase words, filtering short/common words.
func tokenize(s string) []string {
	words := strings.Fields(s)
	var tokens []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()")
		if len(w) > 2 && !isStopWord(w) {
			tokens = append(tokens, strings.ToLower(w))
		}
	}
	return tokens
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "our": true,
	"you": true, "are": true, "will": true, "have": true, "this": true,
	"that": true, "from": true, "your": true, "not": true, "but": true,
}

func isStopWord(w string) bool { return stopWords[w] }

// wordOverlap returns the Jaccard similarity between two token sets.
func wordOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, w := range a {
		setA[w] = true
	}
	intersection := 0
	for _, w := range b {
		if setA[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// contains checks if text contains any of the given substrings.
func contains(text string, subs ...string) bool {
	for _, s := range subs {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}
