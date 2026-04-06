"use client";

import { useState } from "react";
import { Zap } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { StepResume } from "@/components/onboarding/step-resume";
import { StepPreferences } from "@/components/onboarding/step-preferences";
import { StepComplete } from "@/components/onboarding/step-complete";
type Step = "resume" | "preferences" | "complete";

const STEPS: Step[] = ["resume", "preferences", "complete"];

const STEP_LABELS: Record<Step, string> = {
  resume: "Upload Resume",
  preferences: "Job Preferences",
  complete: "Ready!",
};

export default function OnboardingPage() {
  const [step, setStep] = useState<Step>("resume");

  const currentIndex = STEPS.indexOf(step);
  const totalSteps = STEPS.length;
  const progressPct = ((currentIndex + 1) / totalSteps) * 100;

  return (
    <div className="min-h-screen bg-surface flex items-center justify-center p-4">
      <div className="w-full max-w-lg">
        {/* Logo */}
        <div className="flex items-center gap-3 mb-8 justify-center">
          <div className="w-9 h-9 bg-brand-600 rounded-lg flex items-center justify-center shadow">
            <Zap className="h-5 w-5 text-white" />
          </div>
          <span className="text-lg font-bold text-gray-900">AutoDreamApplier</span>
        </div>

        {/* Progress */}
        <div className="mb-6 space-y-3">
          <div className="flex items-center justify-between text-xs text-gray-400">
            {STEPS.map((s, i) => (
              <span
                key={s}
                className={`font-medium ${i <= currentIndex ? "text-brand-600" : "text-gray-400"}`}
              >
                {STEP_LABELS[s]}
              </span>
            ))}
          </div>
          {/* Animated progress bar */}
          <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
            <motion.div
              initial={{ width: "0%" }}
              animate={{ width: `${progressPct}%` }}
              transition={{ duration: 0.4, ease: [0.4, 0, 0.2, 1] }}
              className="h-full rounded-full bg-gradient-to-r from-indigo-500 to-violet-500"
            />
          </div>
        </div>

        {/* Step card */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-6 overflow-hidden">
          <AnimatePresence mode="wait">
            <motion.div
              key={step}
              initial={{ opacity: 0, x: 32 }}
              animate={{ opacity: 1, x: 0, transition: { duration: 0.28, ease: [0.16, 1, 0.3, 1] } }}
              exit={{ opacity: 0, x: -32, transition: { duration: 0.18 } }}
            >
              {step === "resume" && (
                <StepResume
                  onComplete={() => {
                    setStep("preferences");
                  }}
                />
              )}
              {step === "preferences" && (
                <StepPreferences
                  onComplete={() => {
                    setStep("complete");
                  }}
                />
              )}
              {step === "complete" && <StepComplete />}
            </motion.div>
          </AnimatePresence>
        </div>

        {/* Step dots */}
        <div className="flex justify-center gap-2 mt-5">
          {STEPS.map((s, i) => {
            const isActive = i === currentIndex;
            const isCompleted = i < currentIndex;
            return (
              <motion.div
                key={s}
                animate={{
                  scale: isActive ? 1.3 : 1,
                  backgroundColor: isCompleted ? '#6366f1' : isActive ? '#6366f1' : '#e2e8f0',
                }}
                transition={{ type: "spring", stiffness: 400, damping: 25 }}
                className="w-2.5 h-2.5 rounded-full"
              />
            );
          })}
        </div>
      </div>
    </div>
  );
}
