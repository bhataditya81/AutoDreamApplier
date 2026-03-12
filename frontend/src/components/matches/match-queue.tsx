"use client";

import { useState, useEffect, useCallback } from "react";
import { RefreshCw, Inbox } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Alert } from "@/components/ui/alert";
import { MatchCard } from "./match-card";
import { listMatches } from "@/lib/api";
import type { Match, MatchStatus } from "@/lib/types";

const STATUS_TABS: { label: string; value: MatchStatus | "all" }[] = [
  { label: "Pending", value: "pending" },
  { label: "Approved", value: "approved" },
  { label: "Rejected", value: "rejected" },
  { label: "All", value: "all" },
];

export function MatchQueue() {
  const [matches, setMatches] = useState<Match[]>([]);
  const [status, setStatus] = useState<MatchStatus | "all">("pending");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const fetchMatches = useCallback(
    async (pageNum: number, replace: boolean) => {
      try {
        setError(null);
        const params: Record<string, string | number> = { page: pageNum, limit: 12 };
        if (status !== "all") params.status = status;
        const res = await listMatches(params);
        setMatches((prev) => (replace ? res.data : [...prev, ...res.data]));
        setHasMore(res.hasMore);
      } catch (e: unknown) {
        console.error("[MatchQueue] fetchMatches error:", e);
        setError(e instanceof Error ? e.message : "Failed to load matches");
      }
    },
    [status]
  );

  // Reset & reload when status tab changes
  useEffect(() => {
    setPage(1);
    setMatches([]);
    setLoading(true);
    fetchMatches(1, true).finally(() => setLoading(false));
  }, [fetchMatches]);

  async function handleRefresh() {
    setRefreshing(true);
    setPage(1);
    await fetchMatches(1, true);
    setRefreshing(false);
  }

  async function handleLoadMore() {
    const next = page + 1;
    setPage(next);
    await fetchMatches(next, false);
  }

  function handleUpdate(updated: Match) {
    setMatches((prev) =>
      prev.map((m) => (m.id === updated.id ? updated : m))
    );
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
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

        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={refreshing}
        >
          <RefreshCw className={`h-3.5 w-3.5 ${refreshing ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {/* Error state */}
      {error && (
        <Alert variant="error" title="Failed to load matches">
          {error}
        </Alert>
      )}

      {/* Loading state */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Spinner size="lg" />
        </div>
      ) : matches.length === 0 ? (
        /* Empty state */
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mb-4">
            <Inbox className="h-8 w-8 text-gray-400" />
          </div>
          <h3 className="text-base font-semibold text-gray-900 mb-1">
            No {status === "all" ? "" : status} matches
          </h3>
          <p className="text-sm text-gray-500 max-w-xs">
            {status === "pending"
              ? "New job matches will appear here as our discovery engine finds them."
              : `No matches with status "${status}" yet.`}
          </p>
        </div>
      ) : (
        <>
          {/* Match grid */}
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {matches.map((match) => (
              <MatchCard key={match.id} match={match} onUpdate={handleUpdate} />
            ))}
          </div>

          {/* Load more */}
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
