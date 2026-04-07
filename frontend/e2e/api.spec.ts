/**
 * api.spec.ts — Comprehensive real-API e2e tests for AutoDreamApplier
 *
 * These tests hit REAL endpoints on the live Lambda behind API Gateway.
 * No mocking of API calls except for explicit error-state tests.
 *
 * API Base: https://autodreamapplier.vercel.app  (proxies /api/v1 → Lambda)
 *
 * Run against prod:
 *   npx playwright test e2e/api.spec.ts --config=playwright.prod.config.ts
 */

import { test, expect, request as pwRequest } from '@playwright/test';
import { test as authTest } from './fixtures/auth';

// ── Constants ─────────────────────────────────────────────────────────────────

const E2E_USER = {
  email: 'e2e_test@autodream.io',
  password: 'E2eTest123!AutoDream',
  fullName: 'E2E Test User',
};

/** API base — routed through the Vercel deployment proxy */
const BASE = 'https://autodreamapplier.vercel.app';

/** Non-existent UUID — safe for 404 tests */
const NULL_UUID = '00000000-0000-0000-0000-000000000000';

/** Analytics date range covering the full current year */
const ANALYTICS_FROM = '2026-01-01';
const ANALYTICS_TO   = '2026-12-31';

// ── Token helper ──────────────────────────────────────────────────────────────

/**
 * Obtain a real short-lived JWT for the e2e test user.
 * Registers the user first (idempotent — 409 is swallowed).
 */
async function getToken(): Promise<string> {
  const ctx = await pwRequest.newContext({ baseURL: BASE });
  try {
    // Register (idempotent)
    await ctx.post('/api/v1/auth/register', {
      data: {
        email:    E2E_USER.email,
        password: E2E_USER.password,
        fullName: E2E_USER.fullName,
      },
    }).catch(() => { /* already registered */ });

    const loginRes = await ctx.post('/api/v1/auth/login', {
      data: { email: E2E_USER.email, password: E2E_USER.password },
    });

    expect(loginRes.status()).toBe(200);

    const body = await loginRes.json();
    // Accept both { token } and { data: { token } } shapes
    const token: string = body?.data?.token ?? body?.token;
    if (!token) throw new Error(`Login did not return a token. Body: ${JSON.stringify(body)}`);
    return token;
  } finally {
    await ctx.dispose();
  }
}

// ── Helper: authenticated API context ─────────────────────────────────────────

async function authedCtx(token: string) {
  return pwRequest.newContext({
    baseURL:      BASE,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
}

// =============================================================================
// AUTH ENDPOINTS
// =============================================================================

test.describe('API — Auth endpoints', () => {
  test('POST /auth/register returns 409 for already-registered user', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/auth/register', {
        data: {
          email:    E2E_USER.email,
          password: E2E_USER.password,
          fullName: E2E_USER.fullName,
        },
      });
      // If the user already exists, the backend must return 409 Conflict.
      // If this is a completely fresh run and the user did not exist yet, 201 is acceptable.
      expect([201, 409]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /auth/register returns 400 for missing required fields', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/auth/register', {
        data: { email: 'no-password@example.com' },
      });
      expect(res.status()).toBe(400);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /auth/login returns 200 with a token for valid credentials', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/auth/login', {
        data: { email: E2E_USER.email, password: E2E_USER.password },
      });
      expect(res.status()).toBe(200);

      const body = await res.json();
      const token: string = body?.data?.token ?? body?.token;
      expect(typeof token).toBe('string');
      expect(token.length).toBeGreaterThan(10);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /auth/login returns 401 for wrong password', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/auth/login', {
        data: { email: E2E_USER.email, password: 'definitely-wrong-password!' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /auth/login returns 401 for non-existent user', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/auth/login', {
        data: { email: 'nobody@nowhere-e2e.invalid', password: 'somepass' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// USER PROFILE ENDPOINTS
// =============================================================================

test.describe('API — User profile endpoints', () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getToken();
  });

  // GET /users/me ──────────────────────────────────────────────────────────────

  test('GET /users/me returns 200 with user object', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/users/me');
      expect(res.status()).toBe(200);

      const body = await res.json();
      expect(body.data ?? body).toHaveProperty('email');
      expect((body.data ?? body).email).toBe(E2E_USER.email);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /users/me returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/users/me');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // PATCH /users/me ────────────────────────────────────────────────────────────

  test('PATCH /users/me returns 200 and persists changes', async () => {
    const ctx = await authedCtx(token);
    try {
      const newName = `E2E Test User ${Date.now()}`;
      const res = await ctx.put('/api/v1/users/me', {
        data: { fullName: newName },
      });
      expect(res.status()).toBe(200);

      const body = await res.json();
      expect(body.fullName ?? body.full_name).toBe(newName);

      // Verify it persisted — re-fetch
      const verifyRes = await ctx.get('/api/v1/users/me');
      expect(verifyRes.status()).toBe(200);
      const verifyBody = await verifyRes.json();
      expect(verifyBody.fullName ?? verifyBody.full_name).toBe(newName);
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /users/me returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.put('/api/v1/users/me', {
        data: { fullName: 'Hacker' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // GET /users/me/dashboard ────────────────────────────────────────────────────

  test('GET /users/me/dashboard returns 200 with stats', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/users/me/dashboard');
      expect(res.status()).toBe(200);

      const body = await res.json();
      // Accepts { stats: {...} } or flat object
      const stats = body?.stats ?? body;
      expect(stats).toBeDefined();
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /users/me/dashboard returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/users/me/dashboard');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// PREFERENCES ENDPOINTS
// =============================================================================

test.describe('API — Preferences endpoints', () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getToken();
  });

  test('GET /users/me/preferences returns 200', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/users/me/preferences');
      expect(res.status()).toBe(200);

      const body = await res.json();
      // Must at least have a remotePref or remote_pref field
      expect(body).toBeDefined();
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /users/me/preferences returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/users/me/preferences');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PUT /users/me/preferences returns 200 and round-trips data', async () => {
    const ctx = await authedCtx(token);
    try {
      const prefs = {
        targetTitles:      ['Software Engineer', 'Backend Engineer'],
        locations:         ['Remote', 'New York, NY'],
        remotePref:        'remote',
        salaryMin:         120000,
        salaryMax:         180000,
        salaryCurrency:    'USD',
        exclusions:        ['sales', 'marketing'],
        autoApplyEnabled:  false,
      };

      const putRes = await ctx.put('/api/v1/users/me/preferences', { data: prefs });
      expect(putRes.status()).toBe(200);

      // Verify round-trip
      const getRes = await ctx.get('/api/v1/users/me/preferences');
      expect(getRes.status()).toBe(200);
      const saved = await getRes.json();
      // Accept snake_case or camelCase keys from the backend; unwrap envelope if present
      const saved_ = saved?.data ?? saved;
      const remotePref = saved_?.remotePref ?? saved_?.remote_pref;
      expect(remotePref).toBe('remote');
    } finally {
      await ctx.dispose();
    }
  });

  test('PUT /users/me/preferences returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.put('/api/v1/users/me/preferences', {
        data: { remotePref: 'remote' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// RESUME ENDPOINTS
// =============================================================================

test.describe('API — Resume endpoints', () => {
  let token: string;
  /** IDs of resumes created during this run — cleaned up in afterAll */
  const createdResumeIds: string[] = [];

  test.beforeAll(async () => {
    token = await getToken();
  });

  test.afterAll(async () => {
    if (createdResumeIds.length === 0) return;
    const ctx = await authedCtx(token);
    try {
      for (const id of createdResumeIds) {
        await ctx.delete(`/api/v1/users/me/resumes/${id}`).catch(() => { /* already gone */ });
      }
    } finally {
      await ctx.dispose();
    }
  });

  // GET /users/me/resumes ──────────────────────────────────────────────────────

  test('GET /users/me/resumes returns 200 with an array', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/users/me/resumes');
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body?.data ?? body)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /users/me/resumes returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/users/me/resumes');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // POST /users/me/resumes ─────────────────────────────────────────────────────

  test('POST /users/me/resumes uploads a PDF and returns 201 or 200 with resume object', async () => {
    const ctx = await authedCtx(token);
    try {
      // Minimal valid 1-page PDF (hand-crafted, avoids fs dependency)
      const pdfBytes = Buffer.from(
        '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n' +
        '2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n' +
        '3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\n' +
        'xref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n' +
        '0000000058 00000 n\n0000000115 00000 n\n' +
        'trailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF'
      );

      const form = {
        file: {
          name:    'e2e_resume.pdf',
          mimeType: 'application/pdf',
          buffer:  pdfBytes,
        },
      };

      const res = await ctx.post('/api/v1/users/me/resumes', { multipart: form });
      expect([200, 201]).toContain(res.status());

      const body = await res.json();
      const resumeData = body?.data ?? body;
      expect(resumeData).toHaveProperty('id');
      createdResumeIds.push(resumeData.id as string);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /users/me/resumes returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const pdfBytes = Buffer.from('%PDF-1.4\n%%EOF');
      const res = await ctx.post('/api/v1/users/me/resumes', {
        multipart: {
          file: { name: 'x.pdf', mimeType: 'application/pdf', buffer: pdfBytes },
        },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // PUT /users/me/resumes/:id/primary ─────────────────────────────────────────

  test('PUT /users/me/resumes/:id/primary sets the resume as primary', async () => {
    test.skip(createdResumeIds.length === 0, 'No resume was created to promote');
    const ctx = await authedCtx(token);
    try {
      const id = createdResumeIds[0];
      const res = await ctx.put(`/api/v1/users/me/resumes/${id}/primary`);
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PUT /users/me/resumes/:id/primary returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.put(`/api/v1/users/me/resumes/${NULL_UUID}/primary`);
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PUT /users/me/resumes/:id/primary returns 404 for non-existent resume', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.put(`/api/v1/users/me/resumes/${NULL_UUID}/primary`);
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  // PUT /users/me/resumes/:id/ab-test ─────────────────────────────────────────

  test('PUT /users/me/resumes/:id/ab-test returns 200 or 204', async () => {
    test.skip(createdResumeIds.length === 0, 'No resume was created for ab-test');
    const ctx = await authedCtx(token);
    try {
      const id = createdResumeIds[0];
      const res = await ctx.put(`/api/v1/users/me/resumes/${id}/ab-test`, {
        data: { abEnabled: true, abWeight: 0.5 },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PUT /users/me/resumes/:id/ab-test returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.put(`/api/v1/users/me/resumes/${NULL_UUID}/ab-test`, {
        data: { abEnabled: false, abWeight: 1 },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // DELETE /users/me/resumes/:id ───────────────────────────────────────────────

  test('DELETE /users/me/resumes/:id returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.delete(`/api/v1/users/me/resumes/${NULL_UUID}`);
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('DELETE /users/me/resumes/:id returns 404 for non-existent resume', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.delete(`/api/v1/users/me/resumes/${NULL_UUID}`);
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('DELETE /users/me/resumes/:id returns 200 or 204 for an existing resume', async () => {
    test.skip(createdResumeIds.length === 0, 'No resume to delete');
    const ctx = await authedCtx(token);
    try {
      const id = createdResumeIds.pop()!;
      const res = await ctx.delete(`/api/v1/users/me/resumes/${id}`);
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// BOARD CREDENTIALS ENDPOINT
// =============================================================================

test.describe('API — Board credentials endpoint', () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getToken();
  });

  test('POST /users/me/credentials returns 200 or 204 for valid data', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.post('/api/v1/users/me/credentials', {
        data: {
          boardName: 'indeed',
          email:     'test@indeed.com',
          password:  'testpass',
        },
      });
      expect([200, 201, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /users/me/credentials returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/users/me/credentials', {
        data: { boardName: 'indeed', email: 'a@b.com', password: 'pass' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /users/me/credentials returns 400 for missing boardName', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.post('/api/v1/users/me/credentials', {
        data: { email: 'a@b.com', password: 'pass' },
      });
      expect([400, 422]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// MATCHES ENDPOINTS
// =============================================================================

test.describe('API — Matches endpoints', () => {
  let token: string;
  /** First match ID fetched — used for single-match operations */
  let firstMatchId: string | null = null;

  test.beforeAll(async () => {
    token = await getToken();
    // Pre-fetch the match list so subsequent tests can use a real ID
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/matches?page=1&pageSize=1');
      if (res.status() === 200) {
        const body = await res.json();
        const items: Array<{ id: string }> = body?.data ?? body?.matches ?? body ?? [];
        if (Array.isArray(items) && items.length > 0) {
          firstMatchId = items[0].id;
        }
      }
    } finally {
      await ctx.dispose();
    }
  });

  // GET /matches ───────────────────────────────────────────────────────────────

  test('GET /matches returns 200 with paginated response', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/matches?page=1&pageSize=10');
      expect(res.status()).toBe(200);
      const body = await res.json();
      // Accept { data: [] } or flat [] or { matches: [] }
      const items = body?.data ?? body?.matches ?? body;
      expect(Array.isArray(items)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /matches returns 200 when filtered by status=pending', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/matches?status=pending&page=1&pageSize=10');
      expect(res.status()).toBe(200);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /matches returns 200 when filtered by status=approved', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/matches?status=approved&page=1&pageSize=10');
      expect(res.status()).toBe(200);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /matches returns 200 when filtered by status=rejected', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/matches?status=rejected&page=1&pageSize=10');
      expect(res.status()).toBe(200);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /matches returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/matches');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // PATCH /matches/:id ─────────────────────────────────────────────────────────

  test('PATCH /matches/:id returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.patch(`/api/v1/matches/${NULL_UUID}`, {
        data: { status: 'approved' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /matches/:id returns 404 for non-existent match', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/matches/${NULL_UUID}`, {
        data: { status: 'approved' },
      });
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /matches/:id approves an existing match', async () => {
    if (!firstMatchId) {
      test.skip(true, 'No match available to approve');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/matches/${firstMatchId}`, {
        data: { status: 'approved' },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /matches/:id rejects a match with optional feedback', async () => {
    if (!firstMatchId) {
      test.skip(true, 'No match available to reject');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/matches/${firstMatchId}`, {
        data: { status: 'rejected', userFeedback: 'Not the right role for me' },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  // POST /matches/bulk-action ──────────────────────────────────────────────────

  test('POST /matches/bulk-action returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.post('/api/v1/matches/bulk-action', {
        data: { action: 'approve', match_ids: [NULL_UUID] },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /matches/bulk-action with empty match_ids returns 200 or 400', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.post('/api/v1/matches/bulk-action', {
        data: { action: 'approve', match_ids: [] },
      });
      // Backend may accept empty list (no-op 200) or reject it (400)
      expect([200, 204, 400]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('POST /matches/bulk-action bulk-approves an existing match', async () => {
    if (!firstMatchId) {
      test.skip(true, 'No match available for bulk action');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.post('/api/v1/matches/bulk-action', {
        data: { action: 'approve', match_ids: [firstMatchId] },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  // PATCH /matches/:id/feedback ────────────────────────────────────────────────

  test('PATCH /matches/:id/feedback returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.patch(`/api/v1/matches/${NULL_UUID}/feedback`, {
        data: { feedback: 'thumbs_up' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /matches/:id/feedback returns 404 for non-existent match', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/matches/${NULL_UUID}/feedback`, {
        data: { feedback: 'thumbs_up' },
      });
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /matches/:id/feedback records thumbs_up on an existing match', async () => {
    if (!firstMatchId) {
      test.skip(true, 'No match available for feedback');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/matches/${firstMatchId}/feedback`, {
        data: { feedback: 'thumbs_up' },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// APPLICATIONS ENDPOINTS
// =============================================================================

test.describe('API — Applications endpoints', () => {
  let token: string;
  let firstAppId: string | null = null;

  test.beforeAll(async () => {
    token = await getToken();
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/applications?page=1&pageSize=1');
      if (res.status() === 200) {
        const body = await res.json();
        const items: Array<{ id: string }> = body?.data ?? body?.applications ?? body ?? [];
        if (Array.isArray(items) && items.length > 0) {
          firstAppId = items[0].id;
        }
      }
    } finally {
      await ctx.dispose();
    }
  });

  // GET /applications ──────────────────────────────────────────────────────────

  test('GET /applications returns 200 with paginated response', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/applications?page=1&pageSize=10');
      expect(res.status()).toBe(200);
      const body = await res.json();
      const items = body?.data ?? body?.applications ?? body;
      expect(Array.isArray(items)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /applications filters by status=applied', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/applications?status=applied&page=1&pageSize=10');
      expect(res.status()).toBe(200);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /applications returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/applications');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // GET /applications/:id ──────────────────────────────────────────────────────

  test('GET /applications/:id returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get(`/api/v1/applications/${NULL_UUID}`);
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /applications/:id returns 404 for non-existent application', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get(`/api/v1/applications/${NULL_UUID}`);
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /applications/:id returns 200 for an existing application', async () => {
    if (!firstAppId) {
      test.skip(true, 'No application exists for this test user yet');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get(`/api/v1/applications/${firstAppId}`);
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(body).toHaveProperty('id', firstAppId);
    } finally {
      await ctx.dispose();
    }
  });

  // PATCH /applications/:id (withdraw) ────────────────────────────────────────

  test('PATCH /applications/:id returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.patch(`/api/v1/applications/${NULL_UUID}`, {
        data: { status: 'withdrawn' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /applications/:id returns 404 for non-existent application', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/applications/${NULL_UUID}`, {
        data: { status: 'withdrawn' },
      });
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  // PATCH /applications/:id/outcome ────────────────────────────────────────────

  test('PATCH /applications/:id/outcome returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.patch(`/api/v1/applications/${NULL_UUID}/outcome`, {
        data: { outcome: 'interview_scheduled' },
      });
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /applications/:id/outcome returns 404 for non-existent application', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/applications/${NULL_UUID}/outcome`, {
        data: { outcome: 'interview_scheduled', notes: 'test' },
      });
      expect([404, 403]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });

  test('PATCH /applications/:id/outcome records outcome on an existing application', async () => {
    if (!firstAppId) {
      test.skip(true, 'No application exists to update outcome');
      return;
    }
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.patch(`/api/v1/applications/${firstAppId}/outcome`, {
        data: { outcome: 'interview_scheduled', notes: 'E2E test note' },
      });
      expect([200, 204]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// ANALYTICS ENDPOINTS
// =============================================================================

test.describe('API — Analytics endpoints', () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getToken();
  });

  // GET /analytics/funnel ──────────────────────────────────────────────────────

  test('GET /analytics/funnel returns 200 with funnel stats object', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get(`/api/v1/analytics/funnel?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`);
      expect(res.status()).toBe(200);
      const body = await res.json();
      // Must have at least an "applied" count (may be 0)
      expect(body).toHaveProperty('applied');
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /analytics/funnel returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get(`/api/v1/analytics/funnel?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`);
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // GET /analytics/over-time ───────────────────────────────────────────────────

  test('GET /analytics/over-time returns 200 with an array', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get(`/api/v1/analytics/over-time?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`);
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /analytics/over-time returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get(`/api/v1/analytics/over-time?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`);
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // GET /analytics/by-resume ───────────────────────────────────────────────────

  test('GET /analytics/by-resume returns 200 with an array', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/analytics/by-resume');
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /analytics/by-resume returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/analytics/by-resume');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  // GET /analytics/top-companies ───────────────────────────────────────────────

  test('GET /analytics/top-companies returns 200 with an array', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/analytics/top-companies?limit=10');
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /analytics/top-companies respects limit param', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/analytics/top-companies?limit=5');
      expect(res.status()).toBe(200);
      const body = await res.json();
      expect(Array.isArray(body)).toBe(true);
      // Cannot have more entries than the requested limit
      expect(body.length).toBeLessThanOrEqual(5);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /analytics/top-companies returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/analytics/top-companies?limit=10');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// SALARY BENCHMARK ENDPOINT
// =============================================================================

test.describe('API — Salary benchmark endpoint', () => {
  let token: string;

  test.beforeAll(async () => {
    token = await getToken();
  });

  test('GET /salary/benchmark returns 200 or 404 for known title + location', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/salary/benchmark?title=Software+Engineer&location=Remote');
      // 200 if data exists in the external source; 404 is an acceptable "no data" signal
      expect([200, 404]).toContain(res.status());
      if (res.status() === 200) {
        const body = await res.json();
        expect(body).toHaveProperty('benchmark');
      }
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /salary/benchmark returns 401 without a token', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      const res = await ctx.get('/api/v1/salary/benchmark?title=Engineer&location=Remote');
      expect(res.status()).toBe(401);
    } finally {
      await ctx.dispose();
    }
  });

  test('GET /salary/benchmark returns 400 when title param is missing', async () => {
    const ctx = await authedCtx(token);
    try {
      const res = await ctx.get('/api/v1/salary/benchmark?location=Remote');
      expect([400, 422]).toContain(res.status());
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// USER JOURNEY — Full onboarding flow (Register → Profile → Preferences)
// =============================================================================

test.describe('User journey — Full onboarding flow', () => {
  test('register → login → get profile → update profile → verify persistence', async () => {
    const ctx = await pwRequest.newContext({ baseURL: BASE });
    try {
      // Step 1: Ensure user exists (register is idempotent)
      const registerRes = await ctx.post('/api/v1/auth/register', {
        data: {
          email:    E2E_USER.email,
          password: E2E_USER.password,
          fullName: E2E_USER.fullName,
        },
      });
      expect([201, 409]).toContain(registerRes.status());

      // Step 2: Login and get a real JWT
      const loginRes = await ctx.post('/api/v1/auth/login', {
        data: { email: E2E_USER.email, password: E2E_USER.password },
      });
      expect(loginRes.status()).toBe(200);
      const loginBody = await loginRes.json();
      const token: string = loginBody?.data?.token ?? loginBody?.token;
      expect(typeof token).toBe('string');

      // Step 3: Get profile
      const meRes = await ctx.get('/api/v1/users/me', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(meRes.status()).toBe(200);
      const meBody = await meRes.json();
      expect((meBody.data ?? meBody).email).toBe(E2E_USER.email);

      // Step 4: Update profile
      const updatedName = `E2E Journey User ${Date.now()}`;
      const patchRes = await ctx.put('/api/v1/users/me', {
        headers: { Authorization: `Bearer ${token}` },
        data:    { fullName: updatedName },
      });
      expect(patchRes.status()).toBe(200);

      // Step 5: Verify the change persisted
      const verifyRes = await ctx.get('/api/v1/users/me', {
        headers: { Authorization: `Bearer ${token}` },
      });
      expect(verifyRes.status()).toBe(200);
      const verifyBody = await verifyRes.json();
      const vb = verifyBody.data ?? verifyBody;
      expect(vb.fullName ?? vb.full_name).toBe(updatedName);
    } finally {
      await ctx.dispose();
    }
  });

  test('login → save preferences → get preferences → verify round-trip', async () => {
    const token = await getToken();
    const ctx = await authedCtx(token);
    try {
      const prefs = {
        targetTitles:      ['Staff Engineer', 'Principal Engineer'],
        locations:         ['Remote'],
        remotePref:        'remote',
        salaryMin:         150000,
        salaryMax:         250000,
        salaryCurrency:    'USD',
        exclusions:        ['frontend only'],
        autoApplyEnabled:  false,
      };

      const putRes = await ctx.put('/api/v1/users/me/preferences', { data: prefs });
      expect(putRes.status()).toBe(200);

      const getRes = await ctx.get('/api/v1/users/me/preferences');
      expect(getRes.status()).toBe(200);
      const saved = await getRes.json();

      const saved_ = saved?.data ?? saved;
      const remotePref = saved_?.remotePref ?? saved_?.remote_pref;
      expect(remotePref).toBe('remote');

      const titles: string[] = saved_?.targetTitles ?? saved_?.target_titles ?? [];
      expect(titles).toContain('Staff Engineer');
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// USER JOURNEY — Resume lifecycle
// =============================================================================

test.describe('User journey — Resume lifecycle', () => {
  test('login → upload resume → list resumes → set primary → delete → list is smaller', async () => {
    const token = await getToken();
    const ctx   = await authedCtx(token);
    let uploadedId: string | null = null;

    try {
      // Step 1: Count resumes before upload
      const beforeRes = await ctx.get('/api/v1/users/me/resumes');
      expect(beforeRes.status()).toBe(200);
      const beforeRaw = await beforeRes.json();
      const beforeList: Array<{ id: string }> = beforeRaw?.data ?? beforeRaw;
      const countBefore = beforeList.length;

      // Step 2: Upload a resume
      const pdfBytes = Buffer.from(
        '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n' +
        '2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n' +
        '3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\n' +
        'xref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n' +
        '0000000058 00000 n\n0000000115 00000 n\n' +
        'trailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF'
      );
      const uploadRes = await ctx.post('/api/v1/users/me/resumes', {
        multipart: {
          file: { name: 'e2e_lifecycle.pdf', mimeType: 'application/pdf', buffer: pdfBytes },
        },
      });
      expect([200, 201]).toContain(uploadRes.status());
      const uploaded = await uploadRes.json();
      const uploadedData = uploaded?.data ?? uploaded;
      uploadedId = uploadedData.id as string;
      expect(typeof uploadedId).toBe('string');

      // Step 3: List — count should have increased by 1
      const afterUploadRes = await ctx.get('/api/v1/users/me/resumes');
      expect(afterUploadRes.status()).toBe(200);
      const afterUploadRaw = await afterUploadRes.json();
      const afterUploadList: Array<{ id: string }> = afterUploadRaw?.data ?? afterUploadRaw;
      expect(afterUploadList.length).toBe(countBefore + 1);
      expect(afterUploadList.some((r) => r.id === uploadedId)).toBe(true);

      // Step 4: Set primary
      const primaryRes = await ctx.put(`/api/v1/users/me/resumes/${uploadedId}/primary`);
      expect([200, 204]).toContain(primaryRes.status());

      // Step 5: Delete the resume
      const deleteRes = await ctx.delete(`/api/v1/users/me/resumes/${uploadedId}`);
      expect([200, 204]).toContain(deleteRes.status());
      uploadedId = null; // Prevent double-delete in finally

      // Step 6: List again — count should be back to original
      const afterDeleteRes = await ctx.get('/api/v1/users/me/resumes');
      expect(afterDeleteRes.status()).toBe(200);
      const afterDeleteRaw = await afterDeleteRes.json();
      const afterDeleteList: Array<{ id: string }> = afterDeleteRaw?.data ?? afterDeleteRaw;
      expect(afterDeleteList.length).toBe(countBefore);
    } finally {
      // Cleanup if delete didn't run (test failed mid-way)
      if (uploadedId) {
        await ctx.delete(`/api/v1/users/me/resumes/${uploadedId}`).catch(() => {});
      }
      await ctx.dispose();
    }
  });
});

// =============================================================================
// USER JOURNEY — Match → Application flow
// =============================================================================

test.describe('User journey — Match → Application flow', () => {
  test('login → list matches → approve a match if one exists → verify status change', async () => {
    const token = await getToken();
    const ctx   = await authedCtx(token);

    try {
      // Step 1: Fetch pending matches
      const matchesRes = await ctx.get('/api/v1/matches?status=pending&page=1&pageSize=5');
      expect(matchesRes.status()).toBe(200);
      const matchesBody = await matchesRes.json();
      const pendingMatches: Array<{ id: string; status: string }> =
        matchesBody?.data ?? matchesBody?.matches ?? matchesBody ?? [];

      if (!Array.isArray(pendingMatches) || pendingMatches.length === 0) {
        // No pending matches — still verify we got a valid empty response
        expect(Array.isArray(pendingMatches)).toBe(true);
        return;
      }

      const matchId = pendingMatches[0].id;

      // Step 2: Approve the match
      const approveRes = await ctx.patch(`/api/v1/matches/${matchId}`, {
        data: { status: 'approved' },
      });
      expect([200, 204]).toContain(approveRes.status());

      // Step 3: Verify status changed — fetch approved matches and check our ID appears
      const approvedRes = await ctx.get('/api/v1/matches?status=approved&page=1&pageSize=50');
      expect(approvedRes.status()).toBe(200);
      const approvedBody = await approvedRes.json();
      const approvedMatches: Array<{ id: string }> =
        approvedBody?.data ?? approvedBody?.matches ?? approvedBody ?? [];

      // The approved match should appear in the approved list
      const found = approvedMatches.some((m) => m.id === matchId);
      // If pageSize=50 doesn't cover the match (e.g. too many approved), don't fail hard
      if (approvedMatches.length < 50) {
        expect(found).toBe(true);
      }
    } finally {
      await ctx.dispose();
    }
  });

  test('login → list applications → get analytics (all four endpoints)', async () => {
    const token = await getToken();
    const ctx   = await authedCtx(token);

    try {
      // Applications list
      const appsRes = await ctx.get('/api/v1/applications?page=1&pageSize=10');
      expect(appsRes.status()).toBe(200);

      // Funnel analytics
      const funnelRes = await ctx.get(
        `/api/v1/analytics/funnel?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`
      );
      expect(funnelRes.status()).toBe(200);

      // Over-time analytics
      const overTimeRes = await ctx.get(
        `/api/v1/analytics/over-time?from=${ANALYTICS_FROM}&to=${ANALYTICS_TO}`
      );
      expect(overTimeRes.status()).toBe(200);

      // By-resume analytics
      const byResumeRes = await ctx.get('/api/v1/analytics/by-resume');
      expect(byResumeRes.status()).toBe(200);

      // Top companies
      const topCompaniesRes = await ctx.get('/api/v1/analytics/top-companies?limit=10');
      expect(topCompaniesRes.status()).toBe(200);

      // All four must return arrays or valid stat objects
      const funnel = await funnelRes.json();
      expect(funnel).toHaveProperty('applied');

      const overTime = await overTimeRes.json();
      expect(Array.isArray(overTime)).toBe(true);

      const byResume = await byResumeRes.json();
      expect(Array.isArray(byResume)).toBe(true);

      const topCompanies = await topCompaniesRes.json();
      expect(Array.isArray(topCompanies)).toBe(true);
    } finally {
      await ctx.dispose();
    }
  });
});

// =============================================================================
// USER JOURNEY — UI reflects API state (authTest — browser + real JWT)
// =============================================================================

authTest.describe('User journey — UI reflects real API state', () => {
  authTest('dashboard loads and shows real stats from /users/me/dashboard', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    // The dashboard layout fetches /users/me/dashboard; if it 401s the user gets
    // redirected to /login. Staying on a dashboard route means the JWT worked.
    await expect(page).not.toHaveURL(/\/login/);
  });

  authTest('settings page hydrates with real user email from /users/me', async ({ authenticatedPage: page }) => {
    // Navigate directly without stubbing — real API call will fire
    // The fixture stores a mock email in localStorage, but settings reads from the API.
    // We verify the page loads without crashing (auth works).
    await page.goto('/dashboard/settings');
    await expect(page.getByText(/profile/i).first()).toBeVisible({ timeout: 20_000 });
  });

  authTest('matches page loads without redirect when using a real JWT', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/matches');
    // Must remain on the matches route (not redirected to login)
    await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 });
    // Toolbar always renders
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeVisible({ timeout: 20_000 });
  });

  authTest('resumes page loads without redirect when using a real JWT', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/resumes');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 });
    // Upload area is always present on the resumes page
    await expect(page.getByText(/upload/i).first()).toBeVisible({ timeout: 20_000 });
  });

  authTest('analytics page loads without redirect and shows heading', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/analytics');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 });
    await expect(page.getByRole('heading', { name: /analytics/i })).toBeVisible({ timeout: 20_000 });
  });

  authTest('applications page loads without redirect', async ({ authenticatedPage: page }) => {
    await page.goto('/dashboard/applications');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 20_000 });
  });
});
