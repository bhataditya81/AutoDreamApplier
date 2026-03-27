import { test as authTest, expect } from './fixtures/auth';

const mockUser = {
  id: 'user-1',
  email: 'jane@example.com',
  fullName: 'Jane Smith',
  tier: 'free',
  applyMode: 'review',
  autoThreshold: 0.8,
  dailyLimit: 5,
  isActive: true,
  createdAt: '2024-01-01T00:00:00Z',
};

const mockPrefs = {
  targetTitles: ['Software Engineer'],
  locations: [],
  remotePref: 'any',
  salaryCurrency: 'USD',
  exclusions: [],
  autoApplyEnabled: false,
  dailyApplicationLimit: 10,
  applyStartHour: 9,
  applyEndHour: 17,
  applyTimezone: 'America/New_York',
  slackWebhookUrl: '',
  discordWebhookUrl: '',
  webhookEvents: ['application_submitted'],
  emailDigestEnabled: true,
};

authTest.describe('Settings page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me', (route) => {
      if (route.request().method() === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockUser),
        });
      } else {
        route.continue();
      }
    });
    await page.route('**/api/v1/users/me/preferences', (route) => {
      if (route.request().method() === 'GET') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockPrefs),
        });
      } else {
        route.continue();
      }
    });
    await page.route('**/api/v1/users/me/credentials', (route) => {
      route.fulfill({ status: 204 });
    });
  });

  authTest('renders Profile section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.getByText('Profile & Apply Settings')).toBeVisible();
  });

  authTest('renders Preferences section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.getByRole('heading', { name: 'Job Preferences' })).toBeVisible();
  });

  authTest('renders Credentials section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    // Credentials form has a heading
    await expect(page.getByRole('heading', { name: /board credentials/i })).toBeVisible();
  });

  authTest('renders Notifications section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.getByText('Notifications')).toBeVisible();
  });

  authTest('Auto-Apply toggle is visible in Preferences section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.getByText('Auto-Apply')).toBeVisible();
  });

  authTest('Shows confirmation modal when enabling auto-apply', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');

    // Wait for form to load
    await expect(page.getByText('Software Engineer')).toBeVisible();

    // Find the Auto-Apply switch (it's in the preferences section)
    const autoApplySection = page.getByText('Auto-Apply').locator('../..');
    const switchBtn = autoApplySection.getByRole('switch');
    await switchBtn.click();

    await expect(page.getByText('Enable Auto-Apply?')).toBeVisible();
  });

  authTest('Webhook URL inputs are visible in Notifications section', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.getByPlaceholder(/hooks\.slack\.com/i)).toBeVisible();
    await expect(page.getByPlaceholder(/discord\.com\/api\/webhooks/i)).toBeVisible();
  });

  authTest('Event checkboxes appear after entering webhook URL', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');

    const slackInput = page.getByPlaceholder(/hooks\.slack\.com/i);
    await slackInput.fill('https://hooks.slack.com/services/test');

    await expect(page.getByText('New match')).toBeVisible();
    await expect(page.getByText('Application submitted')).toBeVisible();
  });

  authTest('Profile form shows user email', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/settings');
    await expect(page.locator('input[disabled]')).toHaveValue('jane@example.com');
  });

  authTest('editing Full Name and clicking Save triggers PATCH request', async ({ authenticatedPage: page }) => {
    let patchCalled = false;
    await page.route('**/api/v1/users/me', (route) => {
      if (route.request().method() === 'PATCH') {
        patchCalled = true;
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ...mockUser, fullName: 'Jane Updated' }),
        });
      } else {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockUser),
        });
      }
    });

    await page.goto('/dashboard/settings');
    // Wait for form to hydrate — email value is visible
    await expect(page.locator('input[disabled]')).toHaveValue('jane@example.com');

    // Edit the Full Name field (first text input in the profile form)
    const fullNameInput = page.getByPlaceholder(/jane smith/i);
    await fullNameInput.fill('Jane Updated');

    // Click Save (Profile & Apply Settings save button)
    const saveButtons = page.getByRole('button', { name: /save/i });
    await saveButtons.first().click();

    await expect.poll(() => patchCalled).toBe(true);
  });

  authTest('shows success message or toast after saving profile', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me', (route) => {
      if (route.request().method() === 'PATCH') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ...mockUser, fullName: 'Jane Updated' }),
        });
      } else {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(mockUser),
        });
      }
    });

    await page.goto('/dashboard/settings');
    await expect(page.locator('input[disabled]')).toHaveValue('jane@example.com');

    const fullNameInput = page.getByPlaceholder(/jane smith/i);
    await fullNameInput.fill('Jane Updated');

    const saveButtons = page.getByRole('button', { name: /save/i });
    await saveButtons.first().click();

    // Expect a success indicator — either a toast, an alert, or text containing "saved"
    await expect(
      page.getByText(/saved|success|updated/i).first()
    ).toBeVisible({ timeout: 5000 });
  });
});
