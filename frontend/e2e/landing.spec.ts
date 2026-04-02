import { test, expect } from '@playwright/test';

test.describe('Landing page — unauthenticated', () => {
  test.beforeEach(async ({ page }) => {
    // Ensure no auth token so the landing page is shown (not redirected)
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('autodream_token');
      localStorage.removeItem('autodream_user');
    });
    await page.goto('/');
  });

  // ── Hero ──────────────────────────────────────────────────────────────────

  test('renders hero headline', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /apply to hundreds of/i })).toBeVisible();
  });

  test('renders Start for free CTA button', async ({ page }) => {
    await expect(page.getByRole('link', { name: /start for free/i })).toBeVisible();
  });

  test('renders See how it works link', async ({ page }) => {
    await expect(page.getByRole('link', { name: /see how it works/i })).toBeVisible();
  });

  test('trial badge is visible in hero', async ({ page }) => {
    await expect(page.getByText(/7-day free pro trial/i)).toBeVisible();
  });

  // ── Navbar ────────────────────────────────────────────────────────────────

  test('navbar renders brand logo text', async ({ page }) => {
    await expect(page.getByRole('link', { name: /AutoDream/i }).first()).toBeVisible();
  });

  test('navbar Sign in link points to /login', async ({ page }) => {
    const signIn = page.getByRole('link', { name: /sign in/i });
    await expect(signIn).toBeVisible();
    await expect(signIn).toHaveAttribute('href', '/login');
  });

  test('navbar Start free trial link points to /signup', async ({ page }) => {
    const cta = page.getByRole('link', { name: /start free trial/i }).first();
    await expect(cta).toBeVisible();
    await expect(cta).toHaveAttribute('href', '/signup');
  });

  // ── Stats bar ─────────────────────────────────────────────────────────────

  test('stats bar shows Active users', async ({ page }) => {
    await expect(page.getByText('Active users')).toBeVisible();
  });

  test('stats bar shows submission success rate', async ({ page }) => {
    await expect(page.getByText('Submission success rate')).toBeVisible();
  });

  // ── Features ─────────────────────────────────────────────────────────────

  test('features section heading is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /everything you need to land/i })).toBeVisible();
  });

  test('AI-powered tailoring feature card is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /ai-powered tailoring/i })).toBeVisible();
  });

  test('8 ATS plugins feature card is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /8 ats plugins/i })).toBeVisible();
  });

  // ── How it works ─────────────────────────────────────────────────────────

  test('how it works section is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /up and running in minutes/i })).toBeVisible();
  });

  test('shows all 5 numbered steps', async ({ page }) => {
    await expect(page.getByText('01')).toBeVisible();
    await expect(page.getByText('05')).toBeVisible();
  });

  // ── Pricing ───────────────────────────────────────────────────────────────

  test('pricing section heading is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /simple, transparent pricing/i })).toBeVisible();
  });

  test('pricing shows Free, Starter, and Pro tiers', async ({ page }) => {
    await expect(page.getByText('Free', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Starter', { exact: true })).toBeVisible();
    await expect(page.getByText('Pro', { exact: true })).toBeVisible();
  });

  test('pricing shows $0 for Free tier', async ({ page }) => {
    await expect(page.getByText('$0')).toBeVisible();
  });

  test('"Get started free" button links to /signup', async ({ page }) => {
    const btn = page.getByRole('link', { name: /get started free/i });
    await expect(btn).toBeVisible();
    await expect(btn).toHaveAttribute('href', '/signup');
  });

  test('"Most popular" badge is visible on Starter plan', async ({ page }) => {
    await expect(page.getByText('Most popular')).toBeVisible();
  });

  // ── FAQ ───────────────────────────────────────────────────────────────────

  test('FAQ section heading is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /everything you need to know/i })).toBeVisible();
  });

  test('FAQ accordion items are collapsed by default', async ({ page }) => {
    const firstQ = page.getByText('How does AutoDreamApplier work?');
    await expect(firstQ).toBeVisible();
    // Answer text should not be in the DOM (collapsed)
    await expect(page.getByText(/our cloud browser infrastructure/i)).not.toBeVisible();
  });

  test('clicking a FAQ item expands it', async ({ page }) => {
    await page.getByText('How does AutoDreamApplier work?').click();
    await expect(page.getByText(/our cloud browser infrastructure/i)).toBeVisible();
  });

  test('clicking an expanded FAQ item collapses it', async ({ page }) => {
    await page.getByText('How does AutoDreamApplier work?').click();
    await expect(page.getByText(/our cloud browser infrastructure/i)).toBeVisible();
    await page.getByText('How does AutoDreamApplier work?').click();
    await expect(page.getByText(/our cloud browser infrastructure/i)).not.toBeVisible();
  });

  test('FAQ shows all 8 questions', async ({ page }) => {
    const faqs = [
      'How does AutoDreamApplier work?',
      'Is it safe to automate job applications?',
      'Which job boards and ATS systems do you support?',
      'Will my resume be tailored for each job?',
      'What happens if an application fails?',
      'Can I set a daily application limit?',
      'How do I cancel my subscription?',
      'Is there a free trial?',
    ];
    for (const q of faqs) {
      await expect(page.getByText(q)).toBeVisible();
    }
  });

  // ── Contact ───────────────────────────────────────────────────────────────

  test('contact section heading is visible', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /get in touch/i })).toBeVisible();
  });

  test('contact form has name, email, subject, message fields', async ({ page }) => {
    await expect(page.getByPlaceholder('Jane Smith')).toBeVisible();
    await expect(page.getByPlaceholder('jane@example.com')).toBeVisible();
    await expect(page.getByPlaceholder(/how can we help/i)).toBeVisible();
    await expect(page.getByPlaceholder(/tell us about/i)).toBeVisible();
  });

  test('contact form shows loading state and success on submit', async ({ page }) => {
    await page.route('**/api/v1/contact', (route) => {
      route.fulfill({ status: 200, contentType: 'application/json', body: '{"status":"ok"}' });
    });

    await page.getByPlaceholder('Jane Smith').fill('Test User');
    await page.getByPlaceholder('jane@example.com').fill('test@example.com');
    await page.getByPlaceholder(/tell us about/i).fill('Hello from e2e test');
    await page.getByRole('button', { name: /send message/i }).click();

    await expect(page.getByText(/message sent/i)).toBeVisible();
  });

  test('contact form shows error message on API failure', async ({ page }) => {
    await page.route('**/api/v1/contact', (route) => {
      route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"fail"}' });
    });

    await page.getByPlaceholder('Jane Smith').fill('Test User');
    await page.getByPlaceholder('jane@example.com').fill('test@example.com');
    await page.getByPlaceholder(/tell us about/i).fill('Hello from e2e test');
    await page.getByRole('button', { name: /send message/i }).click();

    await expect(page.getByText(/something went wrong/i)).toBeVisible();
  });

  // ── Footer ────────────────────────────────────────────────────────────────

  test('footer renders Product section links', async ({ page }) => {
    await expect(page.getByRole('link', { name: /features/i }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: /pricing/i }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: /changelog/i })).toBeVisible();
  });

  test('footer renders Legal section links', async ({ page }) => {
    await expect(page.getByRole('link', { name: /privacy policy/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /terms of service/i })).toBeVisible();
  });

  test('footer renders Support section links', async ({ page }) => {
    await expect(page.getByRole('link', { name: /documentation/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /status page/i })).toBeVisible();
  });

  test('footer shows copyright and status indicator', async ({ page }) => {
    await expect(page.getByText(/2026 autodreamapplier/i)).toBeVisible();
    await expect(page.getByText(/all systems operational/i)).toBeVisible();
  });
});

// ── Authenticated root redirect ───────────────────────────────────────────────

test.describe('Landing page — authenticated redirect', () => {
  test('/ renders landing page content when not authenticated', async ({ page }) => {
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.removeItem('autodream_token');
    });
    await page.goto('/');
    await expect(page.getByRole('heading', { name: /apply to hundreds of/i })).toBeVisible();
  });
});
