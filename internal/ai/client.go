package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Python AI Service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new AI service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Request/Response types ---

// ResumeTailorRequest is the request to tailor a resume.
type ResumeTailorRequest struct {
	ResumeText     string `json:"resume_text"`
	JobTitle       string `json:"job_title"`
	JobDescription string `json:"job_description"`
	CompanyName    string `json:"company_name,omitempty"`
	Mode           string `json:"mode"` // "keyword_inject" or "full_rewrite"
}

// ResumeTailorResponse is the response from resume tailoring.
type ResumeTailorResponse struct {
	TailoredText  string   `json:"tailored_text"`
	ChangesMade   []string `json:"changes_made"`
	KeywordsAdded []string `json:"keywords_added"`
}

// CoverLetterRequest is the request to generate a cover letter.
type CoverLetterRequest struct {
	ResumeText     string `json:"resume_text"`
	JobTitle       string `json:"job_title"`
	JobDescription string `json:"job_description"`
	CompanyName    string `json:"company_name"`
	Tone           string `json:"tone,omitempty"` // professional, enthusiastic, conversational
}

// CoverLetterResponse is the response with a generated cover letter.
type CoverLetterResponse struct {
	CoverLetter string `json:"cover_letter"`
	WordCount   int    `json:"word_count"`
}

// FormQARequest is the request to answer a form question.
type FormQARequest struct {
	Question     string   `json:"question"`
	ResumeText   string   `json:"resume_text"`
	JobTitle     string   `json:"job_title,omitempty"`
	QuestionType string   `json:"question_type,omitempty"` // text, textarea, dropdown, radio, checkbox
	Options      []string `json:"options,omitempty"`
}

// FormQAResponse is the response with the form answer.
type FormQAResponse struct {
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
}

// --- Methods ---

// TailorResume calls the AI service to tailor a resume for a specific job.
func (c *Client) TailorResume(ctx context.Context, req *ResumeTailorRequest) (*ResumeTailorResponse, error) {
	var resp ResumeTailorResponse
	if err := c.post(ctx, "/api/v1/resume/tailor", req, &resp); err != nil {
		return nil, fmt.Errorf("tailor resume: %w", err)
	}
	return &resp, nil
}

// GenerateCoverLetter calls the AI service to generate a cover letter.
func (c *Client) GenerateCoverLetter(ctx context.Context, req *CoverLetterRequest) (*CoverLetterResponse, error) {
	var resp CoverLetterResponse
	if err := c.post(ctx, "/api/v1/cover-letter/generate", req, &resp); err != nil {
		return nil, fmt.Errorf("generate cover letter: %w", err)
	}
	return &resp, nil
}

// AnswerFormQuestion calls the AI service to answer a form question.
func (c *Client) AnswerFormQuestion(ctx context.Context, req *FormQARequest) (*FormQAResponse, error) {
	var resp FormQAResponse
	if err := c.post(ctx, "/api/v1/form-qa/answer", req, &resp); err != nil {
		return nil, fmt.Errorf("answer form question: %w", err)
	}
	return &resp, nil
}

// HealthCheck verifies the AI service is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ai service health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ai service unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// post makes a POST request to the AI service and decodes the response.
func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI service error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
