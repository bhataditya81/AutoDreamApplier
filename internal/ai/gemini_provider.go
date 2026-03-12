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

const geminiAPIBase = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiProvider calls the Google Gemini API directly (no SDK).
type GeminiProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiProvider constructs a GeminiProvider.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// --- Gemini wire types ---

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *GeminiProvider) complete(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiAPIBase, p.model, p.apiKey)

	body := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if gr.Error != nil {
		return "", fmt.Errorf("gemini API error: %s", gr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini API status %d: %s", resp.StatusCode, string(respBody))
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned empty response")
	}

	return strings.TrimSpace(gr.Candidates[0].Content.Parts[0].Text), nil
}

// TailorResume implements Provider.
func (p *GeminiProvider) TailorResume(ctx context.Context, req *ResumeTailorRequest) (*ResumeTailorResponse, error) {
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
func (p *GeminiProvider) GenerateCoverLetter(ctx context.Context, req *CoverLetterRequest) (*CoverLetterResponse, error) {
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
func (p *GeminiProvider) AnswerFormQuestion(ctx context.Context, req *FormQARequest) (*FormQAResponse, error) {
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
