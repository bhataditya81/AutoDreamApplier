import { test as authTest, expect } from './fixtures/auth';

const mockDashboard = {
  user: {
    id: 'user-1',
    email: 'test@example.com',
    fullName: 'Test User',
    daily_limit: 10,
    is_active: true,
    tier: 'starter',
    apply_mode: 'review',
    auto_threshold: 0.8,
  },
  stats: {
    total_applications: 42,
    applied_today: 3,
    pending_matches: 7,
    interviews_received: 5,
  },
  preferences: {
    targetTitles: ['Software Engineer'],
    remotePref: 'any',
    salaryCurrency: 'USD',
    autoApplyEnabled: false,
  },
  resumes: [],
};

const mockApplications = [
  {
    id: 'app-1',
    userId: 'user-1',
    jobId: 'job-1',
    matchId: 'match-1',
    resumeId: 'resume-1',
    status: 'applied',
    createdAt: new Date().toISOString(),
    job: {
      id: 'job-1',
      title: 'Senior Backend Engineer',
      company: 'Acme Corp',
      location: 'Remote',
    },
  },
  {
    id: 'app-2',
    userId: 'user-1',
    jobId: 'job-2',
    matchId: 'match-2',
    resumeId: 'resume-1',
    status: 'failed',
    createdAt: new Date().toISOString(),
    job: {
      id: 'job-2',
      title: 'Staff Engineer',
      company: 'Beta Inc',
      location: 'New York, NY',
    },
  },
];

authTest.describe('Overview page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/dashboard', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockDashboard),
      });
    });
    await page.route('**/api/v1/applications**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: mockApplications, total: 2, page: 1, pageSize: 5, hasMore: false }),
      });
    });
    await page.route('**/api/v1/users/me/preferences', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockDashboard.preferences),
      });
    });
  });

  authTest('renders page heading', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByRole('heading', { name: /overview/i })).toBeVisible();
  });

  authTest('renders Total Applied stat card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Total Applied')).toBeVisible();
  });

  authTest('renders Interviews stat card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Interviews', { exact: true }).first()).toBeVisible();
  });

  authTest('renders Pending Matches stat card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Pending Matches')).toBeVisible();
  });

  authTest('renders Success Rate stat card', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Success Rate')).toBeVisible();
  });

  authTest("renders Today's Applications stat card with daily limit", async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText("Today's Applications")).toBeVisible();
    await expect(page.getByText(/of 10 limit/i)).toBeVisible();
  });

  authTest('renders Weekly Activity chart section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Weekly Activity')).toBeVisible();
  });

  authTest('renders Recent Applications section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Recent Applications')).toBeVisible();
  });

  authTest('renders recent application job titles', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Senior Backend Engineer')).toBeVisible();
    await expect(page.getByText('Staff Engineer')).toBeVisible();
  });

  authTest('renders Applied status badge for applied app', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Applied', { exact: true }).first()).toBeVisible();
  });

  authTest('renders Failed status badge for failed app', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Failed')).toBeVisible();
  });

  authTest('shows Manual Mode pill when autoApply is off', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Manual Mode')).toBeVisible();
  });

  authTest('shows Auto-Apply Active pill when autoApply is on', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/preferences', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...mockDashboard.preferences, autoApplyEnabled: true }),
      });
    });
    await page.goto('/dashboard/overview');
    await expect(page.getByText('Auto-Apply Active')).toBeVisible();
  });

  authTest('shows empty state message when no applications exist', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/applications**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0, page: 1, pageSize: 5, hasMore: false }),
      });
    });
    await page.goto('/dashboard/overview');
    await expect(page.getByText(/no applications yet/i)).toBeVisible();
  });

  authTest('shows Retry button on API error', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/dashboard', (route) => {
      route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"server error"}' });
    });
    await page.goto('/dashboard/overview');
    await expect(page.getByRole('button', { name: /retry/i })).toBeVisible();
  });
});
