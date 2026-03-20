ALTER TABLE user_preferences
    ADD COLUMN IF NOT EXISTS slack_webhook_url VARCHAR(500),
    ADD COLUMN IF NOT EXISTS discord_webhook_url VARCHAR(500),
    ADD COLUMN IF NOT EXISTS webhook_events TEXT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS email_digest_enabled BOOLEAN DEFAULT TRUE;
