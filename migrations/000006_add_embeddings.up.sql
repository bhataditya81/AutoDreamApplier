-- Migration 000006: Add embedding columns to jobs table for semantic matching.
-- Note: pgvector extension and resumes.embedding are already enabled in 000001_init_schema.
-- The job_embeddings separate table also exists; this adds inline columns on jobs for
-- fast co-located queries.

-- Enable pgvector extension (idempotent — already created in 000001)
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding column to jobs table (384 dimensions for all-MiniLM-L6-v2)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS embedding vector(384);

-- Track when the embedding was last generated
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS embedding_generated_at TIMESTAMPTZ;

-- IVFFlat index for fast approximate nearest-neighbor search.
-- NOTE: Only effective after a meaningful number of rows are inserted.
-- Build CONCURRENTLY in production after initial data load.
CREATE INDEX IF NOT EXISTS idx_jobs_embedding ON jobs USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
