import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Switch } from '@/components/ui/switch';

describe('Switch', () => {
  it('renders a switch element', () => {
    render(<Switch />);
    expect(screen.getByRole('switch')).toBeInTheDocument();
  });

  it('reflects unchecked state by default', () => {
    render(<Switch />);
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'false');
  });

  it('reflects checked state when checked=true', () => {
    render(<Switch checked />);
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('calls onCheckedChange when toggled', async () => {
    const handler = jest.fn();
    render(<Switch checked={false} onCheckedChange={handler} />);
    await userEvent.click(screen.getByRole('switch'));
    expect(handler).toHaveBeenCalledWith(true);
  });

  it('is disabled when disabled prop is set', () => {
    render(<Switch disabled />);
    expect(screen.getByRole('switch')).toBeDisabled();
  });
});
