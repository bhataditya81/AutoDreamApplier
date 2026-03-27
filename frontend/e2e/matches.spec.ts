import { test as authTest, expect } from './fixtures/auth';

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
    title: 'Senior Frontend Engineer',
    company: 'TechCorp',
    location: 'San Francisco, CA',
    isRemote: false,
    salaryCurrency: 'USD',
    description: 'Build cool stuff',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

authTest.describe('Matches page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
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
        body: JSON.stringify({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false }),
      });
    });
    // Stub salary benchmark so the match card doesn't make unmocked network calls
    await page.route('**/api/v1/salary/benchmark**', (route) => {
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not found' }) });
    });
  });

  authTest('renders status tabs', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    // Wait for MatchQueue to mount — Refresh button is always rendered in the toolbar
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByRole('button', { name: 'Pending' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Approved' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Rejected' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'All', exact: true })).toBeVisible();
  });

  authTest('renders match card with job title and company', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText('Senior Frontend Engineer')).toBeVisible();
    await expect(page.getByText('TechCorp')).toBeVisible();
  });

  authTest('renders Approve (Apply) button on each card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByRole('button', { name: /apply/i }).first()).toBeVisible();
  });

  authTest('renders Skip button on each card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByRole('button', { name: /skip/i }).first()).toBeVisible();
  });

  authTest('clicking Approved tab sends correct status filter', async ({ authenticatedPage: page }) => {
    // Track the request params
    let statusParam: string | null = null;
    await page.route('**/api/v1/matches**', (route) => {
      const url = new URL(route.request().url());
      statusParam = url.searchParams.get('status');
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false }),
      });
    });

    await page.goto('/dashboard/matches');
    await page.getByRole('button', { name: 'Approved' }).click();

    // Wait for the network call with status=approved
    await page.waitForTimeout(500);
    expect(statusParam).toBe('approved');
  });

  authTest('shows Select all button after matches load', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    // Wait for MatchQueue toolbar to mount, then wait for matches to load (bulk toolbar appears)
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/select all/i)).toBeVisible({ timeout: 10000 });
  });

  authTest('shows selection checkbox on individual cards', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    // Wait for matches to load and Select all to appear
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/select all/i)).toBeVisible({ timeout: 10000 });
    // Click "Select all" to reveal selection checkbox on the card
    await page.getByText(/select all/i).click();
    // Now the "Deselect all" text should appear
    await expect(page.getByText(/deselect all/i)).toBeVisible();
  });
});
