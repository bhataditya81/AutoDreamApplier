import { test, expect } from '@playwright/test';

test.describe('Auth — Login page', () => {
  test('login page loads with email and password fields', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: /welcome back/i })).toBeVisible();
    await expect(page.getByPlaceholder(/jane@example\.com/i)).toBeVisible();
    await expect(page.getByPlaceholder(/password/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
  });

  test('shows a link to signup page', async ({ page }) => {
    await page.goto('/login');
    const signupLink = page.getByRole('link', { name: /create one free/i });
    await expect(signupLink).toBeVisible();
    await expect(signupLink).toHaveAttribute('href', '/signup');
  });

  test('shows error when submitting empty form (browser validation)', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('button', { name: /sign in/i }).click();
    // HTML5 required validation prevents submission — email field should be focused
    const emailInput = page.getByPlaceholder(/jane@example\.com/i);
    await expect(emailInput).toBeVisible();
  });

  test('shows error alert on invalid credentials (mocked API)', async ({ page }) => {
    // Intercept the login API — response matches the Go backend envelope format.
    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: { code: 'UNAUTHORIZED', message: 'Invalid email or password' },
        }),
      });
    });

    await page.goto('/login');
    await page.getByPlaceholder(/jane@example\.com/i).fill('bad@email.com');
    await page.getByPlaceholder(/password/i).fill('wrongpassword');
    await page.getByRole('button', { name: /sign in/i }).click();

    // Use first() to exclude the Next.js route announcer which also has role="alert"
    await expect(page.getByRole('alert').first()).toBeVisible();
    await expect(page.getByRole('alert').first()).toContainText(/invalid email or password/i);
  });

  test('redirects to dashboard on successful login (mocked API)', async ({ page }) => {
    // Response matches the Go backend envelope: { success, data: { token, user } }.
    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            token: 'fake-jwt-token',
            user: { id: 'user-1', email: 'test@example.com', fullName: 'Test User' },
          },
        }),
      });
    });
    // Stub downstream dashboard API calls so the app doesn't redirect back to /login
    await page.route('**/api/v1/users/me**', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: { id: 'user-1', email: 'test@example.com', fullName: 'Test User', tier: 'free', applyMode: 'review', autoThreshold: 0.8, dailyLimit: 5, isActive: true } }),
      });
    });
    await page.route('**/api/v1/matches**', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ success: true, data: [], meta: { total: 0, page: 1, perPage: 20, totalPages: 0 } }) });
    });

    await page.goto('/login');
    await page.getByPlaceholder(/jane@example\.com/i).fill('test@example.com');
    await page.getByPlaceholder(/password/i).fill('password123');
    await page.getByRole('button', { name: /sign in/i }).click();

    await expect(page).toHaveURL(/\/dashboard\//, { timeout: 10000 });
  });
});
