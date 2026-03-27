"use client";

import { useCallback, useEffect, useState } from "react";
import { Layers, Calendar, Inbox, TrendingUp, Zap } from "lucide-react";
import { Header } from "@/components/layout/header";
import {
  getDashboardStats,
  listApplications,
  getPreferences,
  type DashboardStats,
} from "@/lib/api";
import type { Application, UserPreferences } from "@/lib/types";
import { Button } from "@/components/ui/button";

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Returns the last 7 days as { label, count } using createdAt dates. */
function buildWeeklyActivity(applications: Application[]) {
  const days: { label: string; date: string; count: number }[] = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    const isoDate = d.toISOString().slice(0, 10); // YYYY-MM-DD
    const label = d.toLocaleDateString("en-US", { weekday: "short" });
    days.push({ label, date: isoDate, count: 0 });
  }

  for (const app of applications) {
    const appDate = app.createdAt?.slice(0, 10);
    const day = days.find((d) => d.date === appDate);
    if (day) day.count++;
  }

  return days;
}

function statusLabel(status: Application["status"]): string {
  const map: Record<Application["status"], string> = {
    queued: "Queued",
    ai_preparing: "AI Preparing",
    ai_ready: "AI Ready",
    applying: "Applying",
    applied: "Applied",
    failed: "Failed",
    withdrawn: "Withdrawn",
  };
  return map[status] ?? status;
}

function statusColor(status: Application["status"]): string {
  switch (status) {
    case "applied":
      return "bg-green-100 text-green-700";
    case "failed":
      return "bg-red-100 text-red-700";
    case "withdrawn":
      return "bg-gray-100 text-gray-600";
    case "ai_preparing":
    case "ai_ready":
    case "applying":
      return "bg-blue-100 text-blue-700";
    default:
      return "bg-yellow-100 text-yellow-700";
  }
}

// ── Skeleton ──────────────────────────────────────────────────────────────────

function StatCardSkeleton() {
  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5 animate-pulse">
      <div className="flex items-center justify-between mb-3">
        <div className="h-3 w-24 bg-gray-200 rounded" />
        <div className="h-4 w-4 bg-gray-200 rounded" />
      </div>
      <div className="h-9 w-16 bg-gray-200 rounded mb-2" />
      <div className="h-3 w-16 bg-gray-100 rounded" />
    </div>
  );
}

// ── Stat card ─────────────────────────────────────────────────────────────────

interface StatCardProps {
  label: string;
  value: string | number;
  sub: string;
  icon: React.ReactNode;
}

function StatCard({ label, value, sub, icon }: StatCardProps) {
  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-gray-500">{label}</span>
        <span className="text-gray-400">{icon}</span>
      </div>
      <p className="text-3xl font-bold text-gray-900">{value}</p>
      <p className="text-xs text-gray-400 mt-1">{sub}</p>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

interface PageData {
  stats: DashboardStats;
  recentApps: Application[];
  prefs: UserPreferences;
}

export default function OverviewPage() {
  const [data, setData] = useState<PageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [stats, appsResp, prefs] = await Promise.all([
        getDashboardStats(),
        listApplications({ limit: 5, page: 1 }),
        getPreferences(),
      ]);
      setData({ stats, recentApps: appsResp.data, prefs });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load dashboard");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // ── Error state ──────────────────────────────────────────────────────────────
  if (error) {
    return (
      <div className="flex flex-col h-full">
        <Header title="Overview" subtitle="Your job search at a glance." />
        <div className="flex-1 px-6 pb-8 flex items-center justify-center">
          <div className="bg-red-50 border border-red-200 rounded-2xl p-6 text-center max-w-sm w-full">
            <p className="text-sm font-medium text-red-700 mb-1">Failed to load overview</p>
            <p className="text-xs text-red-500 mb-4">{error}</p>
            <Button variant="outline" size="sm" onClick={load}>
              Retry
            </Button>
          </div>
        </div>
      </div>
    );
  }

  // ── Loading skeleton ─────────────────────────────────────────────────────────
  if (loading || !data) {
    return (
      <div className="flex flex-col h-full">
        <Header title="Overview" subtitle="Your job search at a glance." />
        <div className="flex-1 px-6 pb-8 pt-6 space-y-6">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
            {[0, 1, 2, 3, 4].map((i) => (
              <StatCardSkeleton key={i} />
            ))}
          </div>
          <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5 animate-pulse">
            <div className="h-4 w-32 bg-gray-200 rounded mb-4" />
            <div className="flex items-end gap-2 h-24">
              {[0, 1, 2, 3, 4, 5, 6].map((i) => (
                <div key={i} className="flex flex-col items-center gap-1 flex-1">
                  <div className="w-full bg-gray-200 rounded-t" style={{ height: "40px" }} />
                  <div className="h-3 w-6 bg-gray-100 rounded" />
                </div>
              ))}
            </div>
          </div>
          <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5 animate-pulse space-y-3">
            <div className="h-4 w-40 bg-gray-200 rounded" />
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="flex items-center justify-between py-2">
                <div className="space-y-1">
                  <div className="h-3 w-48 bg-gray-200 rounded" />
                  <div className="h-3 w-32 bg-gray-100 rounded" />
                </div>
                <div className="h-5 w-16 bg-gray-100 rounded-full" />
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  const { stats, recentApps, prefs } = data;
  const successRatePct = Math.round(stats.successRate * 100);
  const autoApplyOn = prefs.autoApplyEnabled ?? false;
  const weeklyDays = buildWeeklyActivity(recentApps);
  const maxCount = Math.max(1, ...weeklyDays.map((d) => d.count));

  return (
    <div className="flex flex-col h-full">
      <Header
        title="Overview"
        subtitle="Your job search at a glance."
        actions={
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${
              autoApplyOn
                ? "bg-green-100 text-green-700"
                : "bg-gray-100 text-gray-500"
            }`}
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                autoApplyOn ? "bg-green-500" : "bg-gray-400"
              }`}
            />
            {autoApplyOn ? "Auto-Apply Active" : "Manual Mode"}
          </span>
        }
      />

      <div className="flex-1 px-6 pb-8 pt-6 space-y-6">
        {/* ── Stat cards ── */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
          <StatCard
            label="Total Applied"
            value={stats.applicationsThisWeek}
            sub="This week"
            icon={<Layers className="h-4 w-4" />}
          />
          <StatCard
            label="Interviews"
            value={stats.interviewsThisMonth}
            sub="This month"
            icon={<Calendar className="h-4 w-4" />}
          />
          <StatCard
            label="Pending Matches"
            value={stats.pendingMatches}
            sub="Awaiting review"
            icon={<Inbox className="h-4 w-4" />}
          />
          <StatCard
            label="Success Rate"
            value={`${successRatePct}%`}
            sub="Interviews / applications"
            icon={<TrendingUp className="h-4 w-4" />}
          />
          {/* Daily rate limit card */}
          <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-gray-500">Today&apos;s Applications</span>
              <Zap className="h-4 w-4 text-gray-400" />
            </div>
            <p className="text-3xl font-bold text-gray-900">{stats.appliedToday}</p>
            <p className="text-xs text-gray-400 mt-1">of {stats.dailyLimit} limit</p>
            {/* Progress bar */}
            <div className="mt-3 h-1.5 bg-gray-100 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${
                  stats.appliedToday >= stats.dailyLimit
                    ? "bg-red-500"
                    : stats.appliedToday / stats.dailyLimit > 0.8
                    ? "bg-amber-400"
                    : "bg-brand-500"
                }`}
                style={{
                  width: `${Math.min(100, (stats.appliedToday / Math.max(1, stats.dailyLimit)) * 100)}%`,
                }}
              />
            </div>
          </div>
        </div>

        {/* ── Weekly activity bar chart ── */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
          <h2 className="text-sm font-semibold text-gray-900 mb-4">Weekly Activity</h2>
          <div className="flex items-end gap-2 h-24">
            {weeklyDays.map((d) => (
              <div key={d.date} className="flex flex-col items-center gap-1 flex-1">
                <div
                  className="w-full bg-brand-500 rounded-t transition-all"
                  style={{ height: `${Math.max(4, (d.count / maxCount) * 80)}px` }}
                />
                <span className="text-xs text-gray-400">{d.label}</span>
              </div>
            ))}
          </div>
          <p className="text-xs text-gray-400 mt-2">Applications submitted per day (last 7 days)</p>
        </div>

        {/* ── Recent applications ── */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
          <h2 className="text-sm font-semibold text-gray-900 mb-3">Recent Applications</h2>
          {recentApps.length === 0 ? (
            <p className="text-sm text-gray-400 py-4 text-center">No applications yet.</p>
          ) : (
            <ul className="divide-y divide-gray-50">
              {recentApps.map((app) => (
                <li key={app.id} className="flex items-center justify-between py-2.5">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-gray-900 truncate">
                      {app.job?.title ?? "Unknown Role"}
                    </p>
                    <p className="text-xs text-gray-400 truncate">
                      {app.job?.company ?? "—"}{" "}
                      {app.job?.location ? `· ${app.job.location}` : ""}
                    </p>
                  </div>
                  <span
                    className={`ml-4 shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${statusColor(
                      app.status
                    )}`}
                  >
                    {statusLabel(app.status)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}
