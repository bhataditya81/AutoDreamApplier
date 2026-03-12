-- AutoDreamApplier Initial Schema
-- Requires: pgvector extension

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- ============================================
-- Users & Authentication
-- ============================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cognito_sub     VARCHAR(255) UNIQUE NOT NULL,
    email           VARCHAR(255) UNIQUE NOT NULL,
    full_name       VARCHAR(255),
    tier            VARCHAR(20) NOT NULL DEFAULT 'free'
                    CHECK (tier IN ('free', 'starter', 'pro', 'enterprise')),
    apply_mode      VARCHAR(20) NOT NULL DEFAULT 'review'
                    CHECK (apply_mode IN ('review', 'auto')),
    auto_threshold  FLOAT NOT NULL DEFAULT 0.8
                    CHECK (auto_threshold >= 0.0 AND auto_threshold <= 1.0),
    daily_limit     INT NOT NULL DEFAULT 5,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_cognito_sub ON users(cognito_sub);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_tier ON users(tier);

-- ============================================
-- User Preferences
-- ============================================
CREATE TABLE user_preferences (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_titles   TEXT[] NOT NULL DEFAULT '{}',
    locations       TEXT[] NOT NULL DEFAULT '{}',
    remote_pref     VARCHAR(20) NOT NULL DEFAULT 'any'
                    CHECK (remote_pref IN ('remote', 'hybrid', 'onsite', 'any')),
    salary_min      INT,
    salary_max      INT,
    salary_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    exclusions      TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);

-- ============================================
-- Resumes
-- ============================================
CREATE TABLE resumes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_name       VARCHAR(255) NOT NULL,
    s3_key          VARCHAR(512) NOT NULL,
    parsed_json     JSONB,
    raw_text        TEXT,
    embedding       vector(384),
    is_primary      BOOLEAN NOT NULL DEFAULT false,
    interview_count INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_resumes_user_id ON resumes(user_id);
CREATE INDEX idx_resumes_primary ON resumes(user_id, is_primary) WHERE is_primary = true;

-- ============================================
-- Board Credentials (encrypted)
-- ============================================
CREATE TABLE board_credentials (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    board_name      VARCHAR(50) NOT NULL,
    encrypted_creds BYTEA NOT NULL,
    iv              BYTEA NOT NULL,
    is_valid        BOOLEAN NOT NULL DEFAULT true,
    last_verified   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, board_name)
);

CREATE INDEX idx_board_credentials_user_id ON board_credentials(user_id);

-- ============================================
-- Jobs
-- ============================================
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    external_id     VARCHAR(255) NOT NULL,
    source_board    VARCHAR(50) NOT NULL,
    url             VARCHAR(1024),
    title           VARCHAR(512) NOT NULL,
    company         VARCHAR(255),
    location        VARCHAR(255),
    is_remote       BOOLEAN DEFAULT false,
    salary_min      INT,
    salary_max      INT,
    salary_currency VARCHAR(3) DEFAULT 'USD',
    description     TEXT,
    ats_type        VARCHAR(50) DEFAULT 'unknown',
    apply_url       VARCHAR(1024),
    is_active       BOOLEAN NOT NULL DEFAULT true,
    is_scam         BOOLEAN NOT NULL DEFAULT false,
    scam_score      FLOAT,
    posted_at       TIMESTAMPTZ,
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(external_id, source_board)
);

CREATE INDEX idx_jobs_source_board ON jobs(source_board);
CREATE INDEX idx_jobs_title ON jobs USING gin(to_tsvector('english', title));
CREATE INDEX idx_jobs_company ON jobs(company);
CREATE INDEX idx_jobs_location ON jobs(location);
CREATE INDEX idx_jobs_is_active ON jobs(is_active) WHERE is_active = true;
CREATE INDEX idx_jobs_ats_type ON jobs(ats_type);
CREATE INDEX idx_jobs_discovered_at ON jobs(discovered_at DESC);

-- ============================================
-- Job Embeddings (Phase 2 - semantic matching)
-- ============================================
CREATE TABLE job_embeddings (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    embedding       vector(384) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(job_id)
);

-- IVFFlat index for fast approximate nearest neighbor search
-- NOTE: Only create after inserting initial data (needs training)
-- CREATE INDEX idx_job_embeddings_vector ON job_embeddings
--     USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- ============================================
-- Matches
-- ============================================
CREATE TABLE matches (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    match_score     FLOAT NOT NULL CHECK (match_score >= 0.0 AND match_score <= 1.0),
    match_breakdown JSONB,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'applied', 'expired')),
    user_feedback   VARCHAR(20)
                    CHECK (user_feedback IS NULL OR user_feedback IN ('thumbs_up', 'thumbs_down')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, job_id)
);

CREATE INDEX idx_matches_user_id ON matches(user_id);
CREATE INDEX idx_matches_status ON matches(user_id, status);
CREATE INDEX idx_matches_score ON matches(user_id, match_score DESC);
CREATE INDEX idx_matches_created_at ON matches(created_at DESC);

-- ============================================
-- Applications
-- ============================================
CREATE TABLE applications (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id              UUID NOT NULL REFERENCES jobs(id),
    match_id            UUID REFERENCES matches(id),
    resume_id           UUID NOT NULL REFERENCES resumes(id),
    status              VARCHAR(30) NOT NULL DEFAULT 'queued'
                        CHECK (status IN (
                            'queued', 'ai_preparing', 'ai_ready',
                            'applying', 'applied', 'failed', 'withdrawn'
                        )),
    outcome             VARCHAR(30)
                        CHECK (outcome IS NULL OR outcome IN (
                            'viewed', 'rejected', 'interview', 'offer'
                        )),
    tailored_resume_s3  VARCHAR(512),
    cover_letter_s3     VARCHAR(512),
    screenshot_s3       VARCHAR(512),
    form_answers_json   JSONB,
    error_message       TEXT,
    retry_count         INT NOT NULL DEFAULT 0,
    applied_at          TIMESTAMPTZ,
    outcome_updated_at  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, job_id)
);

CREATE INDEX idx_applications_user_id ON applications(user_id);
CREATE INDEX idx_applications_status ON applications(status);
CREATE INDEX idx_applications_user_status ON applications(user_id, status);
CREATE INDEX idx_applications_outcome ON applications(user_id, outcome);
CREATE INDEX idx_applications_created_at ON applications(created_at DESC);

-- ============================================
-- Application Events (Audit Trail)
-- ============================================
CREATE TABLE application_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    event_type      VARCHAR(50) NOT NULL,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_application_events_app_id ON application_events(application_id);
CREATE INDEX idx_application_events_type ON application_events(event_type);
CREATE INDEX idx_application_events_created_at ON application_events(created_at DESC);

-- ============================================
-- Rate Limiting Tracking
-- ============================================
CREATE TABLE rate_limit_tracking (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    board_name      VARCHAR(50) NOT NULL,
    date            DATE NOT NULL DEFAULT CURRENT_DATE,
    app_count       INT NOT NULL DEFAULT 0,
    last_applied_at TIMESTAMPTZ,
    UNIQUE(user_id, board_name, date)
);

CREATE INDEX idx_rate_limit_user_date ON rate_limit_tracking(user_id, date);

-- ============================================
-- Updated at trigger function
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply triggers
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_preferences_updated_at
    BEFORE UPDATE ON user_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_matches_updated_at
    BEFORE UPDATE ON matches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
