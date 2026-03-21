import React from 'react';
import { render, screen } from '@testing-library/react';
import { ApplicationRow } from '@/components/applications/application-row';
import type { Application } from '@/lib/types';

// Mock next/link
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockApplication: Application = {
  id: 'app-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchId: 'match-1',
  resumeId: 'resume-1',
  status: 'applied',
  createdAt: '2024-06-15T00:00:00Z',
  job: {
    id: 'job-1',
    externalId: 'ext-1',
    sourceBoard: 'indeed',
    url: 'https://example.com/job',
    title: 'Software Engineer',
    company: 'Acme Corp',
    location: 'New York, NY',
    isRemote: false,
    salaryCurrency: 'USD',
    description: 'A great job',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2024-01-01T00:00:00Z',
    discoveredAt: '2024-01-02T00:00:00Z',
  },
};

describe('ApplicationRow', () => {
  it('renders job title', () => {
    render(<ApplicationRow application={mockApplication} />);
    expect(screen.getByText('Software Engineer')).toBeInTheDocument();
  });

  it('renders company name', () => {
    render(<ApplicationRow application={mockApplication} />);
    expect(screen.getByText('Acme Corp')).toBeInTheDocument();
  });

  it('renders location when not remote', () => {
    render(<ApplicationRow application={mockApplication} />);
    expect(screen.getByText('New York, NY')).toBeInTheDocument();
  });

  it('renders "Remote" when isRemote is true', () => {
    const remoteApp = {
      ...mockApplication,
      job: { ...mockApplication.job, isRemote: true },
    };
    render(<ApplicationRow application={remoteApp} />);
    expect(screen.getByText('Remote')).toBeInTheDocument();
  });

  it('renders the Details link pointing to the correct route', () => {
    render(<ApplicationRow application={mockApplication} />);
    const link = screen.getByRole('link', { name: /details/i });
    expect(link).toHaveAttribute('href', '/dashboard/applications/app-1');
  });

  it('renders the status timeline', () => {
    render(<ApplicationRow application={mockApplication} />);
    // StatusTimeline renders step labels
    expect(screen.getByText('Applied')).toBeInTheDocument();
  });

  it('renders ATS type badge when atsType is not "unknown"', () => {
    render(<ApplicationRow application={mockApplication} />);
    expect(screen.getByText('greenhouse')).toBeInTheDocument();
  });

  it('does not render ATS badge when atsType is "unknown"', () => {
    const app = {
      ...mockApplication,
      job: { ...mockApplication.job, atsType: 'unknown' },
    };
    render(<ApplicationRow application={app} />);
    expect(screen.queryByText('unknown')).not.toBeInTheDocument();
  });

  it('renders error message when errorMessage is set', () => {
    const app = { ...mockApplication, errorMessage: 'Form submission failed' };
    render(<ApplicationRow application={app} />);
    expect(screen.getByText('Form submission failed')).toBeInTheDocument();
  });
});
