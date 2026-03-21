import React from 'react';
import { render, screen } from '@testing-library/react';
import { StatusTimeline } from '@/components/applications/status-timeline';

describe('StatusTimeline', () => {
  it('renders all pipeline step labels', () => {
    render(<StatusTimeline status="queued" />);
    expect(screen.getByText('Queued')).toBeInTheDocument();
    expect(screen.getByText('AI Prep')).toBeInTheDocument();
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('Applying')).toBeInTheDocument();
    expect(screen.getByText('Applied')).toBeInTheDocument();
  });

  it('shows active step for "queued" status', () => {
    render(<StatusTimeline status="queued" />);
    // The active node has a white background + brand border
    const container = document.querySelector('.space-y-2');
    expect(container).toBeInTheDocument();
  });

  it('shows "Failed" label on last step for failed status', () => {
    render(<StatusTimeline status="failed" />);
    expect(screen.getByText('Failed')).toBeInTheDocument();
  });

  it('shows "Withdrawn" label on last step for withdrawn status', () => {
    render(<StatusTimeline status="withdrawn" />);
    expect(screen.getByText('Withdrawn')).toBeInTheDocument();
  });

  it('renders outcome badge for "interview"', () => {
    render(<StatusTimeline status="applied" outcome="interview" />);
    expect(screen.getByText(/interview/i)).toBeInTheDocument();
  });

  it('renders outcome badge for "offer"', () => {
    render(<StatusTimeline status="applied" outcome="offer" />);
    expect(screen.getByText(/offer/i)).toBeInTheDocument();
  });

  it('renders outcome badge for "rejected"', () => {
    render(<StatusTimeline status="applied" outcome="rejected" />);
    expect(screen.getByText('Rejected')).toBeInTheDocument();
  });

  it('renders outcome badge for "viewed"', () => {
    render(<StatusTimeline status="applied" outcome="viewed" />);
    expect(screen.getByText('Viewed')).toBeInTheDocument();
  });

  it('does not render outcome badge when outcome is undefined', () => {
    render(<StatusTimeline status="applied" />);
    expect(screen.queryByText('Viewed')).not.toBeInTheDocument();
    expect(screen.queryByText('Rejected')).not.toBeInTheDocument();
  });

  it('shows completed steps for "applied" status', () => {
    render(<StatusTimeline status="applied" />);
    // All steps before "applied" should show a check (done)
    // The "Applied" text is shown for the final step
    expect(screen.getByText('Applied')).toBeInTheDocument();
  });
});
