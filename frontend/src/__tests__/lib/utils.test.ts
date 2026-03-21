import { cn, formatSalary, timeAgo, formatDate, statusLabel, scoreColor, scorePercent } from '@/lib/utils';

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar');
  });

  it('handles conditional classes', () => {
    expect(cn('foo', false && 'bar', 'baz')).toBe('foo baz');
  });

  it('resolves Tailwind conflicts (last wins)', () => {
    const result = cn('p-4', 'p-8');
    expect(result).toBe('p-8');
  });

  it('handles arrays', () => {
    expect(cn(['foo', 'bar'])).toBe('foo bar');
  });

  it('handles undefined/null values', () => {
    expect(cn('foo', undefined, null, 'bar')).toBe('foo bar');
  });
});

describe('formatSalary', () => {
  it('returns "Salary not listed" when both min and max are undefined', () => {
    expect(formatSalary()).toBe('Salary not listed');
  });

  it('returns "Salary not listed" when both min and max are 0', () => {
    expect(formatSalary(0, 0)).toBe('Salary not listed');
  });

  it('formats both min and max in USD', () => {
    expect(formatSalary(80000, 120000)).toBe('$80k – $120k');
  });

  it('formats only min', () => {
    expect(formatSalary(80000, undefined)).toBe('From $80k');
  });

  it('formats only max', () => {
    expect(formatSalary(undefined, 120000)).toBe('Up to $120k');
  });

  it('handles values under 1000 (not abbreviated)', () => {
    expect(formatSalary(500, 999)).toBe('$500 – $999');
  });

  it('uses a non-USD currency symbol', () => {
    expect(formatSalary(80000, 120000, 'GBP')).toBe('GBP 80k – GBP 120k');
  });

  it('rounds to nearest k', () => {
    expect(formatSalary(85500, 119999)).toBe('$86k – $120k');
  });
});

describe('timeAgo', () => {
  function isoSecondsAgo(seconds: number): string {
    return new Date(Date.now() - seconds * 1000).toISOString();
  }

  it('returns "just now" for sub-60-second timestamps', () => {
    expect(timeAgo(isoSecondsAgo(30))).toBe('just now');
  });

  it('returns minutes ago', () => {
    expect(timeAgo(isoSecondsAgo(120))).toBe('2m ago');
  });

  it('returns hours ago', () => {
    expect(timeAgo(isoSecondsAgo(7200))).toBe('2h ago');
  });

  it('returns days ago for less than 7 days', () => {
    expect(timeAgo(isoSecondsAgo(3 * 24 * 3600))).toBe('3d ago');
  });

  it('returns a local date string for 7+ days ago', () => {
    const old = new Date(Date.now() - 8 * 24 * 3600 * 1000).toISOString();
    const result = timeAgo(old);
    // Should be a locale date string (not a relative label)
    expect(result).not.toMatch(/ago/);
    expect(result.length).toBeGreaterThan(0);
  });
});

describe('formatDate', () => {
  it('returns "—" for undefined input', () => {
    expect(formatDate()).toBe('—');
    expect(formatDate(undefined)).toBe('—');
  });

  it('returns a formatted date string for a valid ISO date', () => {
    // We test that it's non-empty and not "—"
    const result = formatDate('2024-06-15T00:00:00.000Z');
    expect(result).not.toBe('—');
    expect(result.length).toBeGreaterThan(0);
    expect(result).toMatch(/2024/);
  });
});

describe('statusLabel', () => {
  const cases: [string, string][] = [
    ['queued',       'Queued'],
    ['ai_preparing', 'AI Preparing'],
    ['ai_ready',     'AI Ready'],
    ['applying',     'Applying'],
    ['applied',      'Applied'],
    ['failed',       'Failed'],
    ['withdrawn',    'Withdrawn'],
  ];

  test.each(cases)('statusLabel("%s") === "%s"', (input, expected) => {
    expect(statusLabel(input)).toBe(expected);
  });

  it('returns the raw status for unknown values', () => {
    expect(statusLabel('unknown_status')).toBe('unknown_status');
  });
});

describe('scoreColor', () => {
  it('returns green for score >= 0.8', () => {
    expect(scoreColor(0.9)).toBe('text-green-600');
    expect(scoreColor(0.8)).toBe('text-green-600');
  });

  it('returns yellow for score >= 0.6 and < 0.8', () => {
    expect(scoreColor(0.7)).toBe('text-yellow-600');
    expect(scoreColor(0.6)).toBe('text-yellow-600');
  });

  it('returns red for score < 0.6', () => {
    expect(scoreColor(0.5)).toBe('text-red-500');
    expect(scoreColor(0)).toBe('text-red-500');
  });
});

describe('scorePercent', () => {
  it('converts score to percent string', () => {
    expect(scorePercent(0.75)).toBe('75%');
    expect(scorePercent(1)).toBe('100%');
    expect(scorePercent(0)).toBe('0%');
  });

  it('rounds correctly', () => {
    expect(scorePercent(0.856)).toBe('86%');
  });
});
