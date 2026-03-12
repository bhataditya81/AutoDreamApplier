"use client";

import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { getMe, updateMe } from "@/lib/api";
import type { User, ApplyMode } from "@/lib/types";

export function ProfileForm() {
  const [user, setUser] = useState<User | null>(null);
  const [fullName, setFullName] = useState("");
  const [applyMode, setApplyMode] = useState<ApplyMode>("review");
  const [autoThreshold, setAutoThreshold] = useState("0.80");
  const [dailyLimit, setDailyLimit] = useState("5");
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getMe()
      .then((u) => {
        setUser(u);
        setFullName(u.fullName ?? "");
        setApplyMode(u.applyMode);
        setAutoThreshold(String(u.autoThreshold));
        setDailyLimit(String(u.dailyLimit));
      })
      .catch((e: unknown) => {
        console.error("[ProfileForm] getMe error:", e);
        setError("Failed to load profile.");
      });
  }, []);

  async function handleSave() {
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      const updated = await updateMe({
        fullName,
        applyMode,
        autoThreshold: parseFloat(autoThreshold),
        dailyLimit: parseInt(dailyLimit),
      });
      setUser(updated);
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    } catch (e: unknown) {
      console.error("[ProfileForm] handleSave error:", e);
      setError(e instanceof Error ? e.message : "Failed to save profile.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile & Apply Settings</CardTitle>
        <CardDescription>
          Configure how AutoDreamApplier discovers and applies to jobs on your behalf.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {error && <Alert variant="error">{error}</Alert>}
        {success && <Alert variant="success">Profile saved successfully.</Alert>}

        <div className="space-y-2">
          <Label>Full name</Label>
          <Input
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            placeholder="Jane Smith"
          />
        </div>

        <div className="space-y-2">
          <Label>Email</Label>
          <Input value={user?.email ?? ""} disabled />
          <p className="text-xs text-gray-400">Email is managed by your account provider.</p>
        </div>

        <div className="space-y-2">
          <Label>Apply mode</Label>
          <Select
            value={applyMode}
            onChange={(e) => setApplyMode(e.target.value as ApplyMode)}
          >
            <option value="review">Review — you approve each match before applying</option>
            <option value="auto">Auto — apply automatically above confidence threshold</option>
          </Select>
        </div>

        {applyMode === "auto" && (
          <div className="space-y-2">
            <Label>Auto-apply threshold</Label>
            <div className="flex items-center gap-3">
              <input
                type="range"
                min="0.5"
                max="1.0"
                step="0.05"
                value={autoThreshold}
                onChange={(e) => setAutoThreshold(e.target.value)}
                className="flex-1 accent-brand-600"
              />
              <span className="text-sm font-semibold text-brand-700 w-12 text-right">
                {Math.round(parseFloat(autoThreshold) * 100)}%
              </span>
            </div>
            <p className="text-xs text-gray-400">
              Only apply automatically when match score ≥ {Math.round(parseFloat(autoThreshold) * 100)}%.
            </p>
          </div>
        )}

        <div className="space-y-2">
          <Label>Daily application limit</Label>
          <Input
            type="number"
            min="1"
            max="80"
            value={dailyLimit}
            onChange={(e) => setDailyLimit(e.target.value)}
          />
          <p className="text-xs text-gray-400">
            Max applications per day (across all boards). Recommended: 5–15 for safety.
          </p>
        </div>

        <Button onClick={handleSave} loading={saving}>
          Save changes
        </Button>
      </CardContent>
    </Card>
  );
}
