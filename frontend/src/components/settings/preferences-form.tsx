"use client";

import { useState, useEffect } from "react";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { getPreferences, savePreferences } from "@/lib/api";
import type { RemotePref } from "@/lib/types";

export function PreferencesForm() {
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
        
        // This relies on the backend returning default true, but safely handle undefined.
        setAiTailorEnabled(p.ai_tailor_enabled !== undefined ? p.ai_tailor_enabled : true);
      })
      .catch(() => {/* no prefs yet, use empty defaults */});
  }, []);

  function addTag(
    value: string,
    list: string[],
    setList: (v: string[]) => void,
    setInput: (v: string) => void
  ) {
    const trimmed = value.trim();
    if (trimmed && !list.includes(trimmed)) setList([...list, trimmed]);
    setInput("");
  }

  function removeTag(item: string, list: string[], setList: (v: string[]) => void) {
    setList(list.filter((t) => t !== item));
  }

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

  async function handleSave() {
    if (titles.length === 0) { setError("Add at least one target job title."); return; }
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

        <div className="flex flex-row items-center justify-between rounded-lg border p-4">
          <div className="space-y-0.5">
            <Label className="text-base">AI Resume Tailoring</Label>
            <p className="text-sm text-gray-500">
              When disabled, the system will apply using your exact resume as uploaded without using AI to tailor it to the job description.
            </p>
          </div>
          <Switch
            checked={aiTailorEnabled}
            onCheckedChange={setAiTailorEnabled}
          />
        </div>

        <Button onClick={handleSave} loading={saving}>
          Save preferences
        </Button>
      </CardContent>
    </Card>
  );
}
