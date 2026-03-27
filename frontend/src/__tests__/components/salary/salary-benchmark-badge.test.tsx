import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { SalaryBenchmarkBadge } from '@/components/salary/salary-benchmark-badge';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

function mockFetch(body: unknown, status = 200) {
  return jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

const mockBenchmark = {
  title_key: 'software-engineer',
  location_key: 'san-francisco-ca',
  currency: 'USD',
  min: 100000,
  p25: 120000,
  median: 140000,
  p75: 160000,
  max: 200000,
  sample_size: 50,
  updated_at: '2026-01-01T00:00:00Z',
};

describe('SalaryBenchmarkBadge', () => {
  it('renders nothing when API returns unknown market position', async () => {
    mockFetch({ benchmark: null, market_position: 'unknown' });
    const { container } = render(
      <SalaryBenchmarkBadge title="Software Engineer" location="NYC" />
    );
    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });

  it('renders nothing when benchmark is null', async () => {
    mockFetch({ benchmark: null, market_position: 'unknown' });
    const { container } = render(
      <SalaryBenchmarkBadge title="Software Engineer" location="NYC" />
    );
    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });

  it('renders salary range when market position is "above"', async () => {
    mockFetch({ benchmark: mockBenchmark, market_position: 'above' });
    render(
      <SalaryBenchmarkBadge
        title="Software Engineer"
        location="San Francisco, CA"
        salaryMin={170000}
        salaryMax={200000}
      />
    );
    await waitFor(() => {
      expect(screen.getByText(/above market/i)).toBeInTheDocument();
    });
    // p25: $120k – p75: $160k
    expect(screen.getByText(/\$120k/)).toBeInTheDocument();
    expect(screen.getByText(/\$160k/)).toBeInTheDocument();
  });

  it('renders "At market" label when market position is "at"', async () => {
    mockFetch({ benchmark: mockBenchmark, market_position: 'at' });
    render(
      <SalaryBenchmarkBadge
        title="Software Engineer"
        location="San Francisco, CA"
        salaryMin={140000}
        salaryMax={155000}
      />
    );
    await waitFor(() => {
      expect(screen.getByText(/at market/i)).toBeInTheDocument();
    });
  });

  it('renders "Below market" label when market position is "below"', async () => {
    mockFetch({ benchmark: mockBenchmark, market_position: 'below' });
    render(
      <SalaryBenchmarkBadge
        title="Software Engineer"
        location="San Francisco, CA"
        salaryMin={80000}
        salaryMax={100000}
      />
    );
    await waitFor(() => {
      expect(screen.getByText(/below market/i)).toBeInTheDocument();
    });
  });

  it('renders sample size from benchmark', async () => {
    mockFetch({ benchmark: mockBenchmark, market_position: 'at' });
    render(
      <SalaryBenchmarkBadge title="Software Engineer" location="San Francisco, CA" />
    );
    await waitFor(() => {
      expect(screen.getByText(/50 jobs/)).toBeInTheDocument();
    });
  });

  it('renders nothing on API error (silently swallows)', async () => {
    jest.spyOn(global, 'fetch').mockRejectedValueOnce(new Error('Network error'));
    const { container } = render(
      <SalaryBenchmarkBadge title="Software Engineer" location="NYC" />
    );
    // Wait a tick for the async effect to run
    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });

  it('renders nothing while loading (no flash of content)', () => {
    // Never resolve the promise to simulate in-flight state
    jest.spyOn(global, 'fetch').mockReturnValueOnce(new Promise(() => {}));
    const { container } = render(
      <SalaryBenchmarkBadge title="Software Engineer" location="NYC" />
    );
    expect(container.firstChild).toBeNull();
  });

  it('formats salary in millions correctly', async () => {
    const millionBenchmark = {
      ...mockBenchmark,
      p25: 1200000,
      p75: 1500000,
    };
    mockFetch({ benchmark: millionBenchmark, market_position: 'above' });
    render(
      <SalaryBenchmarkBadge title="Software Engineer" location="NYC" salaryMin={2000000} />
    );
    await waitFor(() => {
      expect(screen.getByText(/\$1\.2M/)).toBeInTheDocument();
      expect(screen.getByText(/\$1\.5M/)).toBeInTheDocument();
    });
  });
});
