import { test as authTest, expect } from './fixtures/auth';

const mockResumeBase = {
  id: 'resume-1',
  userId: 'user-1',
  fileName: 'resume_jane.pdf',
  s3Key: 'resumes/user-1/resume_jane.pdf',
  isPrimary: true,
  ab_enabled: false,
  ab_weight: 1,
  total_applications: 0,
  total_views: 0,
  total_interviews: 0,
  total_offers: 0,
  interview_rate: 0,
  createdAt: '2024-06-01T00:00:00Z',
};

const mockResumeSecondary = {
  id: 'resume-2',
  userId: 'user-1',
  fileName: 'resume_alt.pdf',
  s3Key: 'resumes/user-1/resume_alt.pdf',
  isPrimary: false,
  ab_enabled: false,
  ab_weight: 1,
  total_applications: 0,
  total_views: 0,
  total_interviews: 0,
  total_offers: 0,
  interview_rate: 0,
  createdAt: '2024-07-01T00:00:00Z',
};

authTest.describe('Resumes page', () => {
  authTest.beforeEach(async ({ authenticatedPage: page }) => {
    // Preferences stub used by the sidebar AUTO badge check
    await page.route('**/api/v1/users/me/preferences', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ targetTitles: [], remotePref: 'any', salaryCurrency: 'USD', autoApplyEnabled: false }),
      });
    });
  });

  // ── Upload area ──────────────────────────────────────────────────────────────

  authTest('renders upload area with instructions', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText(/click to upload a resume/i)).toBeVisible();
    await expect(page.getByText(/PDF or DOCX/i)).toBeVisible();
  });

  // ── Empty state ──────────────────────────────────────────────────────────────

  authTest('shows empty state when no resumes exist', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText(/no resumes yet/i)).toBeVisible();
    await expect(page.getByText(/upload your first resume/i)).toBeVisible();
  });

  // ── Single resume ────────────────────────────────────────────────────────────

  authTest('renders resume filename', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText('resume_jane.pdf')).toBeVisible();
  });

  authTest('shows Primary badge on the primary resume', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText('Primary')).toBeVisible();
  });

  authTest('does NOT show Set Primary button on the primary resume', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByRole('button', { name: /set primary/i })).not.toBeVisible();
  });

  authTest('shows delete button for each resume', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByRole('button', { name: /delete resume/i })).toBeVisible();
  });

  // ── A/B testing controls ─────────────────────────────────────────────────────

  authTest('does NOT show A/B toggle with only 1 resume', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByLabel(/toggle a\/b testing/i)).not.toBeVisible();
  });

  authTest('shows A/B toggles when 2 or more resumes exist', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([mockResumeBase, mockResumeSecondary]),
      });
    });

    await page.goto('/dashboard/resumes');
    const switches = page.getByLabel(/toggle a\/b testing/i);
    await expect(switches).toHaveCount(2);
  });

  authTest('shows A/B Testing info banner when 2 or more resumes exist', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([mockResumeBase, mockResumeSecondary]),
      });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText(/A\/B Testing/i)).toBeVisible();
  });

  authTest('shows Set Primary button on non-primary resume', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([mockResumeBase, mockResumeSecondary]),
      });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByRole('button', { name: /set primary/i })).toBeVisible();
  });

  // ── Stats row ────────────────────────────────────────────────────────────────

  authTest('shows stats row for resumes with applications', async ({ authenticatedPage: page }) => {
    const resumeWithStats = {
      ...mockResumeBase,
      total_applications: 20,
      total_interviews: 4,
      total_offers: 1,
      interview_rate: 20.0,
    };

    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([resumeWithStats]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText(/20 apps/i)).toBeVisible();
    await expect(page.getByText(/4 interviews/i)).toBeVisible();
    await expect(page.getByText(/20\.0% rate/i)).toBeVisible();
    await expect(page.getByText(/1 offer/i)).toBeVisible();
  });

  authTest('does NOT show stats row for resumes with zero applications', async ({ authenticatedPage: page }) => {
    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([mockResumeBase]) });
    });

    await page.goto('/dashboard/resumes');
    // No apps → no "0 apps" text rendered
    await expect(page.getByText(/0 apps/i)).not.toBeVisible();
  });

  // ── Best Performer badge ─────────────────────────────────────────────────────

  authTest('shows Best Performer badge on resume with highest interview rate (5+ apps)', async ({ authenticatedPage: page }) => {
    const bestResume = {
      ...mockResumeBase,
      id: 'resume-best',
      fileName: 'resume_best.pdf',
      total_applications: 10,
      total_interviews: 4,
      interview_rate: 40.0,
    };
    const otherResume = {
      ...mockResumeSecondary,
      total_applications: 8,
      total_interviews: 1,
      interview_rate: 12.5,
    };

    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([bestResume, otherResume]),
      });
    });

    await page.goto('/dashboard/resumes');
    // exact: true avoids matching the A/B info banner which also contains "Best Performer" as substring
    await expect(page.getByText('Best Performer', { exact: true })).toBeVisible();
  });

  authTest('does NOT show Best Performer badge when no resume has 5+ applications', async ({ authenticatedPage: page }) => {
    const r1 = { ...mockResumeBase, total_applications: 2, interview_rate: 50.0 };
    const r2 = { ...mockResumeSecondary, total_applications: 3, interview_rate: 33.3 };

    await page.route('**/api/v1/users/me/resumes', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([r1, r2]) });
    });

    await page.goto('/dashboard/resumes');
    await expect(page.getByText('Best Performer', { exact: true })).not.toBeVisible();
  });
});
