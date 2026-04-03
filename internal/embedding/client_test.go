package embedding_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
)

// fakeEmbedServer returns a test HTTP server that responds with a 384-dim zero vector.
func fakeEmbedServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/embeddings/text":
			emb := make([]float32, 384)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"embedding":  emb,
				"dimensions": 384,
			})
		case "/api/v1/embeddings/batch":
			var req struct {
				Texts []string `json:"texts"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			embs := make([][]float32, len(req.Texts))
			for i := range embs {
				embs[i] = make([]float32, 384)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"embeddings": embs,
				"dimensions": 384,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestEmbedText_ReturnsVector(t *testing.T) {
	t.Parallel()
	srv := fakeEmbedServer(t)
	defer srv.Close()

	c := embedding.New(srv.URL)
	emb, err := c.EmbedText(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("EmbedText error: %v", err)
	}
	if len(emb) != 384 {
		t.Errorf("expected 384 dims, got %d", len(emb))
	}
}

func TestEmbedBatch_ReturnsVectors(t *testing.T) {
	t.Parallel()
	srv := fakeEmbedServer(t)
	defer srv.Close()

	c := embedding.New(srv.URL)
	embs, err := c.EmbedBatch(context.Background(), []string{"foo", "bar"})
	if err != nil {
		t.Fatalf("EmbedBatch error: %v", err)
	}
	if len(embs) != 2 {
		t.Errorf("expected 2 embeddings, got %d", len(embs))
	}
}

func TestEmbedText_ServiceDown_ReturnsError(t *testing.T) {
	t.Parallel()
	// Point at a port that is not listening.
	c := embedding.New("http://127.0.0.1:19999")
	_, err := c.EmbedText(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error when service is unreachable, got nil")
	}
}
