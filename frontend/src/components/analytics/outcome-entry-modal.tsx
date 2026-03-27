"use client";

/**
 * OutcomeEntryModal — prompts the user to record an outcome
 * for a specific application (e.g. 7 days after it was submitted).
 *
 * Usage:
 *   <OutcomeEntryModal
 *     application={app}
 *     open={open}
 *     onClose={() => setOpen(false)}
 *     onSaved={() => reload()}
 *   />
 */

import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { updateApplicationOutcome } from "@/lib/api";
import type { Application, ApplicationOutcome } from "@/lib/types";

const OUTCOMES: { value: ApplicationOutcome; label: string; color: string }[] = [
  { value: "viewed",    label: "They viewed my profile",    color: "text-blue-600"   },
  { value: "interview", label: "I got an interview",        color: "text-amber-600"  },
  { value: "offer",     label: "I received an offer",       color: "text-green-600"  },
  { value: "rejected",  label: "I was rejected / ghosted",  color: "text-red-500"    },
];

interface OutcomeEntryModalProps {
  application: Application;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

export function OutcomeEntryModal({
  application,
  open,
  onClose,
  onSaved,
}: OutcomeEntryModalProps) {
  const [selected, setSelected] = useState<ApplicationOutcome | null>(
    application.outcome ?? null
  );
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSave() {
    if (!selected) return;
    setSaving(true);
    setError(null);
    try {
      await updateApplicationOutcome(application.id, selected, notes.trim() || undefined);
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  const company = application.job?.company ?? "the company";
  const title   = application.job?.title   ?? "this role";

  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 backdrop-blur-sm z-40 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-2xl bg-white p-6 shadow-xl focus:outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
          {/* Header */}
          <div className="flex items-start justify-between gap-3 mb-5">
            <div>
              <Dialog.Title className="text-base font-semibold text-gray-900">
                How did it go?
              </Dialog.Title>
              <Dialog.Description className="mt-1 text-sm text-gray-500">
                Did you hear back from <strong>{company}</strong> about <em>{title}</em>?
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <button
                className="rounded-md p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors"
                aria-label="Close"
              >
                <X className="h-4 w-4" />
              </button>
            </Dialog.Close>
          </div>

          {/* Outcome options */}
          <div className="space-y-2 mb-4">
            {OUTCOMES.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setSelected(opt.value)}
                className={`w-full text-left rounded-lg border px-4 py-3 text-sm transition-colors ${
                  selected === opt.value
                    ? "border-brand-500 bg-brand-50"
                    : "border-gray-200 hover:border-gray-300 hover:bg-gray-50"
                }`}
              >
                <span className={selected === opt.value ? "font-medium text-brand-700" : opt.color}>
                  {opt.label}
                </span>
              </button>
            ))}
          </div>

          {/* Optional notes */}
          <div className="mb-5">
            <label className="block text-xs font-medium text-gray-700 mb-1">
              Notes <span className="text-gray-400 font-normal">(optional)</span>
            </label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={2}
              placeholder="e.g. Scheduled interview for March 20 at 2 PM"
              className="w-full resize-none rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>

          {error && (
            <p className="mb-3 text-xs text-red-600 bg-red-50 rounded-md px-3 py-2">{error}</p>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={onClose} disabled={saving}>
              Skip
            </Button>
            <Button
              size="sm"
              onClick={handleSave}
              loading={saving}
              disabled={!selected}
            >
              Save outcome
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
