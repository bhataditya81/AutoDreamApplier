import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { Sidebar } from '@/components/layout/sidebar';

jest.mock('@/lib/auth', () => ({
  getToken: jest.fn(() => 'test-token'),
  logout: jest.fn(),
}));

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
  usePathname: () => '/dashboard/matches',
}));

jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode; [key: string]: unknown }) => (
    <a href={href} {...props}>{children as React.ReactNode}</a>
  ),
}));

function mockFetchPrefs(autoApplyEnabled = false) {
  return jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => ({
      targetTitles: [],
      locations: [],
      remotePref: 'any',
      salaryCurrency: 'USD',
      exclusions: [],
      autoApplyEnabled,
    }),
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

describe('Sidebar', () => {
  it('renders the AutoDream logo/brand text', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    expect(screen.getByText('AutoDream')).toBeInTheDocument();
  });

  it('renders all nav links', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    expect(screen.getByRole('link', { name: /match queue/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /applications/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /settings/i })).toBeInTheDocument();
  });

  it('renders Resumes link in footer', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    expect(screen.getByRole('link', { name: /resumes/i })).toBeInTheDocument();
  });

  it('renders Log out button', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    expect(screen.getByRole('button', { name: /log out/i })).toBeInTheDocument();
  });

  it('marks Match Queue link as active (current pathname)', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    const matchLink = screen.getByRole('link', { name: /match queue/i });
    expect(matchLink.className).toMatch(/bg-brand-50/);
  });

  it('does NOT show AUTO badge when autoApplyEnabled is false', async () => {
    mockFetchPrefs(false);
    render(<Sidebar />);
    await waitFor(() => {
      expect(screen.queryByText('AUTO')).not.toBeInTheDocument();
    });
  });

  it('shows AUTO badge when autoApplyEnabled is true', async () => {
    mockFetchPrefs(true);
    render(<Sidebar />);
    await waitFor(() =>
      expect(screen.getByText('AUTO')).toBeInTheDocument()
    );
  });

  it('nav links have correct hrefs', () => {
    mockFetchPrefs();
    render(<Sidebar />);
    const matchLink = screen.getByRole('link', { name: /match queue/i });
    expect(matchLink).toHaveAttribute('href', '/dashboard/matches');

    const appsLink = screen.getByRole('link', { name: /applications/i });
    expect(appsLink).toHaveAttribute('href', '/dashboard/applications');

    const settingsLink = screen.getByRole('link', { name: /settings/i });
    expect(settingsLink).toHaveAttribute('href', '/dashboard/settings');
  });
});
