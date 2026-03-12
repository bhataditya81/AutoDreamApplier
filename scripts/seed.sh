#!/bin/bash
# Seed development database with test data
# Usage: ./scripts/seed.sh

set -euo pipefail

# Load .env if it exists
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-autodreamapplier}"

echo "🌱 Seeding AutoDreamApplier database..."
echo "   Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}"
echo ""

PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" <<'SQL'

-- Seed test user
INSERT INTO users (id, cognito_sub, email, full_name, tier, apply_mode, auto_threshold, daily_limit)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'test-cognito-sub-001',
    'testuser@example.com',
    'John Developer',
    'pro',
    'review',
    0.8,
    40
) ON CONFLICT (email) DO NOTHING;

-- Seed user preferences
INSERT INTO user_preferences (id, user_id, target_titles, locations, remote_pref, salary_min, salary_max, salary_currency, exclusions)
VALUES (
    'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    ARRAY['Software Engineer', 'Backend Developer', 'Full Stack Developer'],
    ARRAY['Remote', 'San Francisco, CA', 'New York, NY'],
    'remote',
    120000,
    200000,
    'USD',
    ARRAY['staffing-company', 'recruitment-agency']
) ON CONFLICT DO NOTHING;

-- Seed sample jobs
INSERT INTO jobs (id, external_id, source_board, url, title, company, location, is_remote, salary_min, salary_max, salary_currency, description, ats_type, apply_url, posted_at, discovered_at)
VALUES
(
    'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33',
    'greenhouse-12345',
    'indeed',
    'https://boards.greenhouse.io/acmecorp/jobs/12345',
    'Senior Backend Engineer',
    'Acme Corp',
    'San Francisco, CA',
    true,
    150000,
    200000,
    'USD',
    'We are looking for a Senior Backend Engineer to join our platform team. You will work on building scalable microservices using Go and Python. Requirements: 5+ years of experience, strong knowledge of distributed systems, experience with AWS.',
    'greenhouse',
    'https://boards.greenhouse.io/acmecorp/jobs/12345/apply',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '1 day'
),
(
    'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a44',
    'lever-67890',
    'glassdoor',
    'https://jobs.lever.co/techstartup/67890',
    'Full Stack Developer',
    'TechStartup Inc',
    'Remote',
    true,
    130000,
    170000,
    'USD',
    'Join our fast-growing startup as a Full Stack Developer. You will build features end-to-end using React, Node.js, and PostgreSQL. We value clean code and great communication.',
    'lever',
    'https://jobs.lever.co/techstartup/67890/apply',
    NOW() - INTERVAL '1 day',
    NOW()
),
(
    'e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a55',
    'workday-11111',
    'indeed',
    'https://bigcorp.wd5.myworkdayjobs.com/careers/job/Software-Engineer_R-11111',
    'Software Engineer II',
    'BigCorp Industries',
    'New York, NY',
    false,
    140000,
    180000,
    'USD',
    'BigCorp is seeking a Software Engineer II to join our core platform team. Responsibilities include designing and implementing APIs, working with cloud infrastructure, and mentoring junior engineers.',
    'workday',
    'https://bigcorp.wd5.myworkdayjobs.com/careers/job/Software-Engineer_R-11111/apply',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '2 days'
)
ON CONFLICT (external_id, source_board) DO NOTHING;

-- Seed sample matches
INSERT INTO matches (id, user_id, job_id, match_score, match_breakdown, status)
VALUES
(
    'f0eebc99-9c0b-4ef8-bb6d-6bb9bd380a66',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33',
    0.92,
    '{"title": 0.95, "location": 0.9, "salary": 0.85, "skills": 0.95}',
    'pending'
),
(
    'f1eebc99-9c0b-4ef8-bb6d-6bb9bd380a77',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a44',
    0.85,
    '{"title": 0.9, "location": 1.0, "salary": 0.7, "skills": 0.8}',
    'pending'
)
ON CONFLICT DO NOTHING;

SELECT 'Seeded: ' || count(*) || ' users' FROM users;
SELECT 'Seeded: ' || count(*) || ' preferences' FROM user_preferences;
SELECT 'Seeded: ' || count(*) || ' jobs' FROM jobs;
SELECT 'Seeded: ' || count(*) || ' matches' FROM matches;

SQL

echo ""
echo "✅ Database seeded successfully!"
