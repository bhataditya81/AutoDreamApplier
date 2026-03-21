import { test, expect } from '@playwright/test';
import { test as authTest } from './fixtures/auth';

// ── Unauthenticated access ───────────────────────────────────────────────────

test.describe('Dashboard — unauthenticated', () => {
  test('redirects to /login when no token is stored', async ({ page }) => {
    // Clear any stored auth
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('autodream_token');
      localStorage.removeItem('autodream_user');
    });
    await page.goto('/dashboard/matches');
    await expect(page).toHaveURL(/\/login/);
  });
});

// ── Authenticated access ─────────────────────────────────────────────────────

authTest.describe('Dashboard — authenticated', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    // Mock API calls so the page can load
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
  });

  authTest('shows sidebar with nav links', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByText('AutoDream')).toBeVisible();
    await expect(page.getByRole('link', { name: /match queue/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /applications/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /settings/i })).toBeVisible();
  });

  authTest('shows Resumes link in sidebar footer', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByRole('link', { name: /resumes/i })).toBeVisible();
  });

  authTest('does NOT show AUTO badge when autoApply is off', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    await expect(page.getByText('AUTO')).not.toBeVisible();
  });

  authTest('shows AUTO badge when autoApply is on', async ({ authenticatedPage: page }) => {
    // Override with autoApplyEnabled: true
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
    await expect(page.getByText('AUTO')).toBeVisible();
  });
});
