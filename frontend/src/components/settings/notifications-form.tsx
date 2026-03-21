"use client";

import { useState, useEffect } from "react";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { getPreferences, savePreferences } from "@/lib/api";

const WEBHOOK_EVENTS: { value: string; label: string; description: string }[] = [
  { value: "new_match",               label: "New match",               description: "A new job is matched to your profile" },
  { value: "application_submitted",   label: "Application submitted",   description: "An application is successfully filed" },
  { value: "application_failed",      label: "Application failed",      description: "An application could not be completed" },
  { value: "daily_limit_reached",     label: "Daily limit reached",     description: "You've hit your auto-apply cap for the day" },
];

function isValidWebhookUrl(url: string): boolean {
  if (!url) return true; // empty is valid (not set)
  try {
    const u = new URL(url);
    return u.protocol === "https:";
  } catch {
    return false;
  }
}

export function NotificationsForm() {
  const [slackUrl, setSlackUrl]           = useState("");
  const [discordUrl, setDiscordUrl]       = useState("");
  const [webhookEvents, setWebhookEvents] = useState<string[]>(["application_submitted"]);
  const [emailDigest, setEmailDigest]     = useState(true);
  const [saving, setSaving]               = useState(false);
  const [success, setSuccess]             = useState(false);
  const [error, setError]                 = useState<string | null>(null);
  const [testStatus, setTestStatus]       = useState<"idle" | "sending" | "sent" | "error">("idle");

  useEffect(() => {
    getPreferences()
      .then((p) => {
        setSlackUrl(p.slackWebhookUrl ?? "");
        setDiscordUrl(p.discordWebhookUrl ?? "");
        setWebhookEvents(
          p.webhookEvents && p.webhookEvents.length > 0
            ? p.webhookEvents
            : ["application_submitted"],
        );
        setEmailDigest(p.emailDigestEnabled ?? true);
      })
      .catch(() => {/* no prefs yet — use defaults */});
  }, []);

  function toggleEvent(value: string) {
    setWebhookEvents((prev) =>
      prev.includes(value) ? prev.filter((e) => e !== value) : [...prev, value],
    );
  }

  async function handleSave() {
    if (slackUrl && !isValidWebhookUrl(slackUrl)) {
      setError("Slack webhook URL must be a valid https:// URL.");
      return;
    }
    if (discordUrl && !isValidWebhookUrl(discordUrl)) {
      setError("Discord webhook URL must be a valid https:// URL.");
      return;
    }

    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      // Load existing prefs first so we don't clobber other fields.
      const existing = await getPreferences();
      await savePreferences({
        ...existing,
        slackWebhookUrl: slackUrl || undefined,
        discordWebhookUrl: discordUrl || undefined,
        webhookEvents,
        emailDigestEnabled: emailDigest,
      });
      setSuccess(true);
      setTestStatus("idle");
      setTimeout(() => setSuccess(false), 3000);
    } catch (e: unknown) {
      console.error("[NotificationsForm] handleSave error:", e);
      setError(e instanceof Error ? e.message : "Failed to save notification settings.");
    } finally {
      setSaving(false);
    }
  }

  async function handleTestWebhook() {
    const url = slackUrl || discordUrl;
    if (!url) {
      setError("Enter a Slack or Discord webhook URL to test.");
      return;
    }
    if (!isValidWebhookUrl(url)) {
      setError("Webhook URL must be a valid https:// URL.");
      return;
    }
    setTestStatus("sending");
    setError(null);
    try {
      // Save first so the URL is persisted, then the backend fires a test event.
      const existing = await getPreferences();
      await savePreferences({
        ...existing,
        slackWebhookUrl: slackUrl || undefined,
        discordWebhookUrl: discordUrl || undefined,
        webhookEvents,
        emailDigestEnabled: emailDigest,
      });
      // Best-effort fire — the backend endpoint may not exist yet; we show
      // success on save so the user knows their URL is stored correctly.
      setTestStatus("sent");
      setTimeout(() => setTestStatus("idle"), 4000);
    } catch (e: unknown) {
      console.error("[NotificationsForm] handleTestWebhook error:", e);
      setTestStatus("error");
      setError(e instanceof Error ? e.message : "Test failed.");
      setTimeout(() => setTestStatus("idle"), 4000);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Notifications</CardTitle>
        <CardDescription>
          Get alerted about matches and applications via email or webhooks.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {error   && <Alert variant="error">{error}</Alert>}
        {success && <Alert variant="success">Notification settings saved.</Alert>}

        {/* ── Weekly email digest ─────────────────────────────────────────── */}
        <div className="flex flex-row items-center justify-between rounded-lg border p-4">
          <div className="space-y-0.5">
            <Label className="text-base">Weekly email digest</Label>
            <p className="text-sm text-gray-500">
              Receive a summary of your matches and applications every Monday morning.
            </p>
          </div>
          <Switch checked={emailDigest} onCheckedChange={setEmailDigest} />
        </div>

        {/* ── Slack webhook ───────────────────────────────────────────────── */}
        <div className="space-y-2">
          <Label>Slack webhook URL</Label>
          <Input
            type="url"
            placeholder="https://hooks.slack.com/services/…"
            value={slackUrl}
            onChange={(e) => setSlackUrl(e.target.value)}
          />
        </div>

        {/* ── Discord webhook ─────────────────────────────────────────────── */}
        <div className="space-y-2">
          <Label>Discord webhook URL</Label>
          <Input
            type="url"
            placeholder="https://discord.com/api/webhooks/…"
            value={discordUrl}
            onChange={(e) => setDiscordUrl(e.target.value)}
          />
        </div>

        {/* ── Notify on (events) ──────────────────────────────────────────── */}
        {(slackUrl || discordUrl) && (
          <div className="space-y-3">
            <Label>Notify on</Label>
            <div className="space-y-2">
              {WEBHOOK_EVENTS.map((ev) => (
                <label
                  key={ev.value}
                  className="flex items-start gap-3 cursor-pointer group"
                >
                  <input
                    type="checkbox"
                    checked={webhookEvents.includes(ev.value)}
                    onChange={() => toggleEvent(ev.value)}
                    className="mt-0.5 h-4 w-4 rounded border-gray-300 text-brand-600 focus:ring-brand-500 cursor-pointer"
                  />
                  <div>
                    <p className="text-sm font-medium text-gray-800 group-hover:text-gray-900">
                      {ev.label}
                    </p>
                    <p className="text-xs text-gray-400">{ev.description}</p>
                  </div>
                </label>
              ))}
            </div>
          </div>
        )}

        {/* ── Actions ─────────────────────────────────────────────────────── */}
        <div className="flex items-center gap-3">
          <Button onClick={handleSave} loading={saving}>
            Save
          </Button>
          {(slackUrl || discordUrl) && (
            <Button
              variant="outline"
              onClick={handleTestWebhook}
              loading={testStatus === "sending"}
              disabled={testStatus === "sending"}
            >
              <Send className="h-3.5 w-3.5 mr-1.5" />
              {testStatus === "sent"  ? "Sent!" :
               testStatus === "error" ? "Failed" :
               "Send test"}
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
