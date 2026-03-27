import { defineConfig, devices } from '@playwright/test';

/**
 * Production/staging e2e config.
 * Runs against the live Vercel deployment — no local server needed.
 * Usage: npx playwright test --config=playwright.prod.config.ts
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  retries: 1,
  use: {
    baseURL: 'https://autodreamapplier.vercel.app',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  // No webServer — tests run against the live deployment
});
