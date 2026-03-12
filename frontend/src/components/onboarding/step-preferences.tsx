"use client";

import { useState } from "react";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Alert } from "@/components/ui/alert";
import { savePreferences } from "@/lib/api";
import type { UserPreferences, RemotePref } from "@/lib/types";

interface StepPreferencesProps {
  onComplete: (prefs: UserPreferences) => void;
}

export function StepPreferences({ onComplete }: StepPreferencesProps) {
  const [titles, setTitles] = useState<string[]>([]);
  const [titleInput, setTitleInput] = useState("");
  const [locations, setLocations] = useState<string[]>([]);
  const [locationInput, setLocationInput] = useState("");
  const [remotePref, setRemotePref] = useState<RemotePref>("any");
  const [salaryMin, setSalaryMin] = useState("");
  const [salaryMax, setSalaryMax] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function addTag(
    value: string,
    list: string[],
    setList: (v: string[]) => void,
    setInput: (v: string) => void
  ) {
    const trimmed = value.trim();
    if (trimmed && !list.includes(trimmed)) {
      setList([...list, trimmed]);
    }
    setInput("");
  }

  function removeTag(item: string, list: string[], setList: (v: string[]) => void) {
    setList(list.filter((t) => t !== item));
  }

  async function handleSave() {
    if (titles.length === 0) {
      setError("Add at least one target job title.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const prefs = await savePreferences({
        targetTitles: titles,
        locations,
        remotePref,
        salaryMin: salaryMin ? parseInt(salaryMin) : undefined,
        salaryMax: salaryMax ? parseInt(salaryMax) : undefined,
        salaryCurrency: "USD",
        exclusions: [],
      });
      onComplete(prefs);
    } catch (e: unknown) {
      console.error("[StepPreferences] handleSave error:", e);
      setError(e instanceof Error ? e.message : "Failed to save preferences.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-lg font-semibold text-gray-900">Job preferences</h2>
        <p className="text-sm text-gray-500 mt-1">
          Tell us what you&apos;re looking for and we&apos;ll find and filter jobs automatically.
        </p>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {/* Target titles */}
      <div className="space-y-2">
        <Label required>Target job titles</Label>
        <div className="flex gap-2">
          <Input
            value={titleInput}
            onChange={(e) => setTitleInput(e.target.value)}
            placeholder="e.g. Software Engineer"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addTag(titleInput, titles, setTitles, setTitleInput);
              }
            }}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() => addTag(titleInput, titles, setTitles, setTitleInput)}
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {titles.map((t) => (
            <span
              key={t}
              className="inline-flex items-center gap-1 bg-brand-50 text-brand-700 text-xs font-medium px-2.5 py-1 rounded-full border border-brand-100"
            >
              {t}
              <button
                onClick={() => removeTag(t, titles, setTitles)}
                className="hover:text-brand-900 transition-colors"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      </div>

      {/* Locations */}
      <div className="space-y-2">
        <Label>Preferred locations</Label>
        <div className="flex gap-2">
          <Input
            value={locationInput}
            onChange={(e) => setLocationInput(e.target.value)}
            placeholder="e.g. San Francisco, CA"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addTag(locationInput, locations, setLocations, setLocationInput);
              }
            }}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() =>
              addTag(locationInput, locations, setLocations, setLocationInput)
            }
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {locations.map((l) => (
            <span
              key={l}
              className="inline-flex items-center gap-1 bg-gray-100 text-gray-700 text-xs font-medium px-2.5 py-1 rounded-full"
            >
              {l}
              <button
                onClick={() => removeTag(l, locations, setLocations)}
                className="hover:text-gray-900 transition-colors"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
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

      {/* Salary range */}
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

      <Button className="w-full" onClick={handleSave} loading={saving}>
        Save & Continue
      </Button>
    </div>
  );
}
