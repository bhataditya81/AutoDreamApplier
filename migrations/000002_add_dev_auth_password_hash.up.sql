-- Add password_hash column to support dev-only local authentication.
-- In production, authentication is handled by AWS Cognito; this column is only
-- populated when APP_ENV != production.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
