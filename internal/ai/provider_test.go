package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/ai"
)

// ─── NewProvider factory ──────────────────────────────────────────────────────

func TestNewProvider_Python_Empty(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{Provider: "", ServiceURL: "http://localhost:8000"})
	if err != nil {
		t.Fatalf("unexpected error for python provider: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	// Should be a *Client
	if _, ok := p.(*ai.Client); !ok {
		t.Errorf("expected *ai.Client, got %T", p)
	}
}

func TestNewProvider_Python_Explicit(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{Provider: "python", ServiceURL: "http://localhost:8000"})
	if err != nil {
		t.Fatalf("unexpected error for explicit python provider: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := p.(*ai.Client); !ok {
		t.Errorf("expected *ai.Client, got %T", p)
	}
}

func TestNewProvider_Anthropic_Success(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "anthropic",
		AnthropicKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error for anthropic: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := p.(*ai.AnthropicProvider); !ok {
		t.Errorf("expected *ai.AnthropicProvider, got %T", p)
	}
}

func TestNewProvider_Anthropic_MissingKey(t *testing.T) {
	_, err := ai.NewProvider(ai.ProviderConfig{Provider: "anthropic"})
	if err == nil {
		t.Fatal("expected error when anthropic key is missing")
	}
}

func TestNewProvider_Anthropic_DefaultModel(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "anthropic",
		AnthropicKey: "key",
		// AnthropicModel intentionally left empty
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewProvider_Gemini_Success(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "gemini",
		GeminiAPIKey: "gemini-key",
	})
	if err != nil {
		t.Fatalf("unexpected error for gemini: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := p.(*ai.GeminiProvider); !ok {
		t.Errorf("expected *ai.GeminiProvider, got %T", p)
	}
}

func TestNewProvider_Gemini_MissingKey(t *testing.T) {
	_, err := ai.NewProvider(ai.ProviderConfig{Provider: "gemini"})
	if err == nil {
		t.Fatal("expected error when gemini key is missing")
	}
}

func TestNewProvider_Gemini_DefaultModel(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "gemini",
		GeminiAPIKey: "key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewProvider_OpenAI_Success(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "openai",
		OpenAIAPIKey: "openai-key",
	})
	if err != nil {
		t.Fatalf("unexpected error for openai: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := p.(*ai.OpenAIProvider); !ok {
		t.Errorf("expected *ai.OpenAIProvider, got %T", p)
	}
}

func TestNewProvider_OpenAI_MissingKey(t *testing.T) {
	_, err := ai.NewProvider(ai.ProviderConfig{Provider: "openai"})
	if err == nil {
		t.Fatal("expected error when openai key is missing")
	}
}

func TestNewProvider_OpenAI_DefaultModel(t *testing.T) {
	p, err := ai.NewProvider(ai.ProviderConfig{
		Provider:     "openai",
		OpenAIAPIKey: "key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewProvider_Unknown_ReturnsError(t *testing.T) {
	_, err := ai.NewProvider(ai.ProviderConfig{Provider: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

// ─── AnthropicProvider.complete via mock HTTP ─────────────────────────────────
// We test TailorResume/GenerateCoverLetter/AnswerFormQuestion because they call complete().

// We can't inject httpClient into AnthropicProvider/OpenAIProvider/GeminiProvider
// because those fields are private. The providers call real URLs with the private
// httpClient. We therefore test the constructors and the error paths that don't
// require network calls, and use a proxy approach via the Python client (whose
// URL IS configurable) for integration testing of the interface contract.

func TestNewAnthropicProvider_Constructor(t *testing.T) {
	p := ai.NewAnthropicProvider("test-api-key", "claude-3-haiku-20240307")
	if p == nil {
		t.Fatal("expected non-nil AnthropicProvider")
	}
}

func TestNewOpenAIProvider_Constructor(t *testing.T) {
	p := ai.NewOpenAIProvider("test-api-key", "gpt-4o-mini")
	if p == nil {
		t.Fatal("expected non-nil OpenAIProvider")
	}
}

func TestNewGeminiProvider_Constructor(t *testing.T) {
	p := ai.NewGeminiProvider("test-api-key", "gemini-1.5-flash")
	if p == nil {
		t.Fatal("expected non-nil GeminiProvider")
	}
}

// ─── AnthropicProvider — test error handling via httptest ────────────────────
// Since the URL is hardcoded, we test error handling by using the Python client
// as a proxy; but for direct provider error testing we use the provider against
// a mock server that matches the Anthropic API URL through a custom http.Client.
// The cleanest approach: create a test server and set an env variable...
// Since that's not possible with the current architecture, we document and test
// what we can: constructors succeed, Provider interface is satisfied.

// TestAnthropicProvider_ImplementsProvider verifies the interface is satisfied.
func TestAnthropicProvider_ImplementsProvider(t *testing.T) {
	var _ ai.Provider = ai.NewAnthropicProvider("key", "model")
}

func TestOpenAIProvider_ImplementsProvider(t *testing.T) {
	var _ ai.Provider = ai.NewOpenAIProvider("key", "model")
}

func TestGeminiProvider_ImplementsProvider(t *testing.T) {
	var _ ai.Provider = ai.NewGeminiProvider("key", "model")
}

// ─── Python client as Provider (interface contract tests with mock) ────────────

func TestProvider_PythonClient_TailorResume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ResumeTailorResponse{
			TailoredText:  "tailored",
			ChangesMade:   []string{"keyword injection"},
			KeywordsAdded: []string{"Python", "Go"},
		})
	}))
	defer srv.Close()

	var p ai.Provider = ai.NewClient(srv.URL)
	resp, err := p.TailorResume(context.Background(), &ai.ResumeTailorRequest{
		ResumeText: "resume", JobTitle: "SWE", JobDescription: "Go Python",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TailoredText != "tailored" {
		t.Errorf("got %q", resp.TailoredText)
	}
}

func TestProvider_PythonClient_GenerateCoverLetter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.CoverLetterResponse{
			CoverLetter: "Dear Hiring Manager",
			WordCount:   3,
		})
	}))
	defer srv.Close()

	var p ai.Provider = ai.NewClient(srv.URL)
	resp, err := p.GenerateCoverLetter(context.Background(), &ai.CoverLetterRequest{
		ResumeText: "resume", JobTitle: "SWE", CompanyName: "Corp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CoverLetter == "" {
		t.Error("expected non-empty cover letter")
	}
}

func TestProvider_PythonClient_AnswerFormQuestion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.FormQAResponse{
			Answer:     "Yes, I have 5 years of experience",
			Confidence: 0.9,
		})
	}))
	defer srv.Close()

	var p ai.Provider = ai.NewClient(srv.URL)
	resp, err := p.AnswerFormQuestion(context.Background(), &ai.FormQARequest{
		Question:   "Do you have Go experience?",
		ResumeText: "5 years Go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer == "" {
		t.Error("expected non-empty answer")
	}
}
