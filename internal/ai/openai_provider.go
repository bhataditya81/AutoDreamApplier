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

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider calls the OpenAI Chat Completions API directly (no SDK).
type OpenAIProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider constructs an OpenAIProvider.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// --- OpenAI wire types ---

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) complete(ctx context.Context, prompt string) (string, error) {
	body := openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var or openAIResponse
	if err := json.Unmarshal(respBody, &or); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if or.Error != nil {
		return "", fmt.Errorf("openai API error: %s", or.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai API status %d: %s", resp.StatusCode, string(respBody))
	}
	if len(or.Choices) == 0 {
		return "", fmt.Errorf("openai returned empty choices")
	}

	return strings.TrimSpace(or.Choices[0].Message.Content), nil
}

// TailorResume implements Provider.
func (p *OpenAIProvider) TailorResume(ctx context.Context, req *ResumeTailorRequest) (*ResumeTailorResponse, error) {
	prompt := fmt.Sprintf(`You are a professional resume writer. Given the resume and job description below, inject relevant keywords from the job into the resume while keeping it truthful. Return ONLY the revised resume text, no commentary.

Job Title: %s
Company: %s
Job Description:
%s

Resume:
%s`, req.JobTitle, req.CompanyName, req.JobDescription, req.ResumeText)

	tailored, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("tailor resume: %w", err)
	}
	return &ResumeTailorResponse{TailoredText: tailored}, nil
}

// GenerateCoverLetter implements Provider.
func (p *OpenAIProvider) GenerateCoverLetter(ctx context.Context, req *CoverLetterRequest) (*CoverLetterResponse, error) {
	prompt := fmt.Sprintf(`Write a %s cover letter for the following job. Return ONLY the cover letter text, no commentary.

Job Title: %s
Company: %s
Job Description:
%s

Resume:
%s`, req.Tone, req.JobTitle, req.CompanyName, req.JobDescription, req.ResumeText)

	letter, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate cover letter: %w", err)
	}
	words := len(strings.Fields(letter))
	return &CoverLetterResponse{CoverLetter: letter, WordCount: words}, nil
}

// AnswerFormQuestion implements Provider.
func (p *OpenAIProvider) AnswerFormQuestion(ctx context.Context, req *FormQARequest) (*FormQAResponse, error) {
	prompt := fmt.Sprintf(`Answer the following job application question concisely and professionally based on the resume. Return ONLY the answer text.

Question: %s
Job Title: %s

Resume:
%s`, req.Question, req.JobTitle, req.ResumeText)

	answer, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("answer form question: %w", err)
	}
	return &FormQAResponse{Answer: answer, Confidence: 0.85}, nil
}
