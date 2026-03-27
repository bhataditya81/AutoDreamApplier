DROP INDEX IF EXISTS idx_jobs_embedding;
ALTER TABLE jobs DROP COLUMN IF EXISTS embedding;
ALTER TABLE jobs DROP COLUMN IF EXISTS embedding_generated_at;
