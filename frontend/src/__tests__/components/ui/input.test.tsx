import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Input } from '@/components/ui/input';

describe('Input', () => {
  it('renders an input element', () => {
    render(<Input />);
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('renders with placeholder', () => {
    render(<Input placeholder="Enter text" />);
    expect(screen.getByPlaceholderText('Enter text')).toBeInTheDocument();
  });

  it('accepts and reflects value changes', async () => {
    const handler = jest.fn();
    render(<Input onChange={handler} />);
    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'hello');
    expect(handler).toHaveBeenCalled();
  });

  it('shows error message when error prop is provided', () => {
    render(<Input error="This field is required" />);
    expect(screen.getByText('This field is required')).toBeInTheDocument();
  });

  it('applies error border class when error prop is present', () => {
    render(<Input error="oops" />);
    const input = screen.getByRole('textbox');
    expect(input.className).toMatch(/border-red/);
  });

  it('renders as number input when type="number"', () => {
    render(<Input type="number" />);
    const input = document.querySelector('input[type="number"]');
    expect(input).toBeInTheDocument();
  });

  it('is disabled when disabled prop is set', () => {
    render(<Input disabled />);
    expect(screen.getByRole('textbox')).toBeDisabled();
  });
});
