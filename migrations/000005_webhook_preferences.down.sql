-- reverse
ALTER TABLE user_preferences DROP COLUMN IF EXISTS slack_webhook_url, DROP COLUMN IF EXISTS discord_webhook_url, DROP COLUMN IF EXISTS webhook_events, DROP COLUMN IF EXISTS email_digest_enabled;
