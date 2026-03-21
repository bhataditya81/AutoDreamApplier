import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProfileForm } from '@/components/settings/profile-form';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

const mockUser = {
  id: 'user-1',
  email: 'jane@example.com',
  fullName: 'Jane Smith',
  tier: 'free',
  applyMode: 'review',
  autoThreshold: 0.8,
  dailyLimit: 5,
  isActive: true,
  createdAt: '2024-01-01T00:00:00Z',
};

afterEach(() => jest.restoreAllMocks());

describe('ProfileForm', () => {
  it('renders the Profile & Apply Settings heading', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockUser,
    } as Response);
    render(<ProfileForm />);
    await waitFor(() =>
      expect(screen.getByText('Profile & Apply Settings')).toBeInTheDocument()
    );
  });

  it('loads user data on mount', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockUser,
    } as Response);
    render(<ProfileForm />);
    await waitFor(() => {
      expect(screen.getByDisplayValue('Jane Smith')).toBeInTheDocument();
    });
  });

  it('renders email as disabled input', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockUser,
    } as Response);
    render(<ProfileForm />);
    await waitFor(() => screen.getByDisplayValue('jane@example.com'));
    const emailInput = screen.getByDisplayValue('jane@example.com');
    expect(emailInput).toBeDisabled();
  });

  it('renders daily application limit input', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockUser,
    } as Response);
    render(<ProfileForm />);
    await waitFor(() => screen.getByText('Profile & Apply Settings'));
    expect(screen.getByText(/daily application limit/i)).toBeInTheDocument();
  });

  it('saves profile changes and shows success message', async () => {
    jest.spyOn(global, 'fetch')
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => mockUser } as Response)
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ ...mockUser, fullName: 'Jane Doe' }) } as Response);

    render(<ProfileForm />);
    await waitFor(() => screen.getByDisplayValue('Jane Smith'));

    const nameInput = screen.getByDisplayValue('Jane Smith');
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, 'Jane Doe');

    await userEvent.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() =>
      expect(screen.getByText(/profile saved successfully/i)).toBeInTheDocument()
    );
  });

  it('shows error when user load fails', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: async () => ({ error: 'Unauthorized' }),
    } as Response);
    render(<ProfileForm />);
    await waitFor(() =>
      expect(screen.getByText(/failed to load profile/i)).toBeInTheDocument()
    );
  });

  it('shows error when save fails', async () => {
    jest.spyOn(global, 'fetch')
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => mockUser } as Response)
      .mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({ message: 'Server error' }) } as Response);

    render(<ProfileForm />);
    await waitFor(() => screen.getByDisplayValue('Jane Smith'));

    await userEvent.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() =>
      expect(screen.getByRole('alert')).toBeInTheDocument()
    );
  });

  it('shows auto-threshold slider when applyMode is "auto"', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ ...mockUser, applyMode: 'auto' }),
    } as Response);
    render(<ProfileForm />);
    await waitFor(() => screen.getByText(/auto-apply threshold/i));
    expect(screen.getByRole('slider')).toBeInTheDocument();
  });
});
