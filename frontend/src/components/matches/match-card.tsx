"use client";

import { useState } from "react";
import {
  Building2,
  MapPin,
  DollarSign,
  ExternalLink,
  ThumbsUp,
  ThumbsDown,
  Zap,
  Clock,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { setMatchFeedback, updateMatchStatus, type MatchStatusUpdateResponse } from "@/lib/api";
import { formatSalary, scoreColor, scorePercent, timeAgo } from "@/lib/utils";
import type { Match } from "@/lib/types";
import { SalaryBenchmarkBadge } from "@/components/salary/salary-benchmark-badge";

interface MatchCardProps {
  match: Match;
  onUpdate: (updated: MatchStatusUpdateResponse) => void;
  /** Whether this card is currently selected for bulk action */
  selected?: boolean;
  /** Called when the user toggles the selection checkbox */
  onToggleSelect?: (id: string) => void;
}

export function MatchCard({ match, onUpdate, selected, onToggleSelect }: MatchCardProps) {
  const [loading, setLoading] = useState<"approve" | "reject" | null>(null);

  // ── Match quality feedback (thumbs up/down — separate from approve/reject) ──
  const [feedback, setFeedback] = useState<"thumbs_up" | "thumbs_down" | undefined>(
    match.userFeedback
  );
  const [feedbackLoading, setFeedbackLoading] = useState<"thumbs_up" | "thumbs_down" | null>(null);

  async function handleFeedback(value: "thumbs_up" | "thumbs_down") {
    if (feedbackLoading) return;
    setFeedbackLoading(value);
    const prev = feedback;
    setFeedback(value); // optimistic
    try {
      await setMatchFeedback(match.id, value);
    } catch (e) {
      console.error("Failed to set match feedback", e);
      setFeedback(prev); // rollback
    } finally {
      setFeedbackLoading(null);
    }
  }

  async function handleApprove() {
    setLoading("approve");
    try {
      const updated = await updateMatchStatus(match.id, "approved", "thumbs_up");
      onUpdate(updated);
    } catch (e) {
      console.error("Failed to approve match", e);
    } finally {
      setLoading(null);
    }
  }

  async function handleReject() {
    setLoading("reject");
    try {
      const updated = await updateMatchStatus(match.id, "rejected", "thumbs_down");
      onUpdate(updated);
    } catch (e) {
      console.error("Failed to reject match", e);
    } finally {
      setLoading(null);
    }
  }

  const { job } = match;
  const scoreText = scorePercent(match.matchScore);
  const scoreClass = scoreColor(match.matchScore);

  return (
    <Card
      className={`hover:shadow-card-hover transition-shadow relative ${
        selected ? "ring-2 ring-brand-500" : ""
      }`}
    >
      {/* Selection checkbox — only rendered when bulk-select mode is active */}
      {onToggleSelect && (
        <button
          type="button"
          aria-label={selected ? "Deselect" : "Select"}
          onClick={(e) => {
            e.stopPropagation();
            onToggleSelect(match.id);
          }}
          className={`absolute top-3 left-3 z-10 h-5 w-5 rounded border-2 flex items-center justify-center transition-colors ${
            selected
              ? "bg-brand-500 border-brand-500 text-white"
              : "border-gray-300 bg-white hover:border-brand-400"
          }`}
        >
          {selected && (
            <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none">
              <path
                d="M2 6l3 3 5-5"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          )}
        </button>
      )}

      <CardContent className={`pt-5 ${onToggleSelect ? "pl-10" : ""}`}>
        {/* Header row */}
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="flex-1 min-w-0">
            <a
              href={job.url}
              target="_blank"
              rel="noopener noreferrer"
              className="group inline-flex items-center gap-1 font-semibold text-gray-900 hover:text-brand-600 transition-colors text-sm leading-tight"
            >
              {job.title}
              <ExternalLink className="h-3 w-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
            </a>
            <div className="flex items-center gap-1.5 mt-0.5">
              <Building2 className="h-3.5 w-3.5 text-gray-400" />
              <span className="text-sm text-gray-600 truncate">{job.company}</span>
            </div>
          </div>

          {/* Match score pill */}
          <div className="flex flex-col items-end gap-1 shrink-0">
            <span className={`text-lg font-bold tabular-nums ${scoreClass}`}>{scoreText}</span>
            <span className="text-xs text-gray-400">match</span>
          </div>
        </div>

        {/* Meta row */}
        <div className="flex flex-wrap items-center gap-3 text-xs text-gray-500 mb-3">
          <span className="flex items-center gap-1">
            <MapPin className="h-3 w-3" />
            {job.isRemote ? "Remote" : job.location}
          </span>
          <span className="flex items-center gap-1">
            <DollarSign className="h-3 w-3" />
            {formatSalary(job.salaryMin, job.salaryMax, job.salaryCurrency)}
          </span>
          <span className="flex items-center gap-1">
            <Clock className="h-3 w-3" />
            {timeAgo(match.createdAt)}
          </span>
        </div>

        {/* Salary benchmark badge */}
        {job.title && (
          <SalaryBenchmarkBadge
            title={job.title}
            location={job.location ?? ''}
            salaryMin={job.salaryMin}
            salaryMax={job.salaryMax}
          />
        )}

        {/* Score breakdown */}
        <div className="flex flex-wrap gap-2 mb-3">
          {Object.entries(match.matchBreakdown).map(([key, val]) => (
            <div key={key} className="flex items-center gap-1 text-xs">
              <span className="text-gray-400 capitalize">{key}</span>
              <span className={`font-semibold ${scoreColor(val)}`}>
                {scorePercent(val)}
              </span>
            </div>
          ))}
        </div>

        {/* Feedback row — rate match quality (for ML training) */}
        <div className="flex items-center gap-2 mb-3 pt-2 border-t border-gray-100">
          <span className="text-xs text-gray-400">Rate this match:</span>
          <button
            type="button"
            aria-label="Good match"
            onClick={() => handleFeedback("thumbs_up")}
            disabled={!!feedbackLoading}
            className={`p-1 rounded transition-colors disabled:opacity-50 ${
              feedback === "thumbs_up"
                ? "text-green-600 bg-green-50"
                : "text-gray-400 hover:text-green-600 hover:bg-green-50"
            }`}
          >
            <ThumbsUp
              className={`h-3.5 w-3.5 ${feedbackLoading === "thumbs_up" ? "animate-pulse" : ""}`}
            />
          </button>
          <button
            type="button"
            aria-label="Poor match"
            onClick={() => handleFeedback("thumbs_down")}
            disabled={!!feedbackLoading}
            className={`p-1 rounded transition-colors disabled:opacity-50 ${
              feedback === "thumbs_down"
                ? "text-red-500 bg-red-50"
                : "text-gray-400 hover:text-red-500 hover:bg-red-50"
            }`}
          >
            <ThumbsDown
              className={`h-3.5 w-3.5 ${feedbackLoading === "thumbs_down" ? "animate-pulse" : ""}`}
            />
          </button>
        </div>

        {/* Badges */}
        <div className="flex flex-wrap gap-1.5">
          <Badge variant="default">{job.sourceBoard}</Badge>
          {job.atsType && job.atsType !== "unknown" && (
            <Badge variant="brand">
              <Zap className="h-3 w-3" />
              {job.atsType}
            </Badge>
          )}
          {job.isRemote && <Badge variant="success">Remote</Badge>}
          {job.isScam && <Badge variant="danger">Potential Scam</Badge>}
        </div>
      </CardContent>

      <CardFooter className="gap-2">
        <Button
          variant="success"
          size="sm"
          className="flex-1"
          onClick={handleApprove}
          loading={loading === "approve"}
          disabled={!!loading}
        >
          <ThumbsUp className="h-3.5 w-3.5" />
          Apply
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="flex-1"
          onClick={handleReject}
          loading={loading === "reject"}
          disabled={!!loading}
        >
          <ThumbsDown className="h-3.5 w-3.5" />
          Skip
        </Button>
        <Button
          variant="ghost"
          size="sm"
          asChild
        >
          <a href={job.url} target="_blank" rel="noopener noreferrer">
            View
          </a>
        </Button>
      </CardFooter>
    </Card>
  );
}
