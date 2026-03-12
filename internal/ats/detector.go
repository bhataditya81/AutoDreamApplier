package ats

import (
	"strings"
)

// DetectATSFromURL attempts to identify the ATS type from a URL string.
// This is a fast, heuristic-based detection. Plugins can override with DOM inspection.
func DetectATSFromURL(url string) string {
	urlLower := strings.ToLower(url)

	patterns := map[string][]string{
		"greenhouse": {
			"boards.greenhouse.io",
			"grnh.se",
			"job_app",
		},
		"lever": {
			"jobs.lever.co",
			"lever.co/",
		},
		"workday": {
			"myworkdayjobs.com",
			"wd1.myworkdayjobs",
			"wd3.myworkdayjobs",
			"wd5.myworkdayjobs",
		},
		"icims": {
			"icims.com",
			"jobs-icims",
		},
		"taleo": {
			"taleo.net",
			"taleo.com",
		},
		"successfactors": {
			"successfactors.com",
			"successfactors.eu",
		},
		"smartrecruiters": {
			"smartrecruiters.com",
			"smrtr.io",
		},
		"bamboohr": {
			"bamboohr.com/careers",
			"bamboohr.com/jobs",
		},
	}

	for atsType, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(urlLower, keyword) {
				return atsType
			}
		}
	}

	return "unknown"
}
