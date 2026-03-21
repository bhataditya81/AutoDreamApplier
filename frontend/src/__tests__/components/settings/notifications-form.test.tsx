import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NotificationsForm } from '@/components/settings/notifications-form';

jest.mock('@/lib/auth', () => ({ getToken: jest.fn(() => 'test-token') }));

const emptyPrefs = {
  targetTitles: [],
  locations: [],
  remotePref: 'any',
  salaryCurrency: 'USD',
  exclusions: [],
  slackWebhookUrl: '',
  discordWebhookUrl: '',
  webhookEvents: ['application_submitted'],
  emailDigestEnabled: true,
};

function mockFetchPrefs(prefs = emptyPrefs) {
  return jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: true,
    status: 200,
    json: async () => prefs,
  } as Response);
}

afterEach(() => jest.restoreAllMocks());

describe('NotificationsForm', () => {
  it('renders Slack webhook URL input', () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    expect(
      screen.getByPlaceholderText(/hooks\.slack\.com/i)
    ).toBeInTheDocument();
  });

  it('renders Discord webhook URL input', () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    expect(
      screen.getByPlaceholderText(/discord\.com\/api\/webhooks/i)
    ).toBeInTheDocument();
  });

  it('renders the weekly email digest toggle', () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    expect(screen.getByText(/weekly email digest/i)).toBeInTheDocument();
    expect(screen.getByRole('switch')).toBeInTheDocument();
  });

  it('does NOT show event checkboxes when no webhook URL entered', () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    expect(screen.queryByText(/notify on/i)).not.toBeInTheDocument();
  });

  it('shows event checkboxes after entering a Slack URL', async () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    const slackInput = screen.getByPlaceholderText(/hooks\.slack\.com/i);
    await userEvent.type(slackInput, 'https://hooks.slack.com/services/test');
    await waitFor(() =>
      expect(screen.getByText(/notify on/i)).toBeInTheDocument()
    );
    expect(screen.getByText('New match')).toBeInTheDocument();
    expect(screen.getByText('Application submitted')).toBeInTheDocument();
  });

  it('shows event checkboxes after entering a Discord URL', async () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    const discordInput = screen.getByPlaceholderText(/discord\.com\/api\/webhooks/i);
    await userEvent.type(discordInput, 'https://discord.com/api/webhooks/123/abc');
    await waitFor(() =>
      expect(screen.getByText(/notify on/i)).toBeInTheDocument()
    );
  });

  it('does NOT render test button when no webhook URL', () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    expect(screen.queryByRole('button', { name: /send test/i })).not.toBeInTheDocument();
  });

  it('shows test button when a webhook URL is entered', async () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    const slackInput = screen.getByPlaceholderText(/hooks\.slack\.com/i);
    await userEvent.type(slackInput, 'https://hooks.slack.com/services/test');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /send test/i })).toBeInTheDocument()
    );
  });

  it('shows success message after saving', async () => {
    jest.spyOn(global, 'fetch')
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => emptyPrefs } as Response)
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => emptyPrefs } as Response)
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => emptyPrefs } as Response);
    render(<NotificationsForm />);
    await waitFor(() => screen.getByRole('switch'));
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() =>
      expect(screen.getByText(/notification settings saved/i)).toBeInTheDocument()
    );
  });

  it('shows error for invalid Slack URL', async () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    const slackInput = screen.getByPlaceholderText(/hooks\.slack\.com/i);
    await userEvent.type(slackInput, 'not-a-url');
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() =>
      expect(screen.getByText(/valid https/i)).toBeInTheDocument()
    );
  });

  it('shows error for invalid Discord URL (http not https)', async () => {
    mockFetchPrefs();
    render(<NotificationsForm />);
    const discordInput = screen.getByPlaceholderText(/discord\.com\/api\/webhooks/i);
    await userEvent.type(discordInput, 'http://insecure.com');
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() =>
      expect(screen.getByText(/valid https/i)).toBeInTheDocument()
    );
  });
});
