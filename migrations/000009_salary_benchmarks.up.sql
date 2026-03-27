CREATE TABLE IF NOT EXISTS salary_benchmarks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_key     VARCHAR(200) NOT NULL,
    location_key  VARCHAR(200) NOT NULL,
    currency      VARCHAR(3)   NOT NULL DEFAULT 'USD',
    min_salary    INT          NOT NULL,
    p25_salary    INT          NOT NULL,
    median_salary INT          NOT NULL,
    p75_salary    INT          NOT NULL,
    max_salary    INT          NOT NULL,
    sample_size   INT          NOT NULL,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (title_key, location_key, currency)
);
CREATE INDEX IF NOT EXISTS idx_salary_benchmarks_lookup
    ON salary_benchmarks (title_key, location_key);
