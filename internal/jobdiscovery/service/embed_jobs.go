package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bhata/AutoDreamApplier/internal/embedding"
)

// EmbedNewJobs generates embeddings for jobs that do not have one yet.
// It processes up to 100 jobs per call. Intended to be called as a background
// goroutine after each scrape batch completes.
//
// Embeddings are stored as a JSON float32 array in the jobs.embedding column
// (pgvector type). If the AI service is unavailable the function logs and returns
// without error so that the discovery loop is not interrupted.
func (s *DiscoveryService) EmbedNewJobs(ctx context.Context, embClient *embedding.Client) error {
	// Fetch jobs that need an embedding (no embedding yet, have a description).
	type jobRow struct {
		ID          uuid.UUID
		Description string
	}

	rows, err := s.repo.Pool().Query(ctx, `
		SELECT id, description
		FROM jobs
		WHERE embedding IS NULL
		  AND description IS NOT NULL
		  AND description != ''
		  AND is_active = true
		ORDER BY discovered_at DESC
		LIMIT 100`,
	)
	if err != nil {
		return fmt.Errorf("query jobs without embeddings: %w", err)
	}
	defer rows.Close()

	var jobs []jobRow
	for rows.Next() {
		var j jobRow
		if err := rows.Scan(&j.ID, &j.Description); err != nil {
			return fmt.Errorf("scan job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate job rows: %w", err)
	}

	if len(jobs) == 0 {
		return nil // nothing to do
	}

	s.log.Info().Int("count", len(jobs)).Msg("generating embeddings for new jobs")

	// Collect descriptions for batch embedding.
	descs := make([]string, len(jobs))
	for i, j := range jobs {
		descs[i] = j.Description
	}

	embeddings, err := embClient.EmbedBatch(ctx, descs)
	if err != nil {
		// AI service unavailable — log and skip; next cycle will retry.
		s.log.Warn().Err(err).Msg("embed batch failed — skipping embedding update")
		return nil
	}

	// Persist each embedding.
	now := time.Now()
	for i, j := range jobs {
		if i >= len(embeddings) {
			break
		}

		embJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			s.log.Warn().Err(err).Str("job_id", j.ID.String()).Msg("marshal embedding failed")
			continue
		}

		_, err = s.repo.Pool().Exec(ctx, `
			UPDATE jobs
			SET embedding = $1::vector, embedding_generated_at = $2
			WHERE id = $3`,
			string(embJSON), now, j.ID,
		)
		if err != nil {
			s.log.Warn().Err(err).Str("job_id", j.ID.String()).Msg("update job embedding failed")
		}
	}

	s.log.Info().Int("updated", len(jobs)).Msg("job embeddings updated")
	return nil
}
