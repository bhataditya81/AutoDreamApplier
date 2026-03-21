import React from 'react';
import { render, screen } from '@testing-library/react';
import { Badge } from '@/components/ui/badge';

describe('Badge', () => {
  it('renders children', () => {
    render(<Badge>Active</Badge>);
    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('renders default variant', () => {
    render(<Badge>Default</Badge>);
    const badge = screen.getByText('Default');
    expect(badge.className).toMatch(/bg-gray-100/);
  });

  it('renders success variant', () => {
    render(<Badge variant="success">Remote</Badge>);
    const badge = screen.getByText('Remote');
    expect(badge.className).toMatch(/bg-green-100/);
  });

  it('renders danger variant', () => {
    render(<Badge variant="danger">Scam</Badge>);
    const badge = screen.getByText('Scam');
    expect(badge.className).toMatch(/bg-red-100/);
  });

  it('renders brand variant', () => {
    render(<Badge variant="brand">ATS</Badge>);
    const badge = screen.getByText('ATS');
    expect(badge.className).toMatch(/bg-brand-100/);
  });

  it('renders applied status variant', () => {
    render(<Badge variant="applied">Applied</Badge>);
    const badge = screen.getByText('Applied');
    expect(badge.className).toMatch(/bg-green-100/);
  });

  it('renders failed status variant', () => {
    render(<Badge variant="failed">Failed</Badge>);
    const badge = screen.getByText('Failed');
    expect(badge.className).toMatch(/bg-red-100/);
  });

  it('renders withdrawn status variant', () => {
    render(<Badge variant="withdrawn">Withdrawn</Badge>);
    const badge = screen.getByText('Withdrawn');
    expect(badge.className).toMatch(/bg-gray-100/);
  });

  it('renders interview outcome variant', () => {
    render(<Badge variant="interview">Interview</Badge>);
    const badge = screen.getByText('Interview');
    expect(badge.className).toMatch(/bg-emerald-100/);
  });

  it('accepts custom className', () => {
    render(<Badge className="custom-class">Tag</Badge>);
    expect(screen.getByText('Tag').className).toMatch(/custom-class/);
  });
});
