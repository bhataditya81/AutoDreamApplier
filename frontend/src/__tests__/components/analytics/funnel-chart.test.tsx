import React from 'react';
import { render, screen } from '@testing-library/react';
import { FunnelChart } from '@/components/analytics/funnel-chart';
import type { FunnelStats } from '@/lib/types';

const mockStats: FunnelStats = {
  applied: 100,
  viewed: 60,
  interviews: 20,
  offers: 5,
  view_rate: 60,
  interview_rate: 20,
  offer_rate: 5,
};

describe('FunnelChart', () => {
  it('renders Applied label', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('Applied')).toBeInTheDocument();
  });

  it('renders Viewed label', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('Viewed')).toBeInTheDocument();
  });

  it('renders Interview label', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('Interview')).toBeInTheDocument();
  });

  it('renders Offer label', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('Offer')).toBeInTheDocument();
  });

  it('renders applied count', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('100')).toBeInTheDocument();
  });

  it('renders viewed count', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('60')).toBeInTheDocument();
  });

  it('renders interview count', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('20')).toBeInTheDocument();
  });

  it('renders offer count', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('renders view rate percentage', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('(60.0%)')).toBeInTheDocument();
  });

  it('renders interview rate percentage', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('(20.0%)')).toBeInTheDocument();
  });

  it('renders offer rate percentage', () => {
    render(<FunnelChart stats={mockStats} />);
    expect(screen.getByText('(5.0%)')).toBeInTheDocument();
  });

  it('does NOT show rate for Applied row (it is the baseline)', () => {
    render(<FunnelChart stats={mockStats} />);
    // There should be exactly 3 percentage displays (viewed, interview, offer)
    const percentages = screen.getAllByText(/\(\d+\.0%\)/);
    expect(percentages).toHaveLength(3);
  });

  it('renders with zero stats without crashing', () => {
    const zeroStats: FunnelStats = {
      applied: 0,
      viewed: 0,
      interviews: 0,
      offers: 0,
      view_rate: 0,
      interview_rate: 0,
      offer_rate: 0,
    };
    render(<FunnelChart stats={zeroStats} />);
    expect(screen.getByText('Applied')).toBeInTheDocument();
  });

  it('renders with large numbers using toLocaleString formatting', () => {
    const largeStats: FunnelStats = {
      applied: 1000,
      viewed: 500,
      interviews: 100,
      offers: 10,
      view_rate: 50,
      interview_rate: 10,
      offer_rate: 1,
    };
    render(<FunnelChart stats={largeStats} />);
    // 1000 renders as "1,000" via toLocaleString
    expect(screen.getByText('1,000')).toBeInTheDocument();
  });
});
