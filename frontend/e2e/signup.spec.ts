import { test, expect } from '@playwright/test';

test.describe('Signup page', () => {
  test('renders email, password, and confirm password fields', async ({ page }) => {
    await page.goto('/signup');
    await expect(page.locator('input[type="email"]')).toBeVisible();
    // Two password fields: Password and Confirm password
    const passwordFields = page.locator('input[type="password"]');
    await expect(passwordFields.first()).toBeVisible();
    await expect(passwordFields.nth(1)).toBeVisible();
  });

  test('"Create free account" button is present', async ({ page }) => {
    await page.goto('/signup');
    await expect(
      page.getByRole('button', { name: /create free account/i })
    ).toBeVisible();
  });

  test('shows link to login page', async ({ page }) => {
    await page.goto('/signup');
    const loginLink = page.getByRole('link', { name: /sign in/i });
    await expect(loginLink).toBeVisible();
    await expect(loginLink).toHaveAttribute('href', '/login');
  });

  test('shows error message on 409 conflict (email already taken)', async ({ page }) => {
    await page.route('**/api/v1/auth/register', (route) => {
      route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Email already registered' }),
      });
    });

    await page.goto('/signup');
    await page.locator('input[placeholder="Jane Smith"]').fill('Jane Doe');
    await page.locator('input[type="email"]').fill('existing@example.com');
    const passwordFields = page.locator('input[type="password"]');
    await passwordFields.first().fill('password123');
    await passwordFields.nth(1).fill('password123');
    await page.getByRole('button', { name: /create free account/i }).click();

    // Use filter to exclude the Next.js route announcer which also has role="alert"
    const alert = page.getByRole('alert').filter({ hasText: /email already registered/i });
    await expect(alert).toBeVisible();
  });

  test('redirects to /dashboard/onboarding on successful signup', async ({ page }) => {
    await page.route('**/api/v1/auth/register', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          token: 'fake-jwt-token',
          user: {
            id: 'user-new',
            email: 'new@example.com',
            fullName: 'New User',
          },
        }),
      });
    });

    await page.goto('/signup');
    await page.locator('input[placeholder="Jane Smith"]').fill('New User');
    await page.locator('input[type="email"]').fill('new@example.com');
    const passwordFields = page.locator('input[type="password"]');
    await passwordFields.first().fill('password123');
    await passwordFields.nth(1).fill('password123');
    await page.getByRole('button', { name: /create free account/i }).click();

    await expect(page).toHaveURL(/\/dashboard\/onboarding/);
  });

  test('shows password mismatch error when passwords do not match', async ({ page }) => {
    await page.goto('/signup');
    await page.locator('input[placeholder="Jane Smith"]').fill('Test User');
    await page.locator('input[type="email"]').fill('test@example.com');
    const passwordFields = page.locator('input[type="password"]');
    await passwordFields.first().fill('password123');
    await passwordFields.nth(1).fill('differentpass');
    await page.getByRole('button', { name: /create free account/i }).click();

    const alert = page.getByRole('alert').filter({ hasText: /passwords do not match/i });
    await expect(alert).toBeVisible();
  });
});
