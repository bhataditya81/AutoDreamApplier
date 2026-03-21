import { test as authTest, expect } from './fixtures/auth';

const mockApplication = {
  id: 'app-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchId: 'match-1',
  resumeId: 'resume-1',
  status: 'applied',
  createdAt: '2024-06-15T00:00:00Z',
  job: {
    id: 'job-1',
    externalId: 'ext-1',
    sourceBoard: 'indeed',
    url: 'https://example.com/job',
    title: 'Software Engineer',
    company: 'Acme Corp',
    location: 'New York, NY',
    isRemote: false,
    salaryCurrency: 'USD',
    description: 'A great job',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

authTest.describe('Applications page', () => {
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
    await page.route('**/api/v1/applications**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [mockApplication], total: 1, page: 1, pageSize: 20, hasMore: false }),
      });
    });
  });

  authTest('renders applications list with job title', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/applications');
    await expect(page.getByText('Software Engineer')).toBeVisible();
    await expect(page.getByText('Acme Corp')).toBeVisible();
  });

  authTest('renders status tabs', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/applications');
    // ApplicationList renders status tabs
    await expect(page.getByRole('button', { name: /all/i }).first()).toBeVisible();
  });

  authTest('renders Details link for each application', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/applications');
    await expect(page.getByRole('link', { name: /details/i })).toBeVisible();
  });

  authTest('Details link navigates to application detail page', async ({ authenticatedPage: page }) => {
    // Mock the detail page API
    await page.route('**/api/v1/applications/app-1', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockApplication),
      });
    });

    await page.goto('/dashboard/applications');
    await page.getByRole('link', { name: /details/i }).click();
    await expect(page).toHaveURL(/\/dashboard\/applications\/app-1/);
  });
});
