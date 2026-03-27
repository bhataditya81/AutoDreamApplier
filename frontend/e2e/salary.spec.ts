import { type Page } from '@playwright/test';
import { test as authTest, expect } from './fixtures/auth';

// ── Shared mock data ──────────────────────────────────────────────────────────

const mockMatch = {
  id: 'match-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchScore: 0.87,
  matchBreakdown: { title: 0.9, location: 0.8, salary: 0.85 },
  status: 'pending',
  createdAt: new Date(Date.now() - 3600000).toISOString(),
  updatedAt: new Date(Date.now() - 3600000).toISOString(),
  job: {
    id: 'job-1',
    externalId: 'ext-1',
    sourceBoard: 'indeed',
    url: 'https://example.com/job',
    title: 'Software Engineer',
    company: 'TechCorp',
    location: 'New York, NY',
    isRemote: false,
    salaryMin: 160000,
    salaryMax: 200000,
    salaryCurrency: 'USD',
    description: 'Build cool stuff',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

const mockBenchmarkResponse = {
  benchmark: {
    title_key: 'software engineer',
    location_key: 'new-york-ny',
    currency: 'USD',
    min: 120000,
    p25: 150000,
    median: 175000,
    p75: 195000,
    max: 230000,
    sample_size: 34,
    updated_at: '2026-03-20T00:00:00Z',
  },
  job_salary_min: 160000,
  job_salary_max: 200000,
  market_position: 'above',
};

// ── Helper to stub common routes ──────────────────────────────────────────────

async function stubBaseRoutes(
  page: Page,
  benchmarkBody: object | null,
  benchmarkStatus = 200,
) {
  // Stub user profile to avoid unmocked GET /users/me calls from the settings or header
  await page.route('**/api/v1/users/me', (route) => {
    if (route.request().method() === 'GET') {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'user-1', email: 'test@example.com', fullName: 'Test User', tier: 'free', applyMode: 'review', autoThreshold: 0.8, dailyLimit: 5, isActive: true }),
      });
    } else {
      route.continue();
    }
  });
  await page.route('**/api/v1/users/me/preferences', (route) => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        targetTitles: [],
        locations: [],
        remotePref: 'any',
        salaryCurrency: 'USD',
        exclusions: [],
        autoApplyEnabled: false,
      }),
    });
  });

  await page.route('**/api/v1/matches**', (route) => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: [mockMatch],
        total: 1,
        page: 1,
        pageSize: 12,
        hasMore: false,
      }),
    });
  });

  await page.route('**/api/v1/salary/benchmark**', (route) => {
    if (benchmarkStatus === 404) {
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not found' }) });
      return;
    }
    route.fulfill({
      status: benchmarkStatus,
      contentType: 'application/json',
      body: JSON.stringify(benchmarkBody),
    });
  });
}

// ── Tests ─────────────────────────────────────────────────────────────────────

authTest.describe('Salary benchmark badge', () => {
  authTest('renders market range when benchmark data is available', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, mockBenchmarkResponse);
    await page.goto('/dashboard/matches');
    // Wait for MatchQueue toolbar then for match card to render
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('Software Engineer')).toBeVisible({ timeout: 10000 });
    // Should show p25–p75 range from the salary benchmark badge
    await expect(page.getByText(/\$150k\s*–\s*\$195k/)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/34 jobs/)).toBeVisible();
  });

  authTest('shows "Above market" label when market_position is above', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, { ...mockBenchmarkResponse, market_position: 'above' });
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('Software Engineer')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/above market/i)).toBeVisible({ timeout: 10000 });
  });

  authTest('shows "At market" label when market_position is at', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, { ...mockBenchmarkResponse, market_position: 'at' });
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('Software Engineer')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/at market/i)).toBeVisible({ timeout: 10000 });
  });

  authTest('shows "Below market" label when market_position is below', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, { ...mockBenchmarkResponse, market_position: 'below' });
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('Software Engineer')).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/below market/i)).toBeVisible({ timeout: 10000 });
  });

  authTest('shows nothing when benchmark is null (insufficient data)', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, { benchmark: null, market_position: 'unknown' });
    await page.goto('/dashboard/matches');

    // The badge should not render — no market range text
    await expect(page.getByText(/Market:/)).not.toBeVisible();
  });

  authTest('shows nothing when API returns 404', async ({ authenticatedPage: page }) => {
    await stubBaseRoutes(page, null, 404);
    await page.goto('/dashboard/matches');

    // The badge should not render — no market range text
    await expect(page.getByText(/Market:/)).not.toBeVisible();
  });
});
