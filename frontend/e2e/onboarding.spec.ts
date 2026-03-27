import { test as authTest, expect } from './fixtures/auth';

authTest.describe('Onboarding page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    // Stub resume upload
    await page.route('**/api/v1/users/me/resumes', (route) => {
      if (route.request().method() === 'POST') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            id: 'r1',
            fileName: 'resume.pdf',
            isPrimary: true,
            createdAt: '2026-01-01',
          }),
        });
      } else {
        // GET resumes → empty initially
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      }
    });

    // Stub preferences save
    await page.route('**/api/v1/users/me/preferences', (route) => {
      if (route.request().method() === 'PUT') {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) });
      } else {
        route.continue();
      }
    });
  });

  authTest('onboarding page loads with step 1 visible', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/onboarding');
    // Step label for first step is visible
    await expect(page.getByText('Upload Resume')).toBeVisible();
  });

  authTest('step 1 has upload area with resume-related text', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/onboarding');
    // The drop zone text or heading contains upload/resume
    await expect(
      page.getByText(/upload|resume/i).first()
    ).toBeVisible();
  });

  authTest('progress indicator shows step labels', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/onboarding');
    await expect(page.getByText('Upload Resume')).toBeVisible();
    await expect(page.getByText('Job Preferences')).toBeVisible();
    await expect(page.getByText('Ready!')).toBeVisible();
  });

  authTest('preferences form step exists (step 2 reachable by completing step 1)', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/onboarding');
    // Upload a file to advance to step 2
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: 'resume.pdf',
      mimeType: 'application/pdf',
      buffer: Buffer.from('PDF content'),
    });
    // Click the Upload & Continue button
    await page.getByRole('button', { name: /upload.*continue/i }).click();
    // Step 2 heading should appear — use heading role to avoid matching the progress label span
    await expect(page.getByRole('heading', { name: /job preferences/i })).toBeVisible();
  });

  authTest('completion step exists with link to dashboard', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/onboarding');

    // Step 1: upload resume
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: 'resume.pdf',
      mimeType: 'application/pdf',
      buffer: Buffer.from('PDF content'),
    });
    await page.getByRole('button', { name: /upload.*continue/i }).click();
    await expect(page.getByRole('heading', { name: /job preferences/i })).toBeVisible();

    // Step 2: stub savePreferences and submit
    await page.route('**/api/v1/users/me/preferences', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          targetTitles: ['Software Engineer'],
          locations: [],
          remotePref: 'any',
          salaryCurrency: 'USD',
          exclusions: [],
        }),
      });
    });

    // Add at least one title and save
    const titleInput = page.getByPlaceholder(/software engineer/i);
    await titleInput.fill('Software Engineer');
    await titleInput.press('Enter');
    await page.getByRole('button', { name: /save.*continue/i }).click();

    // Completion step
    await expect(page.getByText(/you're all set/i)).toBeVisible();
    // Link to dashboard (match queue)
    await expect(page.getByRole('link', { name: 'Go to Match Queue' })).toBeVisible();
  });
});
