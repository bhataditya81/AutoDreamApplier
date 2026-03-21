package ats_test

import (
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/ats"
)

func TestDetectATSFromURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want string
	}{
		// greenhouse
		{name: "greenhouse boards host", url: "https://boards.greenhouse.io/stripe/jobs/123", want: "greenhouse"},
		{name: "greenhouse short link", url: "https://grnh.se/abc123", want: "greenhouse"},
		{name: "greenhouse job_app embed", url: "https://greenhouse.io/embed/job_app?for=company", want: "greenhouse"},
		{name: "greenhouse uppercase", url: "HTTPS://BOARDS.GREENHOUSE.IO/STRIPE/JOBS/123", want: "greenhouse"},

		// lever
		{name: "lever canonical", url: "https://jobs.lever.co/airbnb/abc-def", want: "lever"},
		{name: "lever direct apply", url: "https://jobs.lever.co/airbnb/abc-def/apply", want: "lever"},
		{name: "lever embedded", url: "https://apply.lever.co/jobs", want: "lever"},

		// workday
		{name: "workday myworkdayjobs", url: "https://company.myworkdayjobs.com/en-US/external", want: "workday"},
		{name: "workday wd1", url: "https://microsoft.wd1.myworkdayjobs.com/en-US/jobs/123", want: "workday"},
		{name: "workday wd3", url: "https://ibm.wd3.myworkdayjobs.com/en-US/jobs/456", want: "workday"},
		{name: "workday wd5", url: "https://amazon.wd5.myworkdayjobs.com/en-US/jobs/789", want: "workday"},

		// icims
		{name: "icims direct", url: "https://careers-company.icims.com/jobs/1234/job", want: "icims"},
		{name: "icims subdomain pattern", url: "https://jobs-icims.example.com/job", want: "icims"},

		// taleo
		{name: "taleo net", url: "https://oracle.taleo.net/careersection/2/jobdetail.ftl?job=12345", want: "taleo"},
		{name: "taleo com", url: "https://company.taleo.com/apply", want: "taleo"},

		// smartrecruiters
		{name: "smartrecruiters main", url: "https://jobs.smartrecruiters.com/company/job-id", want: "smartrecruiters"},
		{name: "smartrecruiters short", url: "https://smrtr.io/abc123", want: "smartrecruiters"},

		// bamboohr
		{name: "bamboohr careers", url: "https://company.bamboohr.com/careers/123", want: "bamboohr"},
		{name: "bamboohr jobs", url: "https://company.bamboohr.com/jobs/view.php?id=1", want: "bamboohr"},

		// successfactors
		{name: "successfactors com", url: "https://company.successfactors.com/sf/careers", want: "successfactors"},
		{name: "successfactors eu", url: "https://company.successfactors.eu/sf/careers", want: "successfactors"},

		// unknown / no match
		{name: "unknown domain", url: "https://unknown.example.com/jobs/1", want: "unknown"},
		{name: "empty string", url: "", want: "unknown"},
		{name: "arbitrary path", url: "https://jobs.acme.com/careers", want: "unknown"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ats.DetectATSFromURL(tc.url)
			if got != tc.want {
				t.Errorf("DetectATSFromURL(%q) = %q; want %q", tc.url, got, tc.want)
			}
		})
	}
}
