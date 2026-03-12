"use client";

import { useState } from "react";
import { Zap } from "lucide-react";
import { Progress } from "@/components/ui/progress";
import { StepResume } from "@/components/onboarding/step-resume";
import { StepPreferences } from "@/components/onboarding/step-preferences";
import { StepComplete } from "@/components/onboarding/step-complete";
import type { Resume, UserPreferences } from "@/lib/types";

type Step = "resume" | "preferences" | "complete";

const STEPS: Step[] = ["resume", "preferences", "complete"];

const STEP_LABELS: Record<Step, string> = {
  resume: "Upload Resume",
  preferences: "Job Preferences",
  complete: "Ready!",
};

export default function OnboardingPage() {
  const [step, setStep] = useState<Step>("resume");
  const [resume, setResume] = useState<Resume | null>(null);
  const [preferences, setPreferences] = useState<UserPreferences | null>(null);

  const currentIndex = STEPS.indexOf(step);
  const progress = ((currentIndex + 1) / STEPS.length) * 100;

  // Suppress unused-variable warnings — stored for potential future use
  void resume;
  void preferences;

  return (
    <div className="min-h-screen bg-surface flex items-center justify-center p-4">
      <div className="w-full max-w-lg">
        {/* Header */}
        <div className="flex items-center gap-3 mb-8 justify-center">
          <div className="w-9 h-9 bg-brand-600 rounded-lg flex items-center justify-center shadow">
            <Zap className="h-5 w-5 text-white" />
          </div>
          <span className="text-lg font-bold text-gray-900">AutoDreamApplier</span>
        </div>

        {/* Progress */}
        <div className="mb-6 space-y-2">
          <div className="flex items-center justify-between text-xs text-gray-400">
            {STEPS.map((s, i) => (
              <span
                key={s}
                className={`font-medium ${
                  i <= currentIndex ? "text-brand-600" : "text-gray-400"
                }`}
              >
                {STEP_LABELS[s]}
              </span>
            ))}
          </div>
          <Progress value={progress} color="brand" size="sm" />
        </div>

        {/* Step card */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-6">
          {step === "resume" && (
            <StepResume
              onComplete={(r) => {
                setResume(r);
                setStep("preferences");
              }}
            />
          )}

          {step === "preferences" && (
            <StepPreferences
              onComplete={(p) => {
                setPreferences(p);
                setStep("complete");
              }}
            />
          )}

          {step === "complete" && <StepComplete />}
        </div>

        {/* Step indicators */}
        <div className="flex justify-center gap-2 mt-5">
          {STEPS.map((s, i) => (
            <div
              key={s}
              className={`w-2 h-2 rounded-full transition-colors ${
                i === currentIndex
                  ? "bg-brand-600"
                  : i < currentIndex
                  ? "bg-brand-300"
                  : "bg-gray-200"
              }`}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
