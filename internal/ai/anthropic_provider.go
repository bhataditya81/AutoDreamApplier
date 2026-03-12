package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// AnthropicProvider calls the Anthropic Messages API directly (no SDK).
type AnthropicProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicProvider constructs an AnthropicProvider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// --- Anthropic wire types ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) complete(ctx context.Context, prompt string, maxTokens int) (string, error) {
	body := anthropicRequest{
		Model:     p.model,
		MaxTokens: maxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if ar.Error != nil {
		return "", fmt.Errorf("anthropic API error: %s", ar.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API status %d: %s", resp.StatusCode, string(respBody))
	}
	if len(ar.Content) == 0 {
		return "", fmt.Errorf("anthropic returned empty content")
	}

	return strings.TrimSpace(ar.Content[0].Text), nil
}

// TailorResume implements Provider.
func (p *AnthropicProvider) TailorResume(ctx context.Context, req *ResumeTailorRequest) (*ResumeTailorResponse, error) {
	prompt := fmt.Sprintf(`You are a professional resume writer. Given the resume and job description below, inject relevant keywords from the job into the resume while keeping it truthful. Return ONLY the revised resume text, no commentary.

Job Title: %s
Company: %s
Job Description:
%s

Resume:
%s`, req.JobTitle, req.CompanyName, req.JobDescription, req.ResumeText)

	tailored, err := p.complete(ctx, prompt, 2000)
	if err != nil {
		return nil, fmt.Errorf("tailor resume: %w", err)
	}
	return &ResumeTailorResponse{TailoredText: tailored}, nil
}

// GenerateCoverLetter implements Provider.
func (p *AnthropicProvider) GenerateCoverLetter(ctx context.Context, req *CoverLetterRequest) (*CoverLetterResponse, error) {
	prompt := fmt.Sprintf(`Write a %s cover letter for the following job. Return ONLY the cover letter text, no commentary.

Job Title: %s
Company: %s
Job Description:
%s

Resume:
%s`, req.Tone, req.JobTitle, req.CompanyName, req.JobDescription, req.ResumeText)

	letter, err := p.complete(ctx, prompt, 800)
	if err != nil {
		return nil, fmt.Errorf("generate cover letter: %w", err)
	}
	words := len(strings.Fields(letter))
	return &CoverLetterResponse{CoverLetter: letter, WordCount: words}, nil
}

// AnswerFormQuestion implements Provider.
func (p *AnthropicProvider) AnswerFormQuestion(ctx context.Context, req *FormQARequest) (*FormQAResponse, error) {
	prompt := fmt.Sprintf(`Answer the following job application question concisely and professionally based on the resume. Return ONLY the answer text.

Question: %s
Job Title: %s

Resume:
%s`, req.Question, req.JobTitle, req.ResumeText)

	answer, err := p.complete(ctx, prompt, 300)
	if err != nil {
		return nil, fmt.Errorf("answer form question: %w", err)
	}
	return &FormQAResponse{Answer: answer, Confidence: 0.85}, nil
}
