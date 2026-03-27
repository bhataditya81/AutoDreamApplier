package salary

import (
	"regexp"
	"strings"
)

// seniorityPrefixes are stripped before keying.
var seniorityPrefixes = []string{
	"senior", "sr.", "sr ", "junior", "jr.", "jr ",
	"staff", "principal", "lead", "associate", "mid-level",
	"entry-level", "entry level",
}

// multiSpaceRe collapses consecutive spaces.
var multiSpaceRe = regexp.MustCompile(`\s+`)

// NormalizeTitle returns a lowercase, seniority-stripped title key.
// "Senior Software Engineer" → "software engineer"
func NormalizeTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	for _, prefix := range seniorityPrefixes {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
		}
	}
	return multiSpaceRe.ReplaceAllString(s, " ")
}

// locationSlugRe replaces any run of non-alphanumeric characters with a hyphen.
var locationSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeLocation returns a lowercase hyphenated location key.
// "New York, NY" → "new-york-ny", "Remote" → "remote"
func NormalizeLocation(loc string) string {
	s := strings.ToLower(strings.TrimSpace(loc))
	s = locationSlugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
