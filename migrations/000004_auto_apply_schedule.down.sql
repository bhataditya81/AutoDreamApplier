-- reverse
ALTER TABLE user_preferences DROP COLUMN IF EXISTS auto_apply_enabled, DROP COLUMN IF EXISTS daily_application_limit, DROP COLUMN IF EXISTS apply_start_hour, DROP COLUMN IF EXISTS apply_end_hour, DROP COLUMN IF EXISTS apply_timezone;
