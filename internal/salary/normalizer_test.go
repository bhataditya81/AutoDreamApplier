package salary

import (
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Seniority stripping.
		{"senior prefix", "Senior Software Engineer", "software engineer"},
		{"sr. prefix", "Sr. Software Engineer", "software engineer"},
		{"sr space prefix", "Sr Software Engineer", "software engineer"},
		{"junior prefix", "Junior Developer", "developer"},
		{"jr. prefix", "Jr. Developer", "developer"},
		{"jr space prefix", "Jr Developer", "developer"},
		{"staff prefix", "Staff Engineer", "engineer"},
		{"principal prefix", "Principal Engineer", "engineer"},
		{"lead prefix", "Lead Data Scientist", "data scientist"},
		{"associate prefix", "Associate Product Manager", "product manager"},
		{"mid-level prefix", "Mid-level Analyst", "analyst"},
		{"entry-level prefix", "Entry-level Engineer", "engineer"},
		{"entry level prefix", "Entry level Engineer", "engineer"},

		// Case folding.
		{"all uppercase", "SENIOR SOFTWARE ENGINEER", "software engineer"},
		{"mixed case", "sEnIoR sOftWaRe EngInEeR", "software engineer"},

		// Already normalised.
		{"already normalised", "software engineer", "software engineer"},
		{"no prefix", "Data Analyst", "data analyst"},

		// Edge cases.
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		// NBSP (\u00a0) is not a standard ASCII space; strings.TrimSpace strips it,
		// so the title is treated as "Senior Software Engineer" equivalent.
		// After ToLower and TrimSpace the NBSP is stripped; result has no seniority prefix stripped
		// because the loop checks HasPrefix("senior") on the full lowercased string.
		// The prefix "senior" is matched but the trailing char after it is \u00a0 which
		// TrimSpace does strip, so the stripped tail becomes "software engineer".
		{"unicode spaces", "Senior\u00a0Software Engineer", "software engineer"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeTitle(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeTitle(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeLocation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Location slugification.
		{"city state comma", "New York, NY", "new-york-ny"},
		{"remote", "Remote", "remote"},
		{"lowercase remote", "remote", "remote"},
		{"city state no comma", "San Francisco CA", "san-francisco-ca"},
		{"extra spaces", "  Austin, TX  ", "austin-tx"},
		{"special chars", "Washington D.C.", "washington-d-c"},
		{"multiple commas", "Chicago, IL, USA", "chicago-il-usa"},

		// Case folding.
		{"uppercase", "NEW YORK NY", "new-york-ny"},

		// Already normalised.
		{"already normalised", "new-york-ny", "new-york-ny"},

		// Edge cases.
		{"empty string", "", ""},
		{"leading/trailing hyphens after slug", "---remote---", "remote"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeLocation(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeLocation(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
