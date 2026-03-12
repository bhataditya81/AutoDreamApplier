"use client";

import { useState } from "react";
import { Lock, Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { saveBoardCredentials } from "@/lib/api";

const BOARDS = [
  { id: "indeed", label: "Indeed", placeholder: "indeed.com email" },
  { id: "glassdoor", label: "Glassdoor", placeholder: "glassdoor.com email" },
  { id: "linkedin", label: "LinkedIn (Phase 2)", placeholder: "linkedin.com email", disabled: true },
];

interface CredentialState {
  email: string;
  password: string;
  showPassword: boolean;
  saving: boolean;
  saved: boolean;
  error: string | null;
}

function BoardCredentialCard({
  board,
}: {
  board: { id: string; label: string; placeholder: string; disabled?: boolean };
}) {
  const [state, setState] = useState<CredentialState>({
    email: "",
    password: "",
    showPassword: false,
    saving: false,
    saved: false,
    error: null,
  });

  function update(patch: Partial<CredentialState>) {
    setState((prev) => ({ ...prev, ...patch }));
  }

  async function handleSave() {
    if (!state.email || !state.password) {
      update({ error: "Both email and password are required." });
      return;
    }
    update({ saving: true, error: null, saved: false });
    try {
      await saveBoardCredentials(board.id, state.email, state.password);
      update({ saving: false, saved: true, email: "", password: "" });
      setTimeout(() => update({ saved: false }), 4000);
    } catch (e: unknown) {
      console.error("[CredentialsForm] saveBoardCredentials error:", e);
      update({
        saving: false,
        error: e instanceof Error ? e.message : "Failed to save credentials.",
      });
    }
  }

  return (
    <div
      className={`rounded-xl border p-4 space-y-3 ${
        board.disabled ? "opacity-50 pointer-events-none" : ""
      }`}
    >
      <div className="flex items-center gap-2">
        <Lock className="h-4 w-4 text-gray-400" />
        <span className="font-medium text-sm text-gray-900">{board.label}</span>
        {board.disabled && (
          <span className="text-xs bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full">
            Coming soon
          </span>
        )}
      </div>

      {state.error && <Alert variant="error">{state.error}</Alert>}
      {state.saved && (
        <Alert variant="success">Credentials saved and encrypted.</Alert>
      )}

      <div className="space-y-2">
        <Label>Email</Label>
        <Input
          type="email"
          placeholder={board.placeholder}
          value={state.email}
          onChange={(e) => update({ email: e.target.value })}
          autoComplete="off"
        />
      </div>

      <div className="space-y-2">
        <Label>Password</Label>
        <div className="relative">
          <Input
            type={state.showPassword ? "text" : "password"}
            placeholder="••••••••"
            value={state.password}
            onChange={(e) => update({ password: e.target.value })}
            autoComplete="new-password"
            className="pr-10"
          />
          <button
            type="button"
            onClick={() => update({ showPassword: !state.showPassword })}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
          >
            {state.showPassword ? (
              <EyeOff className="h-4 w-4" />
            ) : (
              <Eye className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>

      <Button size="sm" onClick={handleSave} loading={state.saving}>
        Save {board.label} credentials
      </Button>
    </div>
  );
}

export function CredentialsForm() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Job Board Credentials</CardTitle>
        <CardDescription>
          Credentials are encrypted with AES-256-GCM and stored securely. We never
          share or log your passwords.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Alert variant="info" title="Why do we need credentials?">
          AutoDreamApplier uses your board accounts to submit applications on your behalf.
          Credentials are encrypted at rest and only decrypted inside our secure VPC during
          browser automation runs.
        </Alert>

        {BOARDS.map((board) => (
          <BoardCredentialCard key={board.id} board={board} />
        ))}
      </CardContent>
    </Card>
  );
}
