import Link from "next/link";
import { ExternalLink, Building2, MapPin, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StatusTimeline } from "./status-timeline";
import { formatDate } from "@/lib/utils";
import type { Application } from "@/lib/types";

interface ApplicationRowProps {
  application: Application;
}

export function ApplicationRow({ application }: ApplicationRowProps) {
  const { job } = application;

  return (
    <div className="bg-white rounded-xl border border-gray-100 shadow-sm hover:shadow-card-hover transition-shadow p-4 space-y-3">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <a
            href={job.url}
            target="_blank"
            rel="noopener noreferrer"
            className="group inline-flex items-center gap-1 font-semibold text-gray-900 hover:text-brand-600 transition-colors text-sm"
          >
            {job.title}
            <ExternalLink className="h-3 w-3 opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
          </a>
          <div className="flex items-center gap-3 mt-0.5 text-xs text-gray-500">
            <span className="flex items-center gap-1">
              <Building2 className="h-3 w-3 text-gray-400" />
              {job.company}
            </span>
            <span className="flex items-center gap-1">
              <MapPin className="h-3 w-3 text-gray-400" />
              {job.isRemote ? "Remote" : job.location}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          {job.atsType && job.atsType !== "unknown" && (
            <Badge variant="brand" className="text-[10px]">
              {job.atsType}
            </Badge>
          )}
          {application.errorMessage && (
            <Badge variant="failed" className="text-[10px]">
              Error
            </Badge>
          )}
          <span className="text-xs text-gray-400">
            {formatDate(application.createdAt)}
          </span>
        </div>
      </div>

      {/* Status pipeline */}
      <StatusTimeline
        status={application.status}
        outcome={application.outcome}
      />

      {/* Error message */}
      {application.errorMessage && (
        <p className="text-xs text-red-600 bg-red-50 rounded-md px-3 py-2">
          {application.errorMessage}
        </p>
      )}

      {/* Footer: Details link */}
      <div className="flex justify-end pt-0.5">
        <Link
          href={`/dashboard/applications/${application.id}`}
          className="inline-flex items-center gap-0.5 text-xs text-gray-400 hover:text-brand-600 transition-colors"
        >
          Details
          <ChevronRight className="h-3.5 w-3.5" />
        </Link>
      </div>
    </div>
  );
}
