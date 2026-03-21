package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/ai"
)

// ─── NewClient ────────────────────────────────────────────────────────────────

func TestNewClient_ReturnsNonNil(t *testing.T) {
	c := ai.NewClient("http://localhost:8000")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

// ─── TailorResume ─────────────────────────────────────────────────────────────

func TestClient_TailorResume_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/resume/tailor" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ai.ResumeTailorResponse{
			TailoredText:  "tailored resume text",
			ChangesMade:   []string{"added keyword Go"},
			KeywordsAdded: []string{"Go"},
		})
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	resp, err := c.TailorResume(context.Background(), &ai.ResumeTailorRequest{
		ResumeText:     "original resume",
		JobTitle:       "Software Engineer",
		JobDescription: "Looking for Go developer",
		Mode:           "keyword_inject",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TailoredText != "tailored resume text" {
		t.Errorf("expected 'tailored resume text', got %q", resp.TailoredText)
	}
	if len(resp.KeywordsAdded) != 1 || resp.KeywordsAdded[0] != "Go" {
		t.Errorf("unexpected keywords: %v", resp.KeywordsAdded)
	}
}

func TestClient_TailorResume_Server500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	_, err := c.TailorResume(context.Background(), &ai.ResumeTailorRequest{
		ResumeText: "test",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// ─── GenerateCoverLetter ──────────────────────────────────────────────────────

func TestClient_GenerateCoverLetter_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cover-letter/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ai.CoverLetterResponse{
			CoverLetter: "Dear Hiring Manager, ...",
			WordCount:   50,
		})
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	resp, err := c.GenerateCoverLetter(context.Background(), &ai.CoverLetterRequest{
		ResumeText:  "my resume",
		JobTitle:    "Engineer",
		CompanyName: "ACME Corp",
		Tone:        "professional",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CoverLetter == "" {
		t.Error("expected non-empty cover letter")
	}
	if resp.WordCount != 50 {
		t.Errorf("expected word count 50, got %d", resp.WordCount)
	}
}

func TestClient_GenerateCoverLetter_Server500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	_, err := c.GenerateCoverLetter(context.Background(), &ai.CoverLetterRequest{
		ResumeText: "test",
	})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// ─── AnswerFormQuestion ───────────────────────────────────────────────────────

func TestClient_AnswerFormQuestion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/form-qa/answer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ai.FormQAResponse{
			Answer:     "5 years",
			Confidence: 0.95,
		})
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	resp, err := c.AnswerFormQuestion(context.Background(), &ai.FormQARequest{
		Question:   "How many years of experience do you have?",
		ResumeText: "5 years at various companies",
		JobTitle:   "Engineer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != "5 years" {
		t.Errorf("expected answer '5 years', got %q", resp.Answer)
	}
	if resp.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", resp.Confidence)
	}
}

func TestClient_AnswerFormQuestion_Server500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	_, err := c.AnswerFormQuestion(context.Background(), &ai.FormQARequest{
		Question: "Why do you want to work here?",
	})
	if err == nil {
		t.Fatal("expected error for 503 response, got nil")
	}
}

// ─── HealthCheck ──────────────────────────────────────────────────────────────

func TestClient_HealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	if err := c.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error for unhealthy service, got nil")
	}
}

func TestClient_HealthCheck_NetworkError(t *testing.T) {
	// Point to a port with nothing listening
	c := ai.NewClient("http://127.0.0.1:19997")
	err := c.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

// TestClient_TailorResume_InvalidJSONResponse tests that a 200 with invalid JSON returns error.
func TestClient_TailorResume_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not json {{{"))
	}))
	defer srv.Close()

	c := ai.NewClient(srv.URL)
	_, err := c.TailorResume(context.Background(), &ai.ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
}
