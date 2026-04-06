import React from 'react';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MatchCard } from '@/components/matches/match-card';
import type { Match } from '@/lib/types';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

// Use the manual mock in frontend/__mocks__/framer-motion.js so that
// animation primitives (useMotionValue, animate) resolve synchronously in jsdom.
jest.mock('framer-motion');

// Stub the SalaryBenchmarkBadge so it never fires a fetch call during these tests.
// The badge has its own dedicated test suite (salary-benchmark-badge.test.tsx).
jest.mock('@/components/salary/salary-benchmark-badge', () => ({
  SalaryBenchmarkBadge: () => null,
}));

// Radix Slot passes children as an array [false, element] when Button has a
// loading spinner guard before children; React.Children.count sees count=2 and
// throws. Replace Slot with a simple passthrough so asChild works in jsdom.
// Factory uses require() so it is self-contained after jest.mock() hoisting.
jest.mock('@radix-ui/react-slot', () => {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const R = require('react') as typeof import('react');
  const Slot = R.forwardRef(
    ({ children, ...props }: React.PropsWithChildren<React.HTMLAttributes<HTMLElement>>, ref: React.Ref<HTMLElement>) => {
      const child = R.Children.toArray(children)[0];
      if (!R.isValidElement(child)) return null;
      return R.cloneElement(child as React.ReactElement, { ...props, ref });
    }
  );
  Slot.displayName = 'Slot';
  return { Slot };
});

function mockFetch(body: unknown, status = 200) {
  return jest.spyOn(global, 'fetch').mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

const mockMatch: Match = {
  id: 'match-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchScore: 0.87,
  matchBreakdown: { title: 0.9, location: 0.8, salary: 0.85 },
  status: 'pending',
  createdAt: new Date(Date.now() - 3600000).toISOString(),
  updatedAt: new Date(Date.now() - 3600000).toISOString(),
  job: {
    id: 'job-1',
    externalId: 'ext-1',
    sourceBoard: 'indeed',
    url: 'https://example.com/job',
    title: 'Senior Frontend Engineer',
    company: 'TechCorp',
    location: 'San Francisco, CA',
    isRemote: true,
    salaryMin: 120000,
    salaryMax: 160000,
    salaryCurrency: 'USD',
    description: 'Build cool stuff',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

describe('MatchCard', () => {
  it('renders job title', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('Senior Frontend Engineer')).toBeInTheDocument();
  });

  it('renders company name', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('TechCorp')).toBeInTheDocument();
  });

  it('renders match score', async () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    // AnimatedScore drives its display via useEffect + useState (mocked animate
    // immediately calls setVal).  In React 18 the state update may be deferred;
    // wrapping in act + a zero-delay promise flushes all pending microtasks and
    // scheduled state updates before we assert.
    await act(async () => {
      await new Promise<void>(resolve => setTimeout(resolve, 0));
    });
    // RTL's getByText uses getNodeText (direct text-node children only), so
    // "87%" split across a child <span> and a "%" text-node won't match a
    // plain string.  Use textContent (recursive) directly on the score pill.
    const scoreEl = document.querySelector('.tabular-nums');
    expect(scoreEl?.textContent).toBe('87%');
  });

  it('renders "Remote" when isRemote is true', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    // Both the meta row and the Remote badge render the text "Remote"
    const remoteElements = screen.getAllByText('Remote');
    expect(remoteElements.length).toBeGreaterThanOrEqual(1);
  });

  it('renders salary range', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('$120k – $160k')).toBeInTheDocument();
  });

  it('renders Approve (Apply) and Skip buttons', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /skip/i })).toBeInTheDocument();
  });

  it('renders source board badge', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('indeed')).toBeInTheDocument();
  });

  it('renders ATS badge', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('greenhouse')).toBeInTheDocument();
  });

  it('calls onUpdate after clicking Approve', async () => {
    const onUpdate = jest.fn();
    mockFetch({ id: 'match-1', status: 'approved' });
    render(<MatchCard match={mockMatch} onUpdate={onUpdate} />);
    await userEvent.click(screen.getByRole('button', { name: /apply/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith({ id: 'match-1', status: 'approved' }));
  });

  it('calls onUpdate after clicking Skip (reject)', async () => {
    const onUpdate = jest.fn();
    mockFetch({ id: 'match-1', status: 'rejected' });
    render(<MatchCard match={mockMatch} onUpdate={onUpdate} />);
    await userEvent.click(screen.getByRole('button', { name: /skip/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith({ id: 'match-1', status: 'rejected' }));
  });

  it('does NOT render selection checkbox when onToggleSelect is not provided', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.queryByRole('button', { name: /select|deselect/i })).not.toBeInTheDocument();
  });

  it('renders selection checkbox when onToggleSelect is provided', () => {
    render(
      <MatchCard
        match={mockMatch}
        onUpdate={jest.fn()}
        onToggleSelect={jest.fn()}
        selected={false}
      />
    );
    expect(screen.getByRole('button', { name: /select/i })).toBeInTheDocument();
  });

  it('calls onToggleSelect with the match id when checkbox is clicked', async () => {
    const onToggleSelect = jest.fn();
    render(
      <MatchCard
        match={mockMatch}
        onUpdate={jest.fn()}
        onToggleSelect={onToggleSelect}
        selected={false}
      />
    );
    await userEvent.click(screen.getByRole('button', { name: /select/i }));
    expect(onToggleSelect).toHaveBeenCalledWith('match-1');
  });

  it('renders feedback (thumbs up/down) buttons', () => {
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByRole('button', { name: /good match/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /poor match/i })).toBeInTheDocument();
  });

  it('renders salary range when both salaryMin and salaryMax are present', () => {
    // mockMatch already has salaryMin: 120000 and salaryMax: 160000
    render(<MatchCard match={mockMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('$120k – $160k')).toBeInTheDocument();
  });

  it('renders "Salary not listed" when salaryMin and salaryMax are both absent', () => {
    const matchWithoutSalary: Match = {
      ...mockMatch,
      job: {
        ...mockMatch.job,
        salaryMin: undefined,
        salaryMax: undefined,
      },
    };
    render(<MatchCard match={matchWithoutSalary} onUpdate={jest.fn()} />);
    expect(screen.getByText('Salary not listed')).toBeInTheDocument();
  });

  it('renders job location when isRemote is false', () => {
    const onsiteMatch: Match = {
      ...mockMatch,
      job: {
        ...mockMatch.job,
        isRemote: false,
        location: 'New York, NY',
      },
    };
    render(<MatchCard match={onsiteMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('New York, NY')).toBeInTheDocument();
  });

  it('renders Potential Scam badge when job.isScam is true', () => {
    const scamMatch: Match = {
      ...mockMatch,
      job: { ...mockMatch.job, isScam: true },
    };
    render(<MatchCard match={scamMatch} onUpdate={jest.fn()} />);
    expect(screen.getByText('Potential Scam')).toBeInTheDocument();
  });

  it('calls onUpdate with rejected status after Skip', async () => {
    const onUpdate = jest.fn();
    mockFetch({ id: 'match-1', status: 'rejected' });
    render(<MatchCard match={mockMatch} onUpdate={onUpdate} />);
    await userEvent.click(screen.getByRole('button', { name: /skip/i }));
    await waitFor(() =>
      expect(onUpdate).toHaveBeenCalledWith({ id: 'match-1', status: 'rejected' })
    );
  });
});
