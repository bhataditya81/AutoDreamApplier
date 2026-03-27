import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OutcomeEntryModal } from '@/components/analytics/outcome-entry-modal';
import type { Application } from '@/lib/types';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

function mockFetch(body: unknown, status = 200) {
  return jest.spyOn(global, 'fetch').mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

const mockApplication: Application = {
  id: 'app-1',
  userId: 'user-1',
  jobId: 'job-1',
  matchId: 'match-1',
  resumeId: 'resume-1',
  status: 'applied',
  createdAt: '2026-01-01T00:00:00Z',
  job: {
    id: 'job-1',
    externalId: 'ext-1',
    sourceBoard: 'indeed',
    url: 'https://example.com/job',
    title: 'Software Engineer',
    company: 'Acme Corp',
    location: 'NYC',
    isRemote: false,
    salaryCurrency: 'USD',
    description: 'Great job',
    atsType: 'greenhouse',
    applyUrl: 'https://example.com/apply',
    isScam: false,
    postedAt: '2026-01-01T00:00:00Z',
    discoveredAt: '2026-01-01T00:00:00Z',
  },
};

describe('OutcomeEntryModal', () => {
  it('renders the modal title when open', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.getByText('How did it go?')).toBeInTheDocument();
  });

  it('does not render modal content when closed', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={false}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.queryByText('How did it go?')).not.toBeInTheDocument();
  });

  it('renders company name and job title in description', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.getByText('Acme Corp')).toBeInTheDocument();
    expect(screen.getByText('Software Engineer')).toBeInTheDocument();
  });

  it('renders all four outcome options', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.getByText('They viewed my profile')).toBeInTheDocument();
    expect(screen.getByText('I got an interview')).toBeInTheDocument();
    expect(screen.getByText('I received an offer')).toBeInTheDocument();
    expect(screen.getByText('I was rejected / ghosted')).toBeInTheDocument();
  });

  it('Save outcome button is disabled when no outcome selected', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.getByRole('button', { name: /save outcome/i })).toBeDisabled();
  });

  it('Save outcome button is enabled after selecting an outcome', async () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    await userEvent.click(screen.getByText('I got an interview'));
    expect(screen.getByRole('button', { name: /save outcome/i })).not.toBeDisabled();
  });

  it('calls onSaved and onClose after successful save', async () => {
    mockFetch(undefined, 204);
    const onSaved = jest.fn();
    const onClose = jest.fn();

    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={onClose}
        onSaved={onSaved}
      />
    );

    await userEvent.click(screen.getByText('I received an offer'));
    await userEvent.click(screen.getByRole('button', { name: /save outcome/i }));

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it('shows error message when API call fails', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({ message: 'Server error' }),
    } as Response);

    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );

    await userEvent.click(screen.getByText('I got an interview'));
    await userEvent.click(screen.getByRole('button', { name: /save outcome/i }));

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeInTheDocument();
    });
  });

  it('calls onClose when Skip button is clicked', async () => {
    const onClose = jest.fn();
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={onClose}
        onSaved={jest.fn()}
      />
    );
    await userEvent.click(screen.getByRole('button', { name: /skip/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('renders optional notes textarea', () => {
    render(
      <OutcomeEntryModal
        application={mockApplication}
        open={true}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    );
    expect(screen.getByPlaceholderText(/scheduled interview/i)).toBeInTheDocument();
  });
});
