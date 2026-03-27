-- 000008_analytics.down.sql
DROP TABLE IF EXISTS application_daily_stats;
ALTER TABLE applications DROP COLUMN IF EXISTS outcome_notes;
