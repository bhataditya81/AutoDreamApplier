/**
 * Unit tests for src/lib/api.ts
 * Uses jest.spyOn(global, 'fetch') to mock the fetch layer.
 */

// Mock auth so we don't need localStorage
jest.mock('@/lib/auth', () => ({
  getToken: jest.fn(() => 'test-token'),
}));

import {
  listMatches,
  updateMatchStatus,
  bulkUpdateMatches,
  setMatchFeedback,
  listApplications,
  getApplication,
  withdrawApplication,
  listResumes,
  uploadResume,
  setPrimaryResume,
  deleteResume,
  getMe,
  updateMe,
  getPreferences,
  savePreferences,
  saveBoardCredentials,
  getDashboardStats,
} from '@/lib/api';

// ── Fixtures ──────────────────────────────────────────────────────────────────

const mockJob = {
  id: 'job-1',
  externalId: 'ext-1',
  sourceBoard: 'indeed',
  url: 'https://example.com/job',
  title: 'Software Engineer',
  company: 'Acme Corp',
  location: 'NYC',
  isRemote: false,
  salaryCurrency: 'USD',
  description: 'A great job',
  atsType: 'greenhouse',
  applyUrl: 'https://example.com/apply',
  isScam: false,
  postedAt: '2024-01-01T00:00:00Z',
  discoveredAt: '2024-01-02T00:00:00Z',
};

const mockMatch = {
  id: 'match-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchScore: 0.85,
  matchBreakdown: { title: 0.9, location: 0.8, salary: 0.85 },
  status: 'pending',
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
  job: mockJob,
};

const mockApplication = {
  id: 'app-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchId: 'match-1',
  resumeId: 'resume-1',
  status: 'applied',
  createdAt: '2024-01-01T00:00:00Z',
  job: mockJob,
};

const mockResume = {
  id: 'resume-1',
  userId: 'user-1',
  fileName: 'resume.pdf',
  s3Key: 'resumes/user-1/resume.pdf',
  isPrimary: true,
  interviewCount: 2,
  createdAt: '2024-01-01T00:00:00Z',
};

const mockUser = {
  id: 'user-1',
  email: 'test@example.com',
  fullName: 'Test User',
  tier: 'free',
  applyMode: 'review',
  autoThreshold: 0.8,
  dailyLimit: 10,
  isActive: true,
  createdAt: '2024-01-01T00:00:00Z',
};

const mockPrefs = {
  targetTitles: ['Software Engineer'],
  locations: ['Remote'],
  remotePref: 'remote',
  salaryCurrency: 'USD',
  exclusions: [],
};

// ── Helpers ───────────────────────────────────────────────────────────────────

function mockFetch(body: unknown, status = 200): jest.SpyInstance {
  return jest.spyOn(global, 'fetch').mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

function mockFetch204(): jest.SpyInstance {
  return jest.spyOn(global, 'fetch').mockResolvedValueOnce({
    ok: true,
    status: 204,
    json: async () => undefined,
  } as Response);
}

function mockFetchError(body: { message?: string; error?: string }, status: number): jest.SpyInstance {
  return jest.spyOn(global, 'fetch').mockResolvedValueOnce({
    ok: false,
    status,
    json: async () => body,
  } as Response);
}

afterEach(() => {
  jest.restoreAllMocks();
});

// ── listMatches ───────────────────────────────────────────────────────────────

describe('listMatches', () => {
  it('returns paginated matches', async () => {
    mockFetch({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false });
    const result = await listMatches();
    expect(result.data).toHaveLength(1);
    expect(result.data[0].id).toBe('match-1');
  });

  it('sends auth header', async () => {
    const spy = mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    await listMatches();
    expect(spy).toHaveBeenCalledWith(
      expect.stringContaining('/matches'),
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      })
    );
  });

  it('appends query params', async () => {
    const spy = mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    await listMatches({ status: 'pending', page: 1 });
    const url = (spy.mock.calls[0] as [string])[0];
    expect(url).toContain('status=pending');
    expect(url).toContain('page=1');
  });

  it('throws on 500', async () => {
    mockFetchError({ message: 'Internal Server Error' }, 500);
    await expect(listMatches()).rejects.toThrow('Internal Server Error');
  });

  it('throws with HTTP status when no message body', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => { throw new Error('no json'); },
    } as unknown as Response);
    await expect(listMatches()).rejects.toThrow('HTTP 503');
  });
});

// ── updateMatchStatus ─────────────────────────────────────────────────────────

describe('updateMatchStatus', () => {
  it('returns updated match status', async () => {
    mockFetch({ id: 'match-1', status: 'approved' });
    const result = await updateMatchStatus('match-1', 'approved', 'thumbs_up');
    expect(result.status).toBe('approved');
  });

  it('sends PATCH method', async () => {
    const spy = mockFetch({ id: 'match-1', status: 'rejected' });
    await updateMatchStatus('match-1', 'rejected');
    expect(spy).toHaveBeenCalledWith(
      expect.stringContaining('/matches/match-1'),
      expect.objectContaining({ method: 'PATCH' })
    );
  });

  it('throws on 404', async () => {
    mockFetchError({ error: 'not found' }, 404);
    await expect(updateMatchStatus('bad', 'approved')).rejects.toThrow('not found');
  });
});

// ── bulkUpdateMatches ─────────────────────────────────────────────────────────

describe('bulkUpdateMatches', () => {
  it('resolves without throwing on success', async () => {
    mockFetch({ action: 'approve', updated: 3 });
    await expect(bulkUpdateMatches(['m1', 'm2', 'm3'], 'approve')).resolves.toBeUndefined();
  });

  it('throws on error', async () => {
    mockFetchError({ message: 'Validation failed' }, 422);
    await expect(bulkUpdateMatches([], 'approve')).rejects.toThrow('Validation failed');
  });
});

// ── setMatchFeedback ──────────────────────────────────────────────────────────

describe('setMatchFeedback', () => {
  it('resolves without throwing on success', async () => {
    mockFetch({ feedback: 'thumbs_up' });
    await expect(setMatchFeedback('match-1', 'thumbs_up')).resolves.toBeUndefined();
  });
});

// ── listApplications ──────────────────────────────────────────────────────────

describe('listApplications', () => {
  it('returns paginated applications', async () => {
    mockFetch({ data: [mockApplication], total: 1, page: 1, pageSize: 12, hasMore: false });
    const result = await listApplications();
    expect(result.data[0].id).toBe('app-1');
  });

  it('throws on 401', async () => {
    mockFetchError({ message: 'Unauthorized' }, 401);
    await expect(listApplications()).rejects.toThrow('Unauthorized');
  });
});

// ── getApplication ────────────────────────────────────────────────────────────

describe('getApplication', () => {
  it('returns a single application', async () => {
    mockFetch(mockApplication);
    const result = await getApplication('app-1');
    expect(result.id).toBe('app-1');
  });
});

// ── withdrawApplication ───────────────────────────────────────────────────────

describe('withdrawApplication', () => {
  it('resolves on 204', async () => {
    mockFetch204();
    await expect(withdrawApplication('app-1')).resolves.toBeUndefined();
  });
});

// ── listResumes ───────────────────────────────────────────────────────────────

describe('listResumes', () => {
  it('returns resume array', async () => {
    mockFetch([mockResume]);
    const result = await listResumes();
    expect(result).toHaveLength(1);
    expect(result[0].fileName).toBe('resume.pdf');
  });
});

// ── uploadResume ──────────────────────────────────────────────────────────────

describe('uploadResume', () => {
  it('posts FormData and returns Resume', async () => {
    const spy = mockFetch(mockResume);
    const file = new File(['pdf content'], 'resume.pdf', { type: 'application/pdf' });
    const result = await uploadResume(file);
    expect(result.id).toBe('resume-1');
    // Should NOT set Content-Type: application/json for FormData
    const callArgs = spy.mock.calls[0] as [string, RequestInit];
    expect(callArgs[1].headers).not.toHaveProperty('Content-Type');
  });

  it('throws on upload error', async () => {
    mockFetchError({ message: 'File too large' }, 413);
    const file = new File(['x'], 'x.pdf', { type: 'application/pdf' });
    await expect(uploadResume(file)).rejects.toThrow('File too large');
  });
});

// ── setPrimaryResume ──────────────────────────────────────────────────────────

describe('setPrimaryResume', () => {
  it('returns updated resume', async () => {
    mockFetch({ ...mockResume, isPrimary: true });
    const result = await setPrimaryResume('resume-1');
    expect(result.isPrimary).toBe(true);
  });
});

// ── deleteResume ──────────────────────────────────────────────────────────────

describe('deleteResume', () => {
  it('resolves on 204', async () => {
    mockFetch204();
    await expect(deleteResume('resume-1')).resolves.toBeUndefined();
  });
});

// ── getMe ─────────────────────────────────────────────────────────────────────

describe('getMe', () => {
  it('returns user', async () => {
    mockFetch(mockUser);
    const result = await getMe();
    expect(result.email).toBe('test@example.com');
  });

  it('throws on 401', async () => {
    mockFetchError({ error: 'Unauthorized' }, 401);
    await expect(getMe()).rejects.toThrow('Unauthorized');
  });
});

// ── updateMe ──────────────────────────────────────────────────────────────────

describe('updateMe', () => {
  it('sends PATCH and returns updated user', async () => {
    mockFetch({ ...mockUser, fullName: 'New Name' });
    const result = await updateMe({ fullName: 'New Name' });
    expect(result.fullName).toBe('New Name');
  });
});

// ── getPreferences ────────────────────────────────────────────────────────────

describe('getPreferences', () => {
  it('returns preferences', async () => {
    mockFetch(mockPrefs);
    const result = await getPreferences();
    expect(result.targetTitles).toContain('Software Engineer');
  });
});

// ── savePreferences ───────────────────────────────────────────────────────────

describe('savePreferences', () => {
  it('sends PUT and returns saved preferences', async () => {
    mockFetch(mockPrefs);
    const result = await savePreferences(mockPrefs as never);
    expect(result.remotePref).toBe('remote');
  });

  it('sends PUT method', async () => {
    const spy = mockFetch(mockPrefs);
    await savePreferences(mockPrefs as never);
    expect(spy).toHaveBeenCalledWith(
      expect.stringContaining('/preferences'),
      expect.objectContaining({ method: 'PUT' })
    );
  });
});

// ── saveBoardCredentials ──────────────────────────────────────────────────────

describe('saveBoardCredentials', () => {
  it('posts credentials and resolves on 204', async () => {
    mockFetch204();
    await expect(saveBoardCredentials('linkedin', 'user@example.com', 'pass')).resolves.toBeUndefined();
  });

  it('throws on 400', async () => {
    mockFetchError({ message: 'Invalid credentials' }, 400);
    await expect(saveBoardCredentials('bad', '', '')).rejects.toThrow('Invalid credentials');
  });
});

// ── Auth header on every call ─────────────────────────────────────────────────

describe('auth header', () => {
  it('includes Authorization: Bearer token on protected calls', async () => {
    const spy = mockFetch({ data: [], total: 0, page: 1, pageSize: 20, hasMore: false });
    await listApplications();
    expect(spy).toHaveBeenCalledWith(
      expect.stringContaining('/applications'),
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer test-token' }),
      })
    );
  });
});

// ── Network error propagation ──────────────────────────────────────────────────

describe('network error', () => {
  it('propagates when fetch throws (e.g. offline)', async () => {
    jest.spyOn(global, 'fetch').mockRejectedValueOnce(new Error('Failed to fetch'));
    await expect(listMatches()).rejects.toThrow('Failed to fetch');
  });
});

// ── listApplications query params ─────────────────────────────────────────────

describe('listApplications query params', () => {
  it('appends status and page query params to the URL', async () => {
    const spy = mockFetch({ data: [], total: 0, page: 2, pageSize: 20, hasMore: false });
    await listApplications({ status: 'applied', page: 2 });
    const url = (spy.mock.calls[0] as [string])[0];
    expect(url).toContain('status=applied');
    expect(url).toContain('page=2');
  });

  it('calls /api/v1/applications without params when none provided', async () => {
    const spy = mockFetch({ data: [], total: 0, page: 1, pageSize: 20, hasMore: false });
    await listApplications();
    const url = (spy.mock.calls[0] as [string])[0];
    expect(url).toMatch(/\/api\/v1\/applications$/);
  });
});

// ── getDashboardStats ─────────────────────────────────────────────────────────

describe('getDashboardStats', () => {
  it('maps backend stats to frontend DashboardStats shape', async () => {
    mockFetch({
      stats: {
        total_applications: 20,
        applied_today: 5,
        pending_matches: 8,
        interviews_received: 2,
      },
    });
    const result = await getDashboardStats();
    expect(result.pendingMatches).toBe(8);
    expect(result.applicationsThisWeek).toBe(5);
    expect(result.interviewsThisMonth).toBe(2);
    expect(result.successRate).toBe(0);
  });

  it('handles null stats gracefully', async () => {
    mockFetch({ stats: null });
    const result = await getDashboardStats();
    expect(result.pendingMatches).toBe(0);
    expect(result.applicationsThisWeek).toBe(0);
    expect(result.interviewsThisMonth).toBe(0);
  });
});
