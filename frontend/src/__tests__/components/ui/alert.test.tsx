import React from 'react';
import { render, screen } from '@testing-library/react';
import { Alert } from '@/components/ui/alert';

describe('Alert', () => {
  it('renders children', () => {
    render(<Alert>Something happened</Alert>);
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Something happened')).toBeInTheDocument();
  });

  it('renders title when provided', () => {
    render(<Alert title="Heads up">Details here</Alert>);
    expect(screen.getByText('Heads up')).toBeInTheDocument();
    expect(screen.getByText('Details here')).toBeInTheDocument();
  });

  it('defaults to info variant (sky styling)', () => {
    render(<Alert>Info message</Alert>);
    const alert = screen.getByRole('alert');
    expect(alert.className).toMatch(/bg-sky-50/);
  });

  it('renders success variant', () => {
    render(<Alert variant="success">Done!</Alert>);
    const alert = screen.getByRole('alert');
    expect(alert.className).toMatch(/bg-green-50/);
  });

  it('renders warning variant', () => {
    render(<Alert variant="warning">Watch out</Alert>);
    const alert = screen.getByRole('alert');
    expect(alert.className).toMatch(/bg-yellow-50/);
  });

  it('renders error variant', () => {
    render(<Alert variant="error">Failed</Alert>);
    const alert = screen.getByRole('alert');
    expect(alert.className).toMatch(/bg-red-50/);
  });
});
