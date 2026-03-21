// White-box tests for provider complete() methods.
// Uses package ai (not ai_test) so we can access private fields via test helpers.
// We intercept HTTP calls by replacing the private httpClient with one that
// redirects to a test server using a custom RoundTripper.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// redirectTransport is a RoundTripper that redirects all requests to a fixed base URL.
type redirectTransport struct {
	baseURL string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace scheme+host with test server URL, keep path+query.
	newURL := t.baseURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("redirectTransport: %w", err)
	}
	for k, vs := range req.Header {
		for _, v := range vs {
			newReq.Header.Add(k, v)
		}
	}
	return http.DefaultTransport.RoundTrip(newReq)
}

// setHTTPClient replaces the httpClient on a provider via white-box access.
// Each provider has an httpClient field — we set it here.

func setAnthropicClient(p *AnthropicProvider, c *http.Client) {
	p.httpClient = c
}

func setOpenAIClient(p *OpenAIProvider, c *http.Client) {
	p.httpClient = c
}

func setGeminiClient(p *GeminiProvider, c *http.Client) {
	p.httpClient = c
}

// ─── AnthropicProvider.complete ───────────────────────────────────────────────

func TestAnthropicProvider_complete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "tailored resume content here"}},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText:     "original resume",
		JobTitle:       "Engineer",
		CompanyName:    "ACME",
		JobDescription: "Go developer wanted",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TailoredText != "tailored resume content here" {
		t.Errorf("expected tailored text, got %q", resp.TailoredText)
	}
}

func TestAnthropicProvider_complete_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "invalid API key"},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("bad-key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
}

func TestAnthropicProvider_complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for HTTP 429, got nil")
	}
}

func TestAnthropicProvider_complete_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{Content: nil})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
}

func TestAnthropicProvider_GenerateCoverLetter_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "Dear Hiring Manager, I am excited..."}},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText:     "resume",
		JobTitle:       "Engineer",
		CompanyName:    "ACME",
		JobDescription: "Go dev",
		Tone:           "professional",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CoverLetter == "" {
		t.Error("expected non-empty cover letter")
	}
	if resp.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
}

func TestAnthropicProvider_AnswerFormQuestion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "5 years of Go experience"}},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question:   "How much Go experience?",
		ResumeText: "resume",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer == "" {
		t.Error("expected non-empty answer")
	}
	if resp.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", resp.Confidence)
	}
}

// ─── OpenAIProvider.complete ──────────────────────────────────────────────────

func TestOpenAIProvider_complete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "GPT tailored resume"}}},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText:     "original",
		JobTitle:       "Engineer",
		CompanyName:    "Corp",
		JobDescription: "Python dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TailoredText != "GPT tailored resume" {
		t.Errorf("unexpected result: %q", resp.TailoredText)
	}
}

func TestOpenAIProvider_complete_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "invalid API key"},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("bad-key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
}

func TestOpenAIProvider_complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

func TestOpenAIProvider_complete_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{Choices: nil})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestOpenAIProvider_GenerateCoverLetter_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Dear Hiring Manager openai"}}},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText: "resume", JobTitle: "Eng", CompanyName: "Corp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
}

func TestOpenAIProvider_AnswerFormQuestion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "3 years"}}},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question: "Years of experience?", ResumeText: "3 years",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != "3 years" {
		t.Errorf("expected '3 years', got %q", resp.Answer)
	}
}

// ─── GeminiProvider.complete ──────────────────────────────────────────────────

func TestGeminiProvider_complete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{{Content: struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}{Parts: []struct {
				Text string `json:"text"`
			}{{Text: "Gemini tailored resume"}}}}},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText:     "original",
		JobTitle:       "Engineer",
		CompanyName:    "Corp",
		JobDescription: "Gemini dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TailoredText != "Gemini tailored resume" {
		t.Errorf("unexpected result: %q", resp.TailoredText)
	}
}

func TestGeminiProvider_complete_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "API key invalid"},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider("bad-key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for API error response, got nil")
	}
}

func TestGeminiProvider_complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"forbidden"}}`))
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for HTTP 403, got nil")
	}
}

func TestGeminiProvider_complete_EmptyCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{Candidates: nil})
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.TailorResume(context.Background(), &ResumeTailorRequest{
		ResumeText: "resume",
		JobTitle:   "Engineer",
	})
	if err == nil {
		t.Fatal("expected error for empty candidates, got nil")
	}
}

func TestGeminiProvider_GenerateCoverLetter_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{{Content: struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}{Parts: []struct {
				Text string `json:"text"`
			}{{Text: "Dear Gemini Hiring Manager words here"}}}}},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText: "resume", JobTitle: "Eng", CompanyName: "Corp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
}

func TestAnthropicProvider_GenerateCoverLetter_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText: "resume", JobTitle: "Eng", CompanyName: "Corp", Tone: "professional",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestAnthropicProvider_AnswerFormQuestion_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p := NewAnthropicProvider("key", "claude-3-haiku-20240307")
	setAnthropicClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question: "Years?", ResumeText: "resume",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestOpenAIProvider_GenerateCoverLetter_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText: "resume", JobTitle: "Eng", CompanyName: "Corp",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestOpenAIProvider_AnswerFormQuestion_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("key", "gpt-4o-mini")
	setOpenAIClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question: "Years?", ResumeText: "resume",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestGeminiProvider_GenerateCoverLetter_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.GenerateCoverLetter(context.Background(), &CoverLetterRequest{
		ResumeText: "resume", JobTitle: "Eng", CompanyName: "Corp",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestGeminiProvider_AnswerFormQuestion_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	_, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question: "Years?", ResumeText: "resume",
	})
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestGeminiProvider_AnswerFormQuestion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{{Content: struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}{Parts: []struct {
				Text string `json:"text"`
			}{{Text: "4 years experience"}}}}},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider("key", "gemini-1.5-flash")
	setGeminiClient(p, &http.Client{Transport: &redirectTransport{baseURL: srv.URL}})

	resp, err := p.AnswerFormQuestion(context.Background(), &FormQARequest{
		Question: "Years?", ResumeText: "4 years",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Confidence != 0.85 {
		t.Errorf("expected 0.85, got %f", resp.Confidence)
	}
}
