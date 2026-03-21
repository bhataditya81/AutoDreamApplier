import React from 'react';
import { render } from '@testing-library/react';
import { Spinner } from '@/components/ui/spinner';

describe('Spinner', () => {
  it('renders an SVG element', () => {
    const { container } = render(<Spinner />);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
  });

  it('has aria-hidden', () => {
    const { container } = render(<Spinner />);
    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('defaults to medium size (h-6 w-6)', () => {
    const { container } = render(<Spinner />);
    const svg = container.querySelector('svg');
    // SVG elements have className as SVGAnimatedString in jsdom — use getAttribute
    expect(svg?.getAttribute('class')).toMatch(/h-6/);
    expect(svg?.getAttribute('class')).toMatch(/w-6/);
  });

  it('renders sm size (h-4 w-4)', () => {
    const { container } = render(<Spinner size="sm" />);
    const svg = container.querySelector('svg');
    expect(svg?.getAttribute('class')).toMatch(/h-4/);
  });

  it('renders lg size (h-8 w-8)', () => {
    const { container } = render(<Spinner size="lg" />);
    const svg = container.querySelector('svg');
    expect(svg?.getAttribute('class')).toMatch(/h-8/);
  });

  it('accepts additional className', () => {
    const { container } = render(<Spinner className="custom-spinner" />);
    const svg = container.querySelector('svg');
    expect(svg?.getAttribute('class')).toMatch(/custom-spinner/);
  });
});
