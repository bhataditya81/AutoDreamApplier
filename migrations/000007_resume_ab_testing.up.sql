-- Track A/B test group per resume
ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS ab_enabled BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ab_weight  INT     DEFAULT 1;  -- relative frequency weight

-- Enhanced outcome tracking per resume
-- interview_count already exists; add more granular tracking
ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS total_applications INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_views        INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_interviews   INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_offers       INT DEFAULT 0;

-- Function to update resume stats when application outcome changes
CREATE OR REPLACE FUNCTION update_resume_stats()
RETURNS TRIGGER AS $$
BEGIN
    -- On outcome change to 'interview', increment total_interviews
    IF NEW.outcome = 'interview' AND (OLD.outcome IS NULL OR OLD.outcome != 'interview') THEN
        UPDATE resumes SET total_interviews = total_interviews + 1
        WHERE id = NEW.resume_id;
    END IF;
    -- On outcome change to 'offer', increment total_offers
    IF NEW.outcome = 'offer' AND (OLD.outcome IS NULL OR OLD.outcome != 'offer') THEN
        UPDATE resumes SET total_offers = total_offers + 1
        WHERE id = NEW.resume_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_resume_stats
    AFTER UPDATE OF outcome ON applications
    FOR EACH ROW EXECUTE FUNCTION update_resume_stats();

-- Increment total_applications when application is created
CREATE OR REPLACE FUNCTION increment_resume_application_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE resumes SET total_applications = total_applications + 1
    WHERE id = NEW.resume_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_increment_resume_app_count
    AFTER INSERT ON applications
    FOR EACH ROW EXECUTE FUNCTION increment_resume_application_count();
