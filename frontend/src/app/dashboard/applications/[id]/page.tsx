"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  ExternalLink,
  FileText,
  Mail,
  Camera,
  AlertCircle,
  Calendar,
  CheckCircle,
  Clock,
} from "lucide-react";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Alert } from "@/components/ui/alert";
import { StatusTimeline } from "@/components/applications/status-timeline";
import { getApplication, withdrawApplication } from "@/lib/api";
import type { Application, ApplicationStatus } from "@/lib/types";
import { formatDate, formatSalary, timeAgo, statusLabel } from "@/lib/utils";

// Statuses from which the user can no longer withdraw.
const TERMINAL_STATUSES = new Set<ApplicationStatus>([
  "applied",
  "failed",
  "withdrawn",
]);

export default function ApplicationDetailPage() {
  const { id } = useParams<{ id: string }>();

  const [app, setApp] = useState<Application | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    getApplication(id)
      .then(setApp)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load application"))
      .finally(() => setLoading(false));
  }, [id]);

  async function handleWithdraw() {
    if (!app) return;
    setWithdrawing(true);
    setWithdrawError(null);
    try {
      await withdrawApplication(app.id);
      setApp((prev) => prev ? { ...prev, status: "withdrawn" } : prev);
    } catch (e) {
      setWithdrawError(e instanceof Error ? e.message : "Failed to withdraw application");
    } finally {
      setWithdrawing(false);
    }
  }

  const canWithdraw = app ? !TERMINAL_STATUSES.has(app.status) : false;

  const backButton = (
    <Button asChild variant="ghost" size="sm">
      <Link href="/dashboard/applications">
        <ArrowLeft className="h-4 w-4" />
        Back
      </Link>
    </Button>
  );

  return (
    <div className="flex flex-col h-full">
      <Header
        title="Application Detail"
        subtitle={app ? `${app.job.title} at ${app.job.company}` : undefined}
        actions={backButton}
      />

      <div className="flex-1 overflow-y-auto px-6 py-6">
        {loading ? (
          <div className="flex items-center justify-center py-24">
            <Spinner size="lg" />
          </div>
        ) : error ? (
          <Alert variant="error" title="Failed to load application">
            {error}
          </Alert>
        ) : app ? (
          <div className="max-w-2xl mx-auto space-y-6">

            {/* ── Job info card ──────────────────────────────────────────────── */}
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5 space-y-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <h2 className="text-lg font-semibold text-gray-900 truncate">
                    {app.job.title}
                  </h2>
                  <p className="text-sm text-gray-600 mt-0.5">
                    {app.job.company}
                    {app.job.location && (
                      <span className="text-gray-400"> · {app.job.location}</span>
                    )}
                  </p>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Badge variant={app.status as ApplicationStatus}>{statusLabel(app.status)}</Badge>
                  {app.job.url && (
                    <a
                      href={app.job.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-gray-400 hover:text-brand-500 transition-colors"
                      aria-label="View job posting"
                    >
                      <ExternalLink className="h-4 w-4" />
                    </a>
                  )}
                </div>
              </div>

              {/* Metadata row */}
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500">
                {(app.job.salaryMin || app.job.salaryMax) && (
                  <span>
                    {formatSalary(app.job.salaryMin, app.job.salaryMax, app.job.salaryCurrency ?? "USD")}
                  </span>
                )}
                {app.job.isRemote && (
                  <span className="text-green-600 font-medium">Remote</span>
                )}
                {app.job.atsType && app.job.atsType !== "unknown" && (
                  <span className="capitalize">{app.job.atsType}</span>
                )}
                {app.job.sourceBoard && (
                  <span className="capitalize">{app.job.sourceBoard}</span>
                )}
              </div>
            </div>

            {/* ── Application pipeline ─────────────────────────────────────── */}
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-4">Application Pipeline</h3>
              <StatusTimeline status={app.status} outcome={app.outcome} />
            </div>

            {/* ── AI Artifacts ─────────────────────────────────────────────── */}
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">AI Artifacts</h3>
              <div className="flex flex-wrap gap-2">
                <ArtifactBadge
                  icon={<FileText className="h-3.5 w-3.5" />}
                  label="Tailored Resume"
                  present={!!app.tailoredResumeS3}
                />
                <ArtifactBadge
                  icon={<Mail className="h-3.5 w-3.5" />}
                  label="Cover Letter"
                  present={!!app.coverLetterS3}
                />
                <ArtifactBadge
                  icon={<Camera className="h-3.5 w-3.5" />}
                  label="Submission Screenshot"
                  present={!!app.screenshotS3}
                />
              </div>
              {!app.tailoredResumeS3 && !app.coverLetterS3 && !app.screenshotS3 && (
                <p className="text-xs text-gray-400 mt-2">No AI artifacts generated yet.</p>
              )}
            </div>

            {/* ── Error card ───────────────────────────────────────────────── */}
            {app.errorMessage && (
              <div className="bg-red-50 border border-red-200 rounded-xl p-4 flex gap-3">
                <AlertCircle className="h-5 w-5 text-red-500 shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-medium text-red-700">Application Error</p>
                  <p className="text-sm text-red-600 mt-0.5">{app.errorMessage}</p>
                </div>
              </div>
            )}

            {/* ── Timestamps ───────────────────────────────────────────────── */}
            <div className="bg-white rounded-xl border border-gray-100 shadow-sm p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Timeline</h3>
              <dl className="space-y-2">
                <TimelineRow
                  icon={<Clock className="h-3.5 w-3.5 text-gray-400" />}
                  label="Created"
                  value={formatDate(app.createdAt)}
                  detail={timeAgo(app.createdAt)}
                />
                {app.appliedAt && (
                  <TimelineRow
                    icon={<CheckCircle className="h-3.5 w-3.5 text-green-500" />}
                    label="Submitted"
                    value={formatDate(app.appliedAt)}
                    detail={timeAgo(app.appliedAt)}
                  />
                )}
                {app.outcomeUpdatedAt && (
                  <TimelineRow
                    icon={<Calendar className="h-3.5 w-3.5 text-blue-500" />}
                    label="Outcome updated"
                    value={formatDate(app.outcomeUpdatedAt)}
                    detail={timeAgo(app.outcomeUpdatedAt)}
                  />
                )}
              </dl>
            </div>

            {/* ── Withdraw action ──────────────────────────────────────────── */}
            {withdrawError && (
              <Alert variant="error" title="Withdraw failed">
                {withdrawError}
              </Alert>
            )}
            {canWithdraw && (
              <div className="flex justify-end">
                <Button
                  variant="outline"
                  onClick={handleWithdraw}
                  loading={withdrawing}
                  disabled={withdrawing}
                  className="text-red-600 border-red-200 hover:bg-red-50 hover:border-red-300"
                >
                  Withdraw Application
                </Button>
              </div>
            )}

          </div>
        ) : null}
      </div>
    </div>
  );
}

// ── Sub-components ─────────────────────────────────────────────────────────────

interface ArtifactBadgeProps {
  icon: React.ReactNode;
  label: string;
  present: boolean;
}

function ArtifactBadge({ icon, label, present }: ArtifactBadgeProps) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${
        present
          ? "bg-green-50 border-green-200 text-green-700"
          : "bg-gray-50 border-gray-200 text-gray-400"
      }`}
    >
      {icon}
      {label}
      {present ? (
        <CheckCircle className="h-3 w-3 text-green-500" />
      ) : (
        <Clock className="h-3 w-3 text-gray-300" />
      )}
    </span>
  );
}

interface TimelineRowProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  detail?: string;
}

function TimelineRow({ icon, label, value, detail }: TimelineRowProps) {
  return (
    <div className="flex items-center gap-2 text-sm">
      {icon}
      <dt className="text-gray-500 w-32 shrink-0">{label}</dt>
      <dd className="text-gray-900 font-medium">
        {value}
        {detail && <span className="text-gray-400 font-normal ml-1.5">({detail})</span>}
      </dd>
    </div>
  );
}
