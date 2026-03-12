"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { LayoutList, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Alert } from "@/components/ui/alert";
import { ApplicationRow } from "./application-row";
import { listApplications } from "@/lib/api";
import type { Application, ApplicationStatus } from "@/lib/types";

const STATUS_TABS: { label: string; value: ApplicationStatus | "all" }[] = [
  { label: "All", value: "all" },
  { label: "Applied", value: "applied" },
  { label: "Applying", value: "applying" },
  { label: "AI Prep", value: "ai_preparing" },
  { label: "Failed", value: "failed" },
];

export function ApplicationList() {
  const [applications, setApplications] = useState<Application[]>([]);
  const [status, setStatus] = useState<ApplicationStatus | "all">("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);

  // ── Search ────────────────────────────────────────────────────────────────
  // Two-state pattern: `search` for controlled input, `debouncedSearch` for
  // the actual API param — updated 300 ms after the user stops typing.
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(id);
  }, [search]);

  // Clear search whenever the status tab changes
  useEffect(() => {
    setSearch("");
    setDebouncedSearch("");
  }, [status]);

  const fetchApplications = useCallback(
    async (pageNum: number, replace: boolean) => {
      try {
        setError(null);
        const params: Record<string, string | number> = { page: pageNum, limit: 15 };
        if (status !== "all") params.status = status;
        if (debouncedSearch) params.search = debouncedSearch;
        const res = await listApplications(params);
        setApplications((prev) => (replace ? res.data : [...prev, ...res.data]));
        setHasMore(res.hasMore);
      } catch (e: unknown) {
        console.error("[ApplicationList] fetchApplications error:", e);
        setError(e instanceof Error ? e.message : "Failed to load applications");
      }
    },
    [status, debouncedSearch]
  );

  useEffect(() => {
    setPage(1);
    setApplications([]);
    setLoading(true);
    fetchApplications(1, true).finally(() => setLoading(false));
  }, [fetchApplications]);

  async function handleLoadMore() {
    const next = page + 1;
    setPage(next);
    await fetchApplications(next, false);
  }

  // ── Status polling ────────────────────────────────────────────────────────
  // Re-fetch every 10 s while any application is in a transient state.
  // Uses a ref guard so concurrent interval ticks don't overlap.
  const IN_FLIGHT_STATUSES: (ApplicationStatus | "all")[] = [
    "queued", "ai_preparing", "ai_ready", "applying",
  ];
  const POLLING_INTERVAL_MS = 10_000;
  const isFetchingRef = useRef(false);

  const hasInFlight = applications.some((a) =>
    IN_FLIGHT_STATUSES.includes(a.status as ApplicationStatus)
  );

  useEffect(() => {
    if (!hasInFlight) return;

    const id = setInterval(async () => {
      if (isFetchingRef.current) return;
      isFetchingRef.current = true;
      try {
        await fetchApplications(1, true);
        setPage(1);
      } finally {
        isFetchingRef.current = false;
      }
    }, POLLING_INTERVAL_MS);

    return () => clearInterval(id);
  }, [hasInFlight, fetchApplications]);

  return (
    <div className="space-y-4">
      {/* Toolbar: tabs + search */}
      <div className="flex items-center gap-3 flex-wrap">
        {/* Status tabs */}
        <div className="flex gap-1 bg-gray-100 rounded-lg p-1">
          {STATUS_TABS.map((tab) => (
            <button
              key={tab.value}
              onClick={() => setStatus(tab.value)}
              className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
                status === tab.value
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Search input */}
        <div className="relative flex-1 min-w-[180px] max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-gray-400 pointer-events-none" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by title or company…"
            className="w-full rounded-md border border-gray-200 bg-white py-1.5 pl-8 pr-8 text-sm text-gray-900 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch("")}
              aria-label="Clear search"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      {error && (
        <Alert variant="error" title="Failed to load applications">
          {error}
        </Alert>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Spinner size="lg" />
        </div>
      ) : applications.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mb-4">
            <LayoutList className="h-8 w-8 text-gray-400" />
          </div>
          {debouncedSearch ? (
            <>
              <h3 className="text-base font-semibold text-gray-900 mb-1">
                No results for &ldquo;{debouncedSearch}&rdquo;
              </h3>
              <p className="text-sm text-gray-500 max-w-xs">
                Try a different job title or company name.
              </p>
            </>
          ) : (
            <>
              <h3 className="text-base font-semibold text-gray-900 mb-1">
                No applications yet
              </h3>
              <p className="text-sm text-gray-500 max-w-xs">
                Approve job matches from the queue to start automating applications.
              </p>
            </>
          )}
        </div>
      ) : (
        <>
          <div className="space-y-3">
            {applications.map((app) => (
              <ApplicationRow key={app.id} application={app} />
            ))}
          </div>

          {hasMore && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" onClick={handleLoadMore}>
                Load more
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
