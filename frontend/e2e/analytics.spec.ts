import { test as authTest, expect } from './fixtures/auth';

const mockFunnel = {
  applied: 45,
  viewed: 12,
  interviews: 8,
  offers: 2,
  view_rate: 26.7,
  interview_rate: 17.8,
  offer_rate: 4.4,
};

const mockOverTime = [
  { date: '2026-03-01', count: 3 },
  { date: '2026-03-02', count: 5 },
  { date: '2026-03-03', count: 2 },
];

const mockByResume = [
  {
    resume_id: 'r1',
    file_name: 'resume_jane.pdf',
    total_applications: 20,
    total_interviews: 4,
    total_offers: 1,
    interview_rate: 20.0,
  },
];

const mockTopCompanies = [
  { company: 'Stripe', count: 8 },
  { company: 'Airbnb', count: 5 },
  { company: 'Vercel', count: 3 },
];

authTest.describe('Analytics page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/analytics/funnel**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockFunnel),
      });
    });
    await page.route('**/api/v1/analytics/over-time**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockOverTime),
      });
    });
    await page.route('**/api/v1/analytics/by-resume**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockByResume),
      });
    });
    await page.route('**/api/v1/analytics/top-companies**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockTopCompanies),
      });
    });
    await page.route('**/api/v1/applications**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0, page: 1, pageSize: 20, hasMore: false }),
      });
    });
    await page.route('**/api/v1/users/me/preferences', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ targetTitles: [], remotePref: 'any', salaryCurrency: 'USD', autoApplyEnabled: false }),
      });
    });
  });

  authTest('renders page heading "Analytics"', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByRole('heading', { name: /analytics/i })).toBeVisible();
  });

  authTest('"30 days" button is present', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    // Range buttons render as "30d", "90d", "180d"
    await expect(page.getByRole('button', { name: /30d/i })).toBeVisible();
  });

  authTest('"90 days" and "180 days" buttons are present', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByRole('button', { name: /90d/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /180d/i })).toBeVisible();
  });

  authTest('Applied stat card shows count', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    // 45 appears in both the stat card value and the funnel chart — use first()
    await expect(page.getByText('45').first()).toBeVisible();
  });

  authTest('Interviews stat card shows count', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByText('8')).toBeVisible();
  });

  authTest('shows interview rate conversion percentage', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    // Appears in multiple elements — match the standalone percentage value exactly
    await expect(page.getByText('17.8%', { exact: true })).toBeVisible();
  });

  authTest('Applications Over Time section is visible', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByText(/applications over time/i)).toBeVisible();
  });

  authTest('Conversion Funnel section is visible', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByText(/conversion funnel/i)).toBeVisible();
  });

  authTest('Top Companies section is visible', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByText(/top companies/i)).toBeVisible();
  });

  authTest('top company "Stripe" appears in the chart', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByText('Stripe')).toBeVisible();
  });

  authTest('clicking "90 days" updates URL param to range=90', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await page.getByRole('button', { name: /90d/i }).click();
    await expect(page).toHaveURL(/[?&]range=90/);
  });

  authTest('clicking "180 days" updates URL param to range=180', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await page.getByRole('button', { name: /180d/i }).click();
    await expect(page).toHaveURL(/[?&]range=180/);
  });

  authTest('shows Retry button when API fails', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/analytics/funnel**', (route) => {
      route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"server error"}' });
    });
    await page.goto('/dashboard/analytics');
    await expect(page.getByRole('button', { name: /retry/i })).toBeVisible();
  });

  authTest('shows empty state when no applications exist', async ({ authenticatedPage: page }) => {
    const emptyFunnel = {
      applied: 0, viewed: 0, interviews: 0, offers: 0,
      view_rate: 0, interview_rate: 0, offer_rate: 0,
    };
    await page.route('**/api/v1/analytics/funnel**', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(emptyFunnel) });
    });
    await page.route('**/api/v1/analytics/over-time**', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
    });
    await page.route('**/api/v1/analytics/top-companies**', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
    });

    await page.goto('/dashboard/analytics');
    // Both the heading and the paragraph match this pattern — use first()
    await expect(page.getByText(/no data yet|apply to some jobs/i).first()).toBeVisible();
  });

  authTest('Analytics link is visible in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page.getByRole('link', { name: /analytics/i })).toBeVisible();
  });
});
