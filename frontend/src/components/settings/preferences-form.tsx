"use client";

import { useState, useEffect, useRef } from "react";
import { Plus, X, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { getPreferences, savePreferences } from "@/lib/api";
import type { RemotePref } from "@/lib/types";

// Popular timezones ordered by usage — avoids a 500-item select.
const TIMEZONES = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Vancouver",
  "America/Toronto",
  "America/Sao_Paulo",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Europe/Amsterdam",
  "Europe/Zurich",
  "Europe/Istanbul",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Asia/Seoul",
  "Asia/Shanghai",
  "Australia/Sydney",
  "Pacific/Auckland",
];

const HOURS = Array.from({ length: 24 }, (_, i) => {
  const h = i % 12 || 12;
  const ampm = i < 12 ? "AM" : "PM";
  return { value: i, label: `${h}:00 ${ampm}` };
});

// ── Tag components (module-scope to avoid remounting on every keystroke) ─────

function TagInput({
  value,
  onChange,
  onAdd,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  onAdd: () => void;
  placeholder: string;
}) {
  return (
    <div className="flex gap-2">
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        onKeyDown={(e) => {
          if (e.key === "Enter") { e.preventDefault(); onAdd(); }
        }}
      />
      <Button variant="outline" size="sm" onClick={onAdd}>
        <Plus className="h-4 w-4" />
      </Button>
    </div>
  );
}

function TagList({
  items,
  onRemove,
  colorClass = "bg-brand-50 text-brand-700 border-brand-100",
}: {
  items: string[];
  onRemove: (v: string) => void;
  colorClass?: string;
}) {
  if (items.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((t) => (
        <span
          key={t}
          className={`inline-flex items-center gap-1 text-xs font-medium px-2.5 py-1 rounded-full border ${colorClass}`}
        >
          {t}
          <button onClick={() => onRemove(t)} className="hover:opacity-70 transition-opacity">
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
    </div>
  );
}

export function PreferencesForm() {
  // ── Job preference state ───────────────────────────────────────────────────
  const [titles, setTitles] = useState<string[]>([]);
  const [titleInput, setTitleInput] = useState("");
  const [locations, setLocations] = useState<string[]>([]);
  const [locationInput, setLocationInput] = useState("");
  const [exclusions, setExclusions] = useState<string[]>([]);
  const [exclusionInput, setExclusionInput] = useState("");
  const [remotePref, setRemotePref] = useState<RemotePref>("any");
  const [salaryMin, setSalaryMin] = useState("");
  const [salaryMax, setSalaryMax] = useState("");
  const [aiTailorEnabled, setAiTailorEnabled] = useState(true);

  // ── Auto-apply state ───────────────────────────────────────────────────────
  const [autoApplyEnabled, setAutoApplyEnabled] = useState(false);
  const [dailyLimit, setDailyLimit] = useState(10);
  const [startHour, setStartHour] = useState(9);
  const [endHour, setEndHour] = useState(17);
  const [timezone, setTimezone] = useState("America/New_York");
  const [showConfirm, setShowConfirm] = useState(false);
  // Track pending toggle value while confirmation is shown
  const pendingAutoApply = useRef(false);

  // ── Form feedback ──────────────────────────────────────────────────────────
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getPreferences()
      .then((p) => {
        setTitles(p.targetTitles ?? []);
        setLocations(p.locations ?? []);
        setExclusions(p.exclusions ?? []);
        setRemotePref(p.remotePref ?? "any");
        setSalaryMin(p.salaryMin ? String(p.salaryMin) : "");
        setSalaryMax(p.salaryMax ? String(p.salaryMax) : "");
        setAiTailorEnabled(p.ai_tailor_enabled !== undefined ? p.ai_tailor_enabled : true);
        setAutoApplyEnabled(p.autoApplyEnabled ?? false);
        setDailyLimit(p.dailyApplicationLimit ?? 10);
        setStartHour(p.applyStartHour ?? 9);
        setEndHour(p.applyEndHour ?? 17);
        setTimezone(p.applyTimezone ?? "America/New_York");
      })
      .catch(() => {/* no prefs yet — use defaults */});
  }, []);

  // ── Helpers ────────────────────────────────────────────────────────────────
  function addTag(
    value: string,
    list: string[],
    setList: (v: string[]) => void,
    setInput: (v: string) => void,
  ) {
    const trimmed = value.trim();
    if (trimmed && !list.includes(trimmed)) setList([...list, trimmed]);
    setInput("");
  }

  function removeTag(item: string, list: string[], setList: (v: string[]) => void) {
    setList(list.filter((t) => t !== item));
  }

  // ── Auto-apply toggle (requires confirmation to enable) ────────────────────
  function handleAutoApplyToggle(next: boolean) {
    if (next) {
      // Enabling: show confirmation modal first
      pendingAutoApply.current = true;
      setShowConfirm(true);
    } else {
      setAutoApplyEnabled(false);
    }
  }

  function confirmAutoApply() {
    setAutoApplyEnabled(pendingAutoApply.current);
    setShowConfirm(false);
  }

  function cancelAutoApply() {
    pendingAutoApply.current = false;
    setShowConfirm(false);
  }

  // ── Save ───────────────────────────────────────────────────────────────────
  async function handleSave() {
    if (titles.length === 0) { setError("Add at least one target job title."); return; }
    const limitNum = Number(dailyLimit);
    if (limitNum < 1 || limitNum > 50) { setError("Daily limit must be between 1 and 50."); return; }
    if (startHour >= endHour) { setError("Apply start hour must be before end hour."); return; }

    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await savePreferences({
        targetTitles: titles,
        locations,
        remotePref,
        salaryMin: salaryMin ? parseInt(salaryMin) : undefined,
        salaryMax: salaryMax ? parseInt(salaryMax) : undefined,
        salaryCurrency: "USD",
        exclusions,
        ai_tailor_enabled: aiTailorEnabled,
        autoApplyEnabled,
        dailyApplicationLimit: limitNum,
        applyStartHour: startHour,
        applyEndHour: endHour,
        applyTimezone: timezone,
      });
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    } catch (e: unknown) {
      console.error("[PreferencesForm] handleSave error:", e);
      setError(e instanceof Error ? e.message : "Failed to save preferences.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      {/* ── Confirmation modal ─────────────────────────────────────────────── */}
      {showConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
          <div className="bg-white rounded-xl shadow-xl max-w-sm w-full mx-4 p-6 space-y-4">
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-50">
                <AlertTriangle className="h-5 w-5 text-amber-500" />
              </div>
              <div>
                <h3 className="font-semibold text-gray-900 text-sm">Enable Auto-Apply?</h3>
                <p className="mt-1 text-sm text-gray-500">
                  AutoDreamApplier will submit applications on your behalf — up to your daily
                  limit — without individual review. You can disable this at any time.
                </p>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={cancelAutoApply}>
                Cancel
              </Button>
              <Button size="sm" onClick={confirmAutoApply}>
                Enable Auto-Apply
              </Button>
            </div>
          </div>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Job Preferences</CardTitle>
          <CardDescription>
            Control which jobs are discovered and matched to your profile.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {error && <Alert variant="error">{error}</Alert>}
          {success && <Alert variant="success">Preferences saved.</Alert>}

          {/* Target titles */}
          <div className="space-y-2">
            <Label required>Target job titles</Label>
            <TagInput
              value={titleInput}
              onChange={setTitleInput}
              onAdd={() => addTag(titleInput, titles, setTitles, setTitleInput)}
              placeholder="e.g. Software Engineer"
            />
            <TagList items={titles} onRemove={(t) => removeTag(t, titles, setTitles)} />
          </div>

          {/* Locations */}
          <div className="space-y-2">
            <Label>Preferred locations</Label>
            <TagInput
              value={locationInput}
              onChange={setLocationInput}
              onAdd={() => addTag(locationInput, locations, setLocations, setLocationInput)}
              placeholder="e.g. San Francisco, CA"
            />
            <TagList
              items={locations}
              onRemove={(l) => removeTag(l, locations, setLocations)}
              colorClass="bg-gray-100 text-gray-700 border-gray-200"
            />
          </div>

          {/* Remote preference */}
          <div className="space-y-2">
            <Label>Remote preference</Label>
            <Select
              value={remotePref}
              onChange={(e) => setRemotePref(e.target.value as RemotePref)}
            >
              <option value="remote">Remote only</option>
              <option value="hybrid">Hybrid</option>
              <option value="onsite">On-site only</option>
              <option value="any">Any (no preference)</option>
            </Select>
          </div>

          {/* Salary */}
          <div className="space-y-2">
            <Label>Salary range (USD / year)</Label>
            <div className="grid grid-cols-2 gap-3">
              <Input
                type="number"
                placeholder="Min (e.g. 80000)"
                value={salaryMin}
                onChange={(e) => setSalaryMin(e.target.value)}
              />
              <Input
                type="number"
                placeholder="Max (e.g. 150000)"
                value={salaryMax}
                onChange={(e) => setSalaryMax(e.target.value)}
              />
            </div>
          </div>

          {/* Exclusions */}
          <div className="space-y-2">
            <Label>Excluded companies or keywords</Label>
            <TagInput
              value={exclusionInput}
              onChange={setExclusionInput}
              onAdd={() => addTag(exclusionInput, exclusions, setExclusions, setExclusionInput)}
              placeholder="e.g. Corp, Staffing, Inc."
            />
            <TagList
              items={exclusions}
              onRemove={(e) => removeTag(e, exclusions, setExclusions)}
              colorClass="bg-red-50 text-red-700 border-red-100"
            />
            <p className="text-xs text-gray-400">
              Jobs matching these terms will be automatically skipped.
            </p>
          </div>

          {/* AI tailoring toggle */}
          <div className="flex flex-row items-center justify-between rounded-lg border p-4">
            <div className="space-y-0.5">
              <Label className="text-base">AI Resume Tailoring</Label>
              <p className="text-sm text-gray-500">
                When disabled, your resume is sent as-is without AI customisation.
              </p>
            </div>
            <Switch checked={aiTailorEnabled} onCheckedChange={setAiTailorEnabled} />
          </div>

          {/* ── Auto-Apply section ─────────────────────────────────────────── */}
          <div className="rounded-lg border border-gray-200 overflow-hidden">
            {/* Section header / master toggle */}
            <div className="flex items-center justify-between px-4 py-3 bg-gray-50 border-b border-gray-200">
              <div>
                <p className="text-sm font-medium text-gray-900">Auto-Apply</p>
                <p className="text-xs text-gray-500 mt-0.5">
                  Automatically submit matched jobs without manual review.
                </p>
              </div>
              <Switch
                checked={autoApplyEnabled}
                onCheckedChange={handleAutoApplyToggle}
              />
            </div>

            {/* Collapsed schedule settings — only shown when enabled */}
            <div
              className={`transition-all duration-200 ${
                autoApplyEnabled ? "opacity-100" : "opacity-40 pointer-events-none"
              }`}
            >
              <div className="px-4 py-4 space-y-4">
                {/* Daily limit */}
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <Label className="text-sm">Daily application limit</Label>
                    <p className="text-xs text-gray-400 mt-0.5">1–50 applications per day</p>
                  </div>
                  <Input
                    type="number"
                    min={1}
                    max={50}
                    value={dailyLimit}
                    onChange={(e) => setDailyLimit(Number(e.target.value))}
                    className="w-20 text-center"
                  />
                </div>

                {/* Apply hours */}
                <div className="space-y-2">
                  <Label className="text-sm">Apply window</Label>
                  <div className="flex items-center gap-2">
                    <Select
                      value={String(startHour)}
                      onChange={(e) => setStartHour(Number(e.target.value))}
                      className="flex-1"
                    >
                      {HOURS.map((h) => (
                        <option key={h.value} value={h.value}>{h.label}</option>
                      ))}
                    </Select>
                    <span className="text-sm text-gray-400 shrink-0">to</span>
                    <Select
                      value={String(endHour)}
                      onChange={(e) => setEndHour(Number(e.target.value))}
                      className="flex-1"
                    >
                      {HOURS.map((h) => (
                        <option key={h.value} value={h.value}>{h.label}</option>
                      ))}
                    </Select>
                  </div>
                  <p className="text-xs text-gray-400">
                    Applications are only submitted on weekdays within this window.
                  </p>
                </div>

                {/* Timezone */}
                <div className="space-y-2">
                  <Label className="text-sm">Timezone</Label>
                  <Select
                    value={timezone}
                    onChange={(e) => setTimezone(e.target.value)}
                  >
                    {TIMEZONES.map((tz) => (
                      <option key={tz} value={tz}>{tz}</option>
                    ))}
                  </Select>
                </div>
              </div>
            </div>
          </div>

          <Button onClick={handleSave} loading={saving}>
            Save preferences
          </Button>
        </CardContent>
      </Card>
    </>
  );
}
