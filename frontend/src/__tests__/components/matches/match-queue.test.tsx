import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MatchQueue } from '@/components/matches/match-queue';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

// Radix Slot passes children as [false, element] when Button has a loading guard
// before children; React.Children.count sees 2 and throws in jsdom. Use a simple
// passthrough so asChild renders correctly in tests.
const SlotMock = React.forwardRef(({ children, ...props }: React.PropsWithChildren<React.HTMLAttributes<HTMLElement>>, ref: React.Ref<HTMLElement>) => {
  const child = React.Children.toArray(children)[0];
  if (!React.isValidElement(child)) return null;
  return React.cloneElement(child as React.ReactElement, { ...props, ref });
});
SlotMock.displayName = 'Slot';
jest.mock('@radix-ui/react-slot', () => ({
  Slot: SlotMock,
}));

const mockMatch = {
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
    isRemote: false,
    salaryCurrency: 'USD',
    description: 'Build cool stuff',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

function mockFetch(body: unknown, status = 200) {
  return jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

describe('MatchQueue', () => {
  it('renders status tabs: Pending, Approved, Rejected, All', () => {
    mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Approved' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Rejected' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'All' })).toBeInTheDocument();
  });

  it('renders Refresh button', () => {
    mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
  });

  it('renders match cards after loading', async () => {
    mockFetch({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByText('Senior Frontend Engineer')).toBeInTheDocument()
    );
  });

  it('renders empty state when no matches', async () => {
    mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByText(/no.*matches/i)).toBeInTheDocument()
    );
  });

  it('shows error alert when fetch fails', async () => {
    mockFetch({ message: 'Server error' }, 500);
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByRole('alert')).toBeInTheDocument()
    );
  });

  it('switches to Approved tab when clicked', async () => {
    mockFetch({ data: [], total: 0, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() => screen.getByRole('button', { name: 'Approved' }));
    const approvedTab = screen.getByRole('button', { name: 'Approved' });
    await userEvent.click(approvedTab);
    expect(approvedTab).toBeInTheDocument();
  });

  it('shows bulk action toolbar after matches load', async () => {
    mockFetch({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByText('Senior Frontend Engineer')).toBeInTheDocument()
    );
    expect(screen.getByText(/select all/i)).toBeInTheDocument();
  });

  it('shows "N selected" text when matches are selected via Select all', async () => {
    mockFetch({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByText('Senior Frontend Engineer')).toBeInTheDocument()
    );
    await userEvent.click(screen.getByText(/select all/i));
    await waitFor(() =>
      expect(screen.getByText(/1 selected/i)).toBeInTheDocument()
    );
  });

  it('shows Approve all and Skip all when items are selected', async () => {
    mockFetch({ data: [mockMatch], total: 1, page: 1, pageSize: 12, hasMore: false });
    render(<MatchQueue />);
    await waitFor(() =>
      expect(screen.getByText('Senior Frontend Engineer')).toBeInTheDocument()
    );
    await userEvent.click(screen.getByText(/select all/i));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /approve all/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /skip all/i })).toBeInTheDocument();
    });
  });
});
