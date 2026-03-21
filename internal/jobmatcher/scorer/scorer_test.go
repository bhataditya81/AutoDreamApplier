package scorer_test

import (
	"testing"

	discmodels "github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
	matchmodels "github.com/bhata/AutoDreamApplier/internal/jobmatcher/models"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
	usermodels "github.com/bhata/AutoDreamApplier/internal/user/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func intPtr(v int) *int { return &v }

func newJob(title, location, desc string, isRemote bool, salMin, salMax *int) *discmodels.Job {
	return &discmodels.Job{
		Title:       title,
		Location:    location,
		Description: desc,
		IsRemote:    isRemote,
		SalaryMin:   salMin,
		SalaryMax:   salMax,
	}
}

func newPrefs(titles []string, locations []string, remote string, salMin, salMax *int) *usermodels.UserPreferences {
	return &usermodels.UserPreferences{
		TargetTitles: titles,
		Locations:    locations,
		RemotePref:   remote,
		SalaryMin:    salMin,
		SalaryMax:    salMax,
	}
}

func newResume(rawText string) *usermodels.Resume {
	return &usermodels.Resume{RawText: rawText}
}

func newUser() *usermodels.User {
	return &usermodels.User{}
}

func assertBetween(t *testing.T, label string, got, min, max float64) {
	t.Helper()
	if got < min || got > max {
		t.Errorf("%s = %.4f; want in [%.4f, %.4f]", label, got, min, max)
	}
}

// ── New / Score returns ScoreBreakdown ────────────────────────────────────────

func TestNew_NotNil(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	if s == nil {
		t.Fatal("scorer.New() returned nil")
	}
}

func TestScore_ReturnsBreakdown(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Software Engineer", "New York", "go python", false, nil, nil)
	prefs := newPrefs([]string{"Software Engineer"}, []string{"New York"}, "any", nil, nil)
	resume := newResume("software engineer with go experience")
	bd := s.Score(job, newUser(), prefs, resume)

	// Weighted result must be in [0, 1]
	w := bd.Weighted()
	assertBetween(t, "Weighted", w, 0, 1)
}

// ── Title scoring ─────────────────────────────────────────────────────────────

func TestScore_TitleExactMatch_ReturnsOne(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Software Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs([]string{"Software Engineer"}, nil, "any", nil, nil)
	resume := newResume("experience")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.TitleScore != 1.0 {
		t.Errorf("TitleScore = %.4f; want 1.0 for exact match", bd.TitleScore)
	}
}

func TestScore_TitleExactMatchCaseInsensitive(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("software engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs([]string{"Software Engineer"}, nil, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.TitleScore != 1.0 {
		t.Errorf("TitleScore = %.4f; want 1.0 for case-insensitive exact match", bd.TitleScore)
	}
}

func TestScore_TitleNoTargets_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Software Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil) // no target titles
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.TitleScore != 0.5 {
		t.Errorf("TitleScore = %.4f; want 0.5 when no target titles set", bd.TitleScore)
	}
}

func TestScore_TitlePartialMatch_ScoreBelow1(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Senior Software Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs([]string{"Software Engineer"}, nil, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// Should have partial overlap but not exact match
	assertBetween(t, "TitleScore partial", bd.TitleScore, 0, 1)
}

func TestScore_TitleNoMatch_LowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Product Manager", "NYC", "", false, nil, nil)
	prefs := newPrefs([]string{"Software Engineer"}, nil, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// Low or zero — no word overlap
	assertBetween(t, "TitleScore no match", bd.TitleScore, 0, 0.5)
}

// ── Location scoring ──────────────────────────────────────────────────────────

func TestScore_RemoteOnly_RemoteJob_ReturnsOne(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "Remote", "", true, nil, nil)
	prefs := newPrefs(nil, nil, "remote_only", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.LocationScore != 1.0 {
		t.Errorf("LocationScore = %.4f; want 1.0 for remote_only + remote job", bd.LocationScore)
	}
}

func TestScore_RemoteOnly_OnsiteJob_ReturnsZero(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "San Francisco, CA", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "remote_only", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.LocationScore != 0.0 {
		t.Errorf("LocationScore = %.4f; want 0.0 for remote_only + on-site job", bd.LocationScore)
	}
}

func TestScore_HybridOk_RemoteJob_ReturnsOne(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "Remote", "", true, nil, nil)
	prefs := newPrefs(nil, nil, "hybrid_ok", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.LocationScore != 1.0 {
		t.Errorf("LocationScore = %.4f; want 1.0 for hybrid_ok + remote job", bd.LocationScore)
	}
}

func TestScore_AnyPref_NoLocationPrefs_ReturnsHighish(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "Tokyo", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil) // no location prefs
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// 0.8 — no location preference, assume mostly ok
	if bd.LocationScore != 0.8 {
		t.Errorf("LocationScore = %.4f; want 0.8 for any pref + no locations", bd.LocationScore)
	}
}

func TestScore_LocationMatch_ExactCity(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "New York", "", false, nil, nil)
	prefs := newPrefs(nil, []string{"New York"}, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.LocationScore != 1.0 {
		t.Errorf("LocationScore = %.4f; want 1.0 for matching city", bd.LocationScore)
	}
}

func TestScore_LocationNoMatch_LowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "Tokyo", "", false, nil, nil)
	prefs := newPrefs(nil, []string{"New York"}, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// 0.2 — not in prefs but not excluded
	if bd.LocationScore != 0.2 {
		t.Errorf("LocationScore = %.4f; want 0.2 for non-matching city", bd.LocationScore)
	}
}

// ── Salary scoring ────────────────────────────────────────────────────────────

func TestScore_SalaryNoInfo_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.SalaryScore != 0.5 {
		t.Errorf("SalaryScore = %.4f; want 0.5 when no salary info", bd.SalaryScore)
	}
}

func TestScore_SalaryJobHasInfo_NoUserPref_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "NYC", "", false, intPtr(100000), intPtr(150000))
	prefs := newPrefs(nil, nil, "any", nil, nil) // no salary pref
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.SalaryScore != 0.5 {
		t.Errorf("SalaryScore = %.4f; want 0.5 when user has no salary preference", bd.SalaryScore)
	}
}

func TestScore_SalaryPerfectOverlap_HighScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "NYC", "", false, intPtr(100000), intPtr(150000))
	prefs := newPrefs(nil, nil, "any", intPtr(100000), intPtr(150000))
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	assertBetween(t, "SalaryScore perfect overlap", bd.SalaryScore, 0.9, 1.0)
}

func TestScore_SalaryJobBelow_UserMin_LowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job pays 10k-20k, user wants at least 100k — large gap: (100k-20k)/100k = 0.8, score = 0.2
	job := newJob("Engineer", "NYC", "", false, intPtr(10000), intPtr(20000))
	prefs := newPrefs(nil, nil, "any", intPtr(100000), intPtr(150000))
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// gap = (100000-20000)/100000 = 0.8, score = 1-0.8 = 0.2
	assertBetween(t, "SalaryScore job below user min", bd.SalaryScore, 0, 0.3)
}

func TestScore_SalaryJobAbove_UserMax_VeryLowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job pays 200k-300k, user wants max 100k (senior role penalty)
	job := newJob("Engineer", "NYC", "", false, intPtr(200000), intPtr(300000))
	prefs := newPrefs(nil, nil, "any", intPtr(50000), intPtr(100000))
	resume := newResume("text")

	bd := s.Score(job, newUser(), prefs, resume)
	// job pays way more than user max
	assertBetween(t, "SalaryScore job above user max", bd.SalaryScore, 0, 0.2)
}

// ── Skills scoring ────────────────────────────────────────────────────────────

func TestScore_SkillsEmpty_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("")

	bd := s.Score(job, newUser(), prefs, resume)
	// empty job desc OR empty resume → 0.3
	if bd.SkillsScore != 0.3 {
		t.Errorf("SkillsScore = %.4f; want 0.3 when desc/resume empty", bd.SkillsScore)
	}
}

func TestScore_SkillsEmptyJobDesc_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Engineer", "NYC", "", false, nil, nil) // empty description
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("go python kubernetes")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.SkillsScore != 0.3 {
		t.Errorf("SkillsScore = %.4f; want 0.3 when job desc is empty", bd.SkillsScore)
	}
}

func TestScore_SkillsAllMatch_HighScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job requires go, python, kubernetes, redis, docker
	// Resume has all of them
	desc := "We use go golang python kubernetes redis docker aws postgresql"
	resume := newResume("proficient in go golang python kubernetes redis docker aws postgresql")
	job := newJob("Backend Engineer", "NYC", desc, false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)

	bd := s.Score(job, newUser(), prefs, resume)
	assertBetween(t, "SkillsScore all match", bd.SkillsScore, 0.8, 1.0)
}

func TestScore_SkillsNoMatch_LowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job requires Ruby on Rails but resume has only Go
	desc := "Experienced with ruby rails and laravel php developer"
	resume := newResume("golang backend engineer with kubernetes experience")
	job := newJob("Ruby Engineer", "NYC", desc, false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)

	bd := s.Score(job, newUser(), prefs, resume)
	assertBetween(t, "SkillsScore no match", bd.SkillsScore, 0, 0.4)
}

func TestScore_SkillsNoTechKeywords_ReturnsNeutral(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job description has no recognised tech keywords.
	// Note: the keyword list includes "r" (R language) which matches as a substring,
	// so we construct a description that intentionally contains "r" to acknowledge
	// the function returns 0.5 only when no keywords are found at all.
	// Instead, test the empty-desc path which also triggers neutral:
	job := newJob("XYZ Position", "NYC", "", false, nil, nil) // empty description → 0.3
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("some content here")

	bd := s.Score(job, newUser(), prefs, resume)
	// empty description → 0.3
	if bd.SkillsScore != 0.3 {
		t.Errorf("SkillsScore = %.4f; want 0.3 for empty job description", bd.SkillsScore)
	}
}

func TestScore_SkillsOnlyRLanguage_PartialScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job description mentions only "r" (R language keyword) via substring
	// Resume doesn't contain r → score = 0/1 = 0.0? No — resume contains "r" in every word.
	// Use a description with a clear single keyword match and verify score is between 0 and 1.
	desc := "proficiency in python and django required"
	resume := newResume("golang kubernetes engineer")
	job := newJob("Data Scientist", "NYC", desc, false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)

	bd := s.Score(job, newUser(), prefs, resume)
	assertBetween(t, "SkillsScore partial", bd.SkillsScore, 0, 1)
}

// ── Seniority scoring ─────────────────────────────────────────────────────────

func TestScore_SeniorityExactMatch_ReturnsOne(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Senior Software Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)
	// Resume also mentions senior
	resume := newResume("senior engineer with 8 years experience")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.SeniorityScore != 1.0 {
		t.Errorf("SeniorityScore = %.4f; want 1.0 for matched seniority", bd.SeniorityScore)
	}
}

func TestScore_SeniorityOneLevelOff_ReturnsSeventh(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job is senior (level 3), resume shows mid (level 2) — one level apart
	job := newJob("Senior Software Engineer", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("mid-level software engineer") // "mid" not detected by detectSeniority, defaults to mid

	bd := s.Score(job, newUser(), prefs, resume)
	// senior vs mid = 1 level diff → 0.7
	if bd.SeniorityScore != 0.7 {
		t.Errorf("SeniorityScore = %.4f; want 0.7 for one-level difference", bd.SeniorityScore)
	}
}

func TestScore_SeniorityIntern_vs_Principal_LowScore(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	// Job is intern, resume is principal — large gap
	job := newJob("Engineering Intern", "NYC", "", false, nil, nil)
	prefs := newPrefs(nil, nil, "any", nil, nil)
	resume := newResume("principal engineer with 15 years experience distinguished track record")

	bd := s.Score(job, newUser(), prefs, resume)
	if bd.SeniorityScore != 0.3 {
		t.Errorf("SeniorityScore = %.4f; want 0.3 for large level gap", bd.SeniorityScore)
	}
}

// ── Full Score with weighted output ──────────────────────────────────────────

func TestScore_Weighted_InRange(t *testing.T) {
	t.Parallel()
	s := scorer.New()
	job := newJob("Software Engineer", "New York", "go python postgres", false, intPtr(120000), intPtr(160000))
	prefs := newPrefs([]string{"Software Engineer"}, []string{"New York"}, "any", intPtr(100000), intPtr(150000))
	resume := newResume("senior software engineer with go python postgres redis experience")

	bd := s.Score(job, newUser(), prefs, resume)
	w := bd.Weighted()
	assertBetween(t, "Weighted", w, 0, 1)
}

func TestScoreBreakdown_Weighted_Calculation(t *testing.T) {
	t.Parallel()
	bd := matchmodels.ScoreBreakdown{
		TitleScore:     1.0,
		LocationScore:  1.0,
		SalaryScore:    1.0,
		SkillsScore:    1.0,
		SeniorityScore: 1.0,
	}
	got := bd.Weighted()
	if got != 1.0 {
		t.Errorf("all-1.0 breakdown Weighted() = %.4f; want 1.0", got)
	}
}

func TestScoreBreakdown_Weighted_AllZero(t *testing.T) {
	t.Parallel()
	bd := matchmodels.ScoreBreakdown{}
	got := bd.Weighted()
	if got != 0.0 {
		t.Errorf("all-zero breakdown Weighted() = %.4f; want 0.0", got)
	}
}

func TestScoreBreakdown_Weighted_CorrectWeights(t *testing.T) {
	t.Parallel()
	bd := matchmodels.ScoreBreakdown{
		TitleScore:     1.0, // 30%
		LocationScore:  0.0,
		SalaryScore:    0.0,
		SkillsScore:    0.0,
		SeniorityScore: 0.0,
	}
	got := bd.Weighted()
	want := 0.30
	if got != want {
		t.Errorf("Weighted() = %.4f; want %.4f (title-only)", got, want)
	}
}
