import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Button } from '@/components/ui/button';

describe('Button', () => {
  it('renders children', () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole('button', { name: /click me/i })).toBeInTheDocument();
  });

  it('calls onClick when clicked', async () => {
    const handler = jest.fn();
    render(<Button onClick={handler}>Press</Button>);
    await userEvent.click(screen.getByRole('button'));
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('is disabled when disabled prop is set', () => {
    render(<Button disabled>Disabled</Button>);
    expect(screen.getByRole('button')).toBeDisabled();
  });

  it('is disabled and shows spinner when loading=true', () => {
    render(<Button loading>Saving</Button>);
    const btn = screen.getByRole('button');
    expect(btn).toBeDisabled();
    // Loading spinner is an SVG with aria-hidden
    const svg = btn.querySelector('svg[aria-hidden="true"]');
    expect(svg).toBeInTheDocument();
  });

  it('does not call onClick when disabled', async () => {
    const handler = jest.fn();
    render(<Button disabled onClick={handler}>Nope</Button>);
    await userEvent.click(screen.getByRole('button'), { pointerEventsCheck: 0 });
    expect(handler).not.toHaveBeenCalled();
  });

  it('applies variant class (destructive)', () => {
    render(<Button variant="destructive">Delete</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toMatch(/red/);
  });

  it('applies size class (sm)', () => {
    render(<Button size="sm">Small</Button>);
    const btn = screen.getByRole('button');
    expect(btn.className).toMatch(/h-8/);
  });
});
