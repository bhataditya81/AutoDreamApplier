-- 000008_analytics.up.sql
-- Pre-computed daily stats for fast dashboard queries
CREATE TABLE IF NOT EXISTS application_daily_stats (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        DATE NOT NULL,
    applied     INT DEFAULT 0,
    viewed      INT DEFAULT 0,
    interviews  INT DEFAULT 0,
    offers      INT DEFAULT 0,
    UNIQUE (user_id, date)
);
CREATE INDEX IF NOT EXISTS idx_daily_stats_user_date ON application_daily_stats(user_id, date DESC);

-- outcome_notes for user comments on each outcome
ALTER TABLE applications ADD COLUMN IF NOT EXISTS outcome_notes TEXT;
