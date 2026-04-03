// Package embedding provides an HTTP client for the AI service embedding endpoints.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls the AI service embedding endpoints.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates an embedding client targeting the given AI service base URL.
// Example: embedding.New("http://localhost:8086")
func New(aiServiceURL string) *Client {
	return &Client{
		baseURL: aiServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// embedTextRequest is the JSON body for POST /api/v1/embeddings/text.
type embedTextRequest struct {
	Text string `json:"text"`
}

// embedTextResponse is the JSON response from POST /api/v1/embeddings/text.
type embedTextResponse struct {
	Embedding  []float32 `json:"embedding"`
	Dimensions int       `json:"dimensions"`
}

// EmbedText returns a 384-dim embedding for the given text.
// Returns an error if the AI service is unreachable or returns a non-200 status.
func (c *Client) EmbedText(ctx context.Context, text string) ([]float32, error) {
	var result embedTextResponse
	if err := c.post(ctx, "/api/v1/embeddings/text", embedTextRequest{Text: text}, &result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

// embedBatchRequest is the JSON body for POST /api/v1/embeddings/batch.
type embedBatchRequest struct {
	Texts []string `json:"texts"`
}

// embedBatchResponse is the JSON response from POST /api/v1/embeddings/batch.
type embedBatchResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimensions int         `json:"dimensions"`
}

// EmbedBatch returns embeddings for up to 100 texts in a single request.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var result embedBatchResponse
	if err := c.post(ctx, "/api/v1/embeddings/batch", embedBatchRequest{Texts: texts}, &result); err != nil {
		return nil, err
	}
	return result.Embeddings, nil
}

// post is a helper that makes a POST request to the given path with the given request body,
// and decodes the response JSON into the given result.
func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("embedding service returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
