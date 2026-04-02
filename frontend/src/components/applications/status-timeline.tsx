'use client';

import { motion } from "framer-motion";
import { Check, Loader2, X, Clock } from "lucide-react";
import type { ApplicationStatus, ApplicationOutcome } from "@/lib/types";

const PIPELINE_STEPS: { key: ApplicationStatus; label: string }[] = [
  { key: "queued", label: "Queued" },
  { key: "ai_preparing", label: "AI Prep" },
  { key: "ai_ready", label: "Ready" },
  { key: "applying", label: "Applying" },
  { key: "applied", label: "Applied" },
];

const STATUS_ORDER: Record<ApplicationStatus, number> = {
  queued: 0,
  ai_preparing: 1,
  ai_ready: 2,
  applying: 3,
  applied: 4,
  failed: 4,
  withdrawn: 4,
};

const OUTCOME_COLORS: Record<ApplicationOutcome, string> = {
  viewed: "text-blue-600",
  rejected: "text-red-600",
  interview: "text-green-600",
  offer: "text-emerald-700",
};

const OUTCOME_LABELS: Record<ApplicationOutcome, string> = {
  viewed: "Viewed",
  rejected: "Rejected",
  interview: "Interview 🎉",
  offer: "Offer 🏆",
};

interface StatusTimelineProps {
  status: ApplicationStatus;
  outcome?: ApplicationOutcome;
}

export function StatusTimeline({ status, outcome }: StatusTimelineProps) {
  const currentIndex = STATUS_ORDER[status];
  const isFailed = status === "failed" || status === "withdrawn";

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-0">
        {PIPELINE_STEPS.map((step, i) => {
          const stepIndex = STATUS_ORDER[step.key];
          const isDone = !isFailed && currentIndex > stepIndex;
          const isActive = !isFailed && currentIndex === stepIndex;
          const isError = isFailed && i === PIPELINE_STEPS.length - 1;
          const stepCompleted = isDone;

          return (
            <div key={step.key} className="flex items-center flex-1">
              {/* Animated node */}
              <div className="flex flex-col items-center">
                <motion.div
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: i * 0.08, type: 'spring', stiffness: 400, damping: 25 }}
                  className={`w-7 h-7 rounded-full flex items-center justify-center border-2 transition-colors ${
                    isDone
                      ? "bg-brand-600 border-brand-600"
                      : isActive
                      ? "bg-white border-brand-500"
                      : isError
                      ? "bg-red-100 border-red-400"
                      : "bg-white border-gray-200"
                  }`}
                >
                  {isDone ? (
                    <Check className="h-3.5 w-3.5 text-white" />
                  ) : isActive ? (
                    <Loader2 className="h-3.5 w-3.5 text-brand-500 animate-spin" />
                  ) : isError ? (
                    <X className="h-3.5 w-3.5 text-red-500" />
                  ) : (
                    <Clock className="h-3 w-3 text-gray-300" />
                  )}
                </motion.div>
                <span
                  className={`text-[10px] mt-1 font-medium whitespace-nowrap ${
                    isDone || isActive
                      ? "text-brand-600"
                      : isError
                      ? "text-red-500"
                      : "text-gray-400"
                  }`}
                >
                  {i === PIPELINE_STEPS.length - 1 && isFailed
                    ? status === "withdrawn"
                      ? "Withdrawn"
                      : "Failed"
                    : step.label}
                </span>
              </div>

              {/* Animated connector line */}
              {i < PIPELINE_STEPS.length - 1 && (
                <div className="flex-1 h-0.5 mx-0.5 mb-4 bg-gray-200 overflow-hidden rounded-full">
                  <motion.div
                    initial={{ scaleX: 0 }}
                    animate={{ scaleX: stepCompleted ? 1 : 0 }}
                    style={{ originX: 0 }}
                    transition={{ duration: 0.4, delay: i * 0.08 + 0.1 }}
                    className="h-0.5 w-full bg-indigo-500"
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Outcome badge */}
      {outcome && (
        <motion.p
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className={`text-xs font-semibold ${OUTCOME_COLORS[outcome]}`}
        >
          {OUTCOME_LABELS[outcome]}
        </motion.p>
      )}
    </div>
  );
}
