import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PreferencesForm } from '@/components/settings/preferences-form';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

const mockPrefs = {
  targetTitles: ['Software Engineer'],
  locations: ['Remote'],
  remotePref: 'remote',
  salaryCurrency: 'USD',
  exclusions: [],
  ai_tailor_enabled: true,
  autoApplyEnabled: false,
  dailyApplicationLimit: 10,
  applyStartHour: 9,
  applyEndHour: 17,
  applyTimezone: 'America/New_York',
};

function mockGetPrefs(prefs = mockPrefs) {
  return jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => prefs,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

describe('PreferencesForm', () => {
  it('loads preferences on mount and shows existing titles', async () => {
    mockGetPrefs();
    render(<PreferencesForm />);
    await waitFor(() =>
      expect(screen.getByText('Software Engineer')).toBeInTheDocument()
    );
  });

  it('shows Auto-Apply toggle section', async () => {
    mockGetPrefs();
    render(<PreferencesForm />);
    await waitFor(() => expect(screen.getByText('Auto-Apply')).toBeInTheDocument());
  });

  it('shows confirmation modal when enabling auto-apply', async () => {
    mockGetPrefs();
    render(<PreferencesForm />);
    await waitFor(() => screen.getByText('Auto-Apply'));

    // Auto-apply switch is the last switch rendered
    const switches = screen.getAllByRole('switch');
    const autoApplySwitch = switches[switches.length - 1];
    await userEvent.click(autoApplySwitch);

    await waitFor(() =>
      expect(screen.getByText('Enable Auto-Apply?')).toBeInTheDocument()
    );
  });

  it('enables auto-apply after confirming in modal', async () => {
    mockGetPrefs();
    render(<PreferencesForm />);
    await waitFor(() => screen.getByText('Auto-Apply'));

    const switches = screen.getAllByRole('switch');
    const autoApplySwitch = switches[switches.length - 1];
    await userEvent.click(autoApplySwitch);

    await waitFor(() => screen.getByText('Enable Auto-Apply?'));
    await userEvent.click(screen.getByRole('button', { name: /enable auto-apply/i }));

    await waitFor(() =>
      expect(screen.queryByText('Enable Auto-Apply?')).not.toBeInTheDocument()
    );
    expect(autoApplySwitch).toHaveAttribute('aria-checked', 'true');
  });

  it('cancels auto-apply when cancel is clicked in modal', async () => {
    mockGetPrefs();
    render(<PreferencesForm />);
    await waitFor(() => screen.getByText('Auto-Apply'));

    const switches = screen.getAllByRole('switch');
    const autoApplySwitch = switches[switches.length - 1];
    await userEvent.click(autoApplySwitch);

    await waitFor(() => screen.getByText('Enable Auto-Apply?'));
    await userEvent.click(screen.getByRole('button', { name: /cancel/i }));

    await waitFor(() =>
      expect(screen.queryByText('Enable Auto-Apply?')).not.toBeInTheDocument()
    );
    expect(autoApplySwitch).toHaveAttribute('aria-checked', 'false');
  });

  it('shows success message after saving', async () => {
    // First call: getPreferences; subsequent: savePreferences
    jest.spyOn(global, 'fetch')
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => mockPrefs } as Response)
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => mockPrefs } as Response);

    render(<PreferencesForm />);
    await waitFor(() => screen.getByText('Software Engineer'));

    await userEvent.click(screen.getByRole('button', { name: /save preferences/i }));

    await waitFor(() =>
      expect(screen.getByText(/preferences saved/i)).toBeInTheDocument()
    );
  });

  it('shows error if no target title is added', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ ...mockPrefs, targetTitles: [] }),
    } as Response);

    render(<PreferencesForm />);
    // Wait for component to mount
    await waitFor(() => {
      expect(screen.queryByText('Software Engineer')).not.toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole('button', { name: /save preferences/i }));

    await waitFor(() =>
      expect(screen.getByText(/at least one target job title/i)).toBeInTheDocument()
    );
  });

  it('shows error alert when save fails', async () => {
    jest.spyOn(global, 'fetch')
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => mockPrefs } as Response)
      .mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({ message: 'Server error' }) } as Response);

    render(<PreferencesForm />);
    await waitFor(() => screen.getByText('Software Engineer'));

    await userEvent.click(screen.getByRole('button', { name: /save preferences/i }));

    await waitFor(() =>
      expect(screen.getByRole('alert')).toBeInTheDocument()
    );
  });
});
