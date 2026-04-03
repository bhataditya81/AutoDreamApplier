package scorer_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
	"github.com/bhata/AutoDreamApplier/internal/jobmatcher/scorer"
)

// makeEmbServer returns a test server that always returns the provided vector.
func makeEmbServer(t *testing.T, vec []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding":  vec,
			"dimensions": len(vec),
		})
	}))
}

// unit384 returns a unit vector of length 384 (first element = 1, rest = 0).
func unit384() []float32 {
	v := make([]float32, 384)
	v[0] = 1.0
	return v
}

func TestSemanticScorer_SameVector_ScoreIsOne(t *testing.T) {
	t.Parallel()
	srv := makeEmbServer(t, unit384())
	defer srv.Close()

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "resume text", "job description")
	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("expected score ~1.0 for identical vectors, got %.4f", score)
	}
}

func TestSemanticScorer_ServiceDown_ReturnsFallback(t *testing.T) {
	t.Parallel()
	c := embedding.New("http://127.0.0.1:19998") // nothing listening
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "some resume", "some job")
	if score != 0.5 {
		t.Errorf("expected fallback 0.5 when AI service down, got %.4f", score)
	}
}

func TestSemanticScorer_EmptyTexts_ReturnsFallback(t *testing.T) {
	t.Parallel()
	srv := makeEmbServer(t, unit384())
	defer srv.Close()

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	// Empty resume
	if score := ss.Score(context.Background(), "", "job desc"); score != 0.5 {
		t.Errorf("empty resume: expected 0.5, got %.4f", score)
	}
	// Empty job description
	if score := ss.Score(context.Background(), "resume text", ""); score != 0.5 {
		t.Errorf("empty job desc: expected 0.5, got %.4f", score)
	}
}

func TestSemanticScorer_ScoreInRange(t *testing.T) {
	t.Parallel()
	srv := makeEmbServer(t, unit384())
	defer srv.Close()

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "engineer", "software developer")
	if score < 0 || score > 1 {
		t.Errorf("score %.4f out of [0, 1] range", score)
	}
}

// TestSemanticScorer_AIService500_ReturnsFallback verifies that a 500 response
// from the AI service causes Score to return the neutral fallback 0.5.
func TestSemanticScorer_AIService500_ReturnsFallback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "experienced Go developer", "backend engineer role")
	if score != 0.5 {
		t.Errorf("expected fallback 0.5 when AI service returns 500, got %.4f", score)
	}
}

// TestSemanticScorer_AIServiceUnreachable_ReturnsFallback verifies Score returns
// 0.5 when the AI service is completely unreachable (connection refused).
func TestSemanticScorer_AIServiceUnreachable_ReturnsFallback(t *testing.T) {
	t.Parallel()
	// Nothing listening on this port.
	c := embedding.New("http://127.0.0.1:19996")
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "resume text", "job description text")
	if score != 0.5 {
		t.Errorf("expected fallback 0.5 when AI service is unreachable, got %.4f", score)
	}
}

// TestSemanticScorer_ContextCancelled_ReturnsFallback verifies that Score
// returns 0.5 when the context is cancelled before the HTTP call completes.
func TestSemanticScorer_ContextCancelled_ReturnsFallback(t *testing.T) {
	t.Parallel()
	// Server that never responds so the cancel fires first.
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked // block until test cleanup closes the channel
	}))
	t.Cleanup(func() { close(blocked); srv.Close() })

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	score := ss.Score(ctx, "resume text", "job description text")
	if score != 0.5 {
		t.Errorf("expected fallback 0.5 when context is cancelled, got %.4f", score)
	}
}

// TestSemanticScorer_ScoreClamped verifies the score stays within [0.0, 1.0].
// The AI service returns two perpendicular unit vectors, yielding cosine = 0.
// The scorer must never return values outside the valid range.
func TestSemanticScorer_ScoreClamped(t *testing.T) {
	t.Parallel()

	// Two perpendicular unit vectors: cosine similarity = 0.
	// First call returns e1, second call returns e2.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := make([]float32, 384)
		if callCount == 0 {
			v[0] = 1.0 // e1
		} else {
			v[1] = 1.0 // e2 — perpendicular to e1
		}
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"embedding":  v,
			"dimensions": len(v),
		})
	}))
	defer srv.Close()

	c := embedding.New(srv.URL)
	ss := scorer.NewSemanticScorer(c)

	score := ss.Score(context.Background(), "resume A", "job B")
	if score < 0.0 || score > 1.0 {
		t.Errorf("score %.6f is outside [0.0, 1.0]", score)
	}
}
