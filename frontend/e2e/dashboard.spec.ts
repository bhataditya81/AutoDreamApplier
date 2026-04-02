import { test, expect } from '@playwright/test';
import { test as authTest } from './fixtures/auth';

// ── Unauthenticated access ───────────────────────────────────────────────────

test.describe('Dashboard — unauthenticated', () => {
  test('redirects to /login when no token is stored', async ({ page }) => {
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('autodream_token');
      localStorage.removeItem('autodream_user');
    });
    await page.goto('/dashboard/matches');
    await expect(page).toHaveURL(/\/login/);
  });

  test('root / redirects to /dashboard/overview when authenticated', async ({ page }) => {
    // Set token first, then navigate — useEffect fires and redirects
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.setItem('autodream_token', 'fake-dev-jwt-token');
      localStorage.setItem('autodream_user', JSON.stringify({
        id: 'user-1',
        email: 'test@example.com',
        fullName: 'Test User',
      }));
    });
    await page.goto('/');
    await expect(page).toHaveURL(/\/dashboard\/overview/, { timeout: 10_000 });
  });
});

// ── Authenticated sidebar navigation ────────────────────────────────────────

authTest.describe('Dashboard — authenticated', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    // Stub APIs consumed by the sidebar + match queue
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
        body: JSON.stringify({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false }),
      });
    });
    await page.route('**/api/v1/users/me/dashboard', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user: { id: 'user-1', email: 'test@example.com', fullName: 'Test User', daily_limit: 5 },
          stats: { total_applications: 0, applied_today: 0, pending_matches: 0, interviews_received: 0 },
          preferences: {},
          resumes: [],
        }),
      });
    });
    await page.route('**/api/v1/applications**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0, page: 1, pageSize: 20, hasMore: false }),
      });
    });
  });

  authTest('shows Overview link in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /overview/i })).toBeVisible();
  });

  authTest('shows Match Queue link in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /match queue/i })).toBeVisible();
  });

  authTest('shows Applications link in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /applications/i })).toBeVisible();
  });

  authTest('shows Resumes link in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /resumes/i })).toBeVisible();
  });

  authTest('shows Settings link in sidebar', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /settings/i })).toBeVisible();
  });

  authTest('does NOT show auto-apply badge when autoApply is off', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByTestId('auto-apply-badge')).not.toBeVisible();
  });

  authTest('shows auto-apply badge (pulsing dot) when autoApply is on', async ({ authenticatedPage: page }) => {
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
          autoApplyEnabled: true,
        }),
      });
    });
    await page.goto('/dashboard/matches');
    await expect(page.getByTestId('auto-apply-badge')).toBeVisible();
  });
});
