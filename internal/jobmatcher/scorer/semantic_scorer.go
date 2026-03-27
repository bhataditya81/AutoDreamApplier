package scorer

import (
	"context"
	"math"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
)

// SemanticScorer scores job-resume similarity using vector embeddings from the AI service.
type SemanticScorer struct {
	embClient *embedding.Client
}

// NewSemanticScorer creates a SemanticScorer backed by the given embedding client.
func NewSemanticScorer(embClient *embedding.Client) *SemanticScorer {
	return &SemanticScorer{embClient: embClient}
}

// Score returns a 0.0–1.0 semantic similarity score between a resume and job description.
// Falls back to 0.5 (neutral) if the embedding client returns an error (e.g. AI service down).
// Both texts must be non-empty for a meaningful result; empty input also returns 0.5.
func (s *SemanticScorer) Score(ctx context.Context, resumeText, jobDescription string) float64 {
	if resumeText == "" || jobDescription == "" {
		return 0.5 // neutral when no content to compare
	}

	resumeEmb, err := s.embClient.EmbedText(ctx, resumeText)
	if err != nil {
		return 0.5 // neutral fallback — AI service unreachable
	}

	jobEmb, err := s.embClient.EmbedText(ctx, jobDescription)
	if err != nil {
		return 0.5 // neutral fallback
	}

	return cosineSimilarity32(resumeEmb, jobEmb)
}

// cosineSimilarity32 computes cosine similarity between two float32 slices.
// The AI service returns L2-normalized vectors, so this is equivalent to dot product.
// We still compute full cosine similarity for robustness.
func cosineSimilarity32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
