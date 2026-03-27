import { test as base, request, type Page } from '@playwright/test';

const E2E_USER = {
  email: 'e2e_test@autodream.io',
  password: 'E2eTest123!AutoDream',
  fullName: 'E2E Test User',
};

/**
 * Mock user stored in localStorage so that components reading cached user data
 * see the values that tests assert against (e.g. settings test checks for
 * 'jane@example.com').  Real API calls use the real JWT from the Lambda.
 */
const MOCK_STORED_USER = {
  id: 'user-1',
  email: 'jane@example.com',
  fullName: 'Jane Smith',
};

export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page, baseURL }, use) => {
    // Obtain a real 24-hour JWT from the Lambda so that any un-intercepted API
    // calls (e.g. /api/v1/users/me from the dashboard layout) return 200 rather
    // than 401, which would redirect the page to /login before the test's
    // page.route() mocks have a chance to fire.
    const api = await request.newContext({ baseURL });

    // Register the test user if it doesn't exist yet (idempotent).
    await api.post('/api/v1/auth/register', {
      data: { email: E2E_USER.email, password: E2E_USER.password, fullName: E2E_USER.fullName },
    }).catch(() => { /* already registered — ignore */ });

    const loginRes = await api.post('/api/v1/auth/login', {
      data: { email: E2E_USER.email, password: E2E_USER.password },
    });
    const body = await loginRes.json().catch(() => ({}));
    const realToken: string = body?.data?.token ?? 'fake-dev-jwt-token';

    await api.dispose();

    // Navigate to the login page so localStorage is scoped to the correct
    // origin before we write to it.
    await page.goto('/login');

    await page.evaluate(
      ({ token, user }) => {
        localStorage.setItem('autodream_token', token);
        localStorage.setItem('autodream_user', JSON.stringify(user));
      },
      { token: realToken, user: MOCK_STORED_USER },
    );

    await use(page);
  },
});

export { expect } from '@playwright/test';
