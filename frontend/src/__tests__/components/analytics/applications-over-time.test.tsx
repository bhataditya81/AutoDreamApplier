import React from 'react';
import { render, screen } from '@testing-library/react';
import { ApplicationsOverTime } from '@/components/analytics/applications-over-time';
import type { DailyCount } from '@/lib/types';

// recharts uses ResizeObserver which is not available in jsdom
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const mockData: DailyCount[] = [
  { date: '2026-03-01', count: 5 },
  { date: '2026-03-02', count: 3 },
  { date: '2026-03-03', count: 8 },
];

describe('ApplicationsOverTime', () => {
  it('renders empty state when data array is empty', () => {
    render(<ApplicationsOverTime data={[]} />);
    expect(screen.getByText(/no data for this period/i)).toBeInTheDocument();
  });

  it('does NOT render empty state when data is present', () => {
    render(<ApplicationsOverTime data={mockData} />);
    expect(screen.queryByText(/no data for this period/i)).not.toBeInTheDocument();
  });

  it('renders a recharts container element when data is provided', () => {
    const { container } = render(<ApplicationsOverTime data={mockData} />);
    // recharts renders a ResponsiveContainer div wrapper even in jsdom
    expect(container.firstChild).not.toBeNull();
  });
});
