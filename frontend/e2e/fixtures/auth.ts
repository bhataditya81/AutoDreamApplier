import { test as base, type Page } from '@playwright/test';

export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page }, use) => {
    // Navigate first so localStorage is available on the correct origin
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.setItem('autodream_token', 'fake-dev-jwt-token');
      localStorage.setItem('autodream_user', JSON.stringify({
        id: 'user-1',
        email: 'test@example.com',
        fullName: 'Test User',
      }));
    });
    await use(page);
  },
});

export { expect } from '@playwright/test';
