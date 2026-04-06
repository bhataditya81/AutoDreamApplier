import React from 'react';
import { render } from '@testing-library/react';
import { Spinner } from '@/components/ui/spinner';

// Use the manual mock so motion.div renders as a plain <div> in jsdom.
jest.mock('framer-motion');

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

  // The Spinner wraps the SVG in a motion.div that carries the size classes.
  // We query the first element child (the div) for size-related assertions.

  it('defaults to medium size (h-6 w-6)', () => {
    const { container } = render(<Spinner />);
    const wrapper = container.firstElementChild;
    expect(wrapper?.getAttribute('class')).toMatch(/h-6/);
    expect(wrapper?.getAttribute('class')).toMatch(/w-6/);
  });

  it('renders sm size (h-4 w-4)', () => {
    const { container } = render(<Spinner size="sm" />);
    const wrapper = container.firstElementChild;
    expect(wrapper?.getAttribute('class')).toMatch(/h-4/);
  });

  it('renders lg size (h-8 w-8)', () => {
    const { container } = render(<Spinner size="lg" />);
    const wrapper = container.firstElementChild;
    expect(wrapper?.getAttribute('class')).toMatch(/h-8/);
  });

  it('accepts additional className', () => {
    const { container } = render(<Spinner className="custom-spinner" />);
    const wrapper = container.firstElementChild;
    expect(wrapper?.getAttribute('class')).toMatch(/custom-spinner/);
  });
});
