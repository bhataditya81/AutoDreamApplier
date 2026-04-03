package scrapers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/jobdiscovery/models"
)

func TestEnrichATSType(t *testing.T) {
	// 1. Target ATS server (e.g. Lever)
	atsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Lever Apply Page"))
	}))
	defer atsServer.Close()

	// 2. Intermediate Indeed Redirect Server (simulating the /applystart?continue=... jump)
	indeedRedirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://www.indeed.com/applystart?continue="+url.QueryEscape(atsServer.URL), http.StatusFound)
	}))
	defer indeedRedirectServer.Close()

	// 3. Initial Indeed Wrapper Server
	indeedWrapperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, indeedRedirectServer.URL, http.StatusFound)
	}))
	defer indeedWrapperServer.Close()

	// 4. Dead-end server (simulating no redirect, just standard indeed.com job)
	deadEndServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Standard Indeed Apply"))
	}))
	defer deadEndServer.Close()

	s := NewIndeedScraper().(*indeedScraper)

	tests := []struct {
		name        string
		initialURL  string
		expectedURL string // What we expect ApplyURL to become
	}{
		{
			name:        "Successful Redirect to ATS",
			initialURL:  indeedWrapperServer.URL,
			expectedURL: atsServer.URL,
		},
		{
			name:        "No Redirect (Standard Indeed)",
			initialURL:  deadEndServer.URL,
			expectedURL: deadEndServer.URL,
		},
		{
			name:        "Empty Initial URL",
			initialURL:  "",
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.ScrapedJob{
				Title:    "Test Job",
				ApplyURL: tt.initialURL,
			}

			s.enrichATSType(job)

			if job.ApplyURL != tt.expectedURL {
				t.Errorf("enrichATSType() = %v, want %v", job.ApplyURL, tt.expectedURL)
			}
		})
	}
}
