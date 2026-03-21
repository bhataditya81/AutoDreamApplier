-- Auto-apply schedule and webhook preferences
ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS auto_apply_enabled BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS daily_application_limit INT DEFAULT 10,
    ADD COLUMN IF NOT EXISTS apply_start_hour INT DEFAULT 9,
    ADD COLUMN IF NOT EXISTS apply_end_hour INT DEFAULT 17,
    ADD COLUMN IF NOT EXISTS apply_timezone VARCHAR(50) DEFAULT 'UTC';
