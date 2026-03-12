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
import { updateMatchStatus } from "@/lib/api";
import { formatSalary, scoreColor, scorePercent, timeAgo } from "@/lib/utils";
import type { Match } from "@/lib/types";

interface MatchCardProps {
  match: Match;
  onUpdate: (updated: Match) => void;
}

export function MatchCard({ match, onUpdate }: MatchCardProps) {
  const [loading, setLoading] = useState<"approve" | "reject" | null>(null);

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
    <Card className="hover:shadow-card-hover transition-shadow">
      <CardContent className="pt-5">
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
