"use client";

/**
 * Analytics Page — /dashboard/analytics
 *
 * Shows:
 *   • Date-range toggle (30 / 90 / 180 days)
 *   • Summary stat row (Applied, Interviews, Offers, Success Rate)
 *   • Applications Over Time line chart
 *   • Conversion Funnel horizontal bars
 *   • Top Companies applied to (horizontal bars)
 *   • Outstanding outcome prompts for applications >7 days old with no outcome
 *
 * URL state: ?range=30|90|180  (preserved on back/forward)
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { BarChart2, TrendingUp, Award, Briefcase } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import { FunnelChart } from "@/components/analytics/funnel-chart";
import { ApplicationsOverTime } from "@/components/analytics/applications-over-time";
import { OutcomeEntryModal } from "@/components/analytics/outcome-entry-modal";
import {
  getAnalyticsFunnel,
  getAnalyticsOverTime,
  getAnalyticsTopCompanies,
  listApplications,
  type AnalyticsRange,
} from "@/lib/api";
import type {
  Application,
  CompanyStat,
  DailyCount,
  FunnelStats,
} from "@/lib/types";

// ── Motion variants ────────────────────────────────────────────────────────────

const fadeSlideUp = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] } },
};

const staggerContainer = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.06, delayChildren: 0.08 } },
};

// ── Constants ─────────────────────────────────────────────────────────────────

type RangeDays = 30 | 90 | 180;
const RANGE_OPTIONS: RangeDays[] = [30, 90, 180];

function toRange(days: RangeDays): AnalyticsRange {
  const to = new Date();
  const from = new Date();
  from.setDate(from.getDate() - days);
  return {
    from: from.toISOString().slice(0, 10),
    to: to.toISOString().slice(0, 10),
  };
}

function findPendingOutcomes(apps: Application[]): Application[] {
  const threshold = Date.now() - 7 * 24 * 60 * 60 * 1000;
  return apps.filter((a) => {
    if (a.status !== "applied") return false;
    if (a.outcome) return false;
    const appliedMs = a.appliedAt ? new Date(a.appliedAt).getTime() : new Date(a.createdAt).getTime();
    return appliedMs < threshold;
  });
}

// ── Skeleton helpers ──────────────────────────────────────────────────────────

function CardSkeleton({ h = "h-48" }: { h?: string }) {
  return (
    <div className={`bg-white rounded-2xl border border-gray-100 shadow-sm p-5 animate-pulse ${h}`} />
  );
}

// ── Stat card ─────────────────────────────────────────────────────────────────

function StatCard({
  label,
  value,
  sub,
  icon,
}: {
  label: string;
  value: string | number;
  sub: string;
  icon: React.ReactNode;
}) {
  return (
    <motion.div
      variants={fadeSlideUp}
      whileHover={{ y: -3, boxShadow: "0 8px 24px rgba(99,102,241,.12)" }}
      transition={{ type: "spring", stiffness: 400, damping: 30 }}
      className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5"
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-gray-500">{label}</span>
        <span className="text-gray-400">{icon}</span>
      </div>
      <p className="text-3xl font-bold text-gray-900">{value}</p>
      <p className="text-xs text-gray-400 mt-1">{sub}</p>
    </motion.div>
  );
}

// ── Top-companies chart ───────────────────────────────────────────────────────

function TopCompaniesChart({ data }: { data: CompanyStat[] }) {
  const max = Math.max(1, ...data.map((d) => d.count));
  return (
    <div className="space-y-2.5">
      {data.map((row, i) => (
        <div key={row.company} className="flex items-center gap-3">
          <span className="w-36 truncate text-right text-xs text-gray-500 shrink-0">
            {row.company}
          </span>
          <div className="flex-1 h-5 bg-gray-100 rounded overflow-hidden">
            <motion.div
              initial={{ width: "0%" }}
              animate={{ width: `${Math.max(4, (row.count / max) * 100)}%` }}
              transition={{ delay: i * 0.1, duration: 0.6, ease: [0.4, 0, 0.2, 1] }}
              className="h-full bg-indigo-400 rounded"
            />
          </div>
          <span className="w-6 text-xs font-semibold text-gray-700 tabular-nums shrink-0">
            {row.count}
          </span>
        </div>
      ))}
    </div>
  );
}

// ── Page data ─────────────────────────────────────────────────────────────────

interface PageData {
  funnel: FunnelStats;
  overTime: DailyCount[];
  companies: CompanyStat[];
  pendingOutcomes: Application[];
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function AnalyticsPage() {
  const searchParams = useSearchParams();
  const router = useRouter();

  const rawRange = Number(searchParams.get("range"));
  const initialRange: RangeDays = (RANGE_OPTIONS as number[]).includes(rawRange)
    ? (rawRange as RangeDays)
    : 30;
  const [rangeDays, setRangeDays] = useState<RangeDays>(initialRange);
  const [data, setData] = useState<PageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [modalApp, setModalApp] = useState<Application | null>(null);
  const [modalIndex, setModalIndex] = useState(0);
  const pendingRef = useRef<Application[]>([]);

  const load = useCallback(async (days: RangeDays) => {
    setLoading(true);
    setError(null);
    try {
      const range = toRange(days);
      const [funnel, overTime, companies, appsResp] = await Promise.all([
        getAnalyticsFunnel(range),
        getAnalyticsOverTime(range),
        getAnalyticsTopCompanies(10),
        listApplications({ limit: 100, page: 1 }),
      ]);
      const pendingOutcomes = findPendingOutcomes(appsResp.data);
      pendingRef.current = pendingOutcomes;
      setData({ funnel, overTime, companies, pendingOutcomes });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load analytics");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(rangeDays);
  }, [load, rangeDays]);

  function handleRangeChange(days: RangeDays) {
    setRangeDays(days);
    const params = new URLSearchParams(searchParams.toString());
    params.set("range", String(days));
    router.replace(`?${params.toString()}`);
  }

  function startOutcomeFlow() {
    if (!data?.pendingOutcomes.length) return;
    setModalIndex(0);
    setModalApp(data.pendingOutcomes[0]);
  }

  function handleOutcomeClose() {
    setModalApp(null);
    const next = modalIndex + 1;
    if (data && next < data.pendingOutcomes.length) {
      setModalIndex(next);
      setModalApp(data.pendingOutcomes[next]);
    }
  }

  function handleOutcomeSaved() {
    load(rangeDays);
  }

  function isEmpty(): boolean {
    return !!data && data.funnel.applied === 0;
  }

  if (error) {
    return (
      <div className="flex flex-col h-full">
        <Header title="Analytics" subtitle="Track your application funnel and outcomes." />
        <div className="flex-1 px-6 pb-8 flex items-center justify-center">
          <div className="bg-red-50 border border-red-200 rounded-2xl p-6 text-center max-w-sm w-full">
            <p className="text-sm font-medium text-red-700 mb-1">Failed to load analytics</p>
            <p className="text-xs text-red-500 mb-4">{error}</p>
            <Button variant="outline" size="sm" onClick={() => load(rangeDays)}>Retry</Button>
          </div>
        </div>
      </div>
    );
  }

  // Range toggle with layoutId sliding pill
  const rangeToggle = (
    <div className="relative flex bg-gray-100 rounded-lg p-1">
      {RANGE_OPTIONS.map((d) => (
        <button
          key={d}
          onClick={() => handleRangeChange(d)}
          className="relative px-3 py-1.5 text-xs font-medium z-10 transition-colors"
          style={{ color: rangeDays === d ? '#111827' : '#6b7280' }}
        >
          {rangeDays === d && (
            <motion.div
              layoutId="range-pill"
              className="absolute inset-0 bg-white rounded-md shadow-sm"
              transition={{ type: "spring", stiffness: 380, damping: 30 }}
            />
          )}
          <span className="relative">{d}d</span>
        </button>
      ))}
    </div>
  );

  if (loading || !data) {
    return (
      <div className="flex flex-col h-full">
        <Header
          title="Analytics"
          subtitle="Track your application funnel and outcomes."
          actions={rangeToggle}
        />
        <div className="flex-1 px-6 pb-8 pt-6 space-y-6">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {[0, 1, 2, 3].map((i) => <CardSkeleton key={i} h="h-28" />)}
          </div>
          <CardSkeleton h="h-64" />
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <CardSkeleton h="h-52" />
            <CardSkeleton h="h-52" />
          </div>
        </div>
      </div>
    );
  }

  const { funnel, overTime, companies, pendingOutcomes } = data;

  if (isEmpty()) {
    return (
      <div className="flex flex-col h-full">
        <Header
          title="Analytics"
          subtitle="Track your application funnel and outcomes."
          actions={rangeToggle}
        />
        <div className="flex-1 px-6 pb-8 flex items-center justify-center">
          <div className="text-center max-w-sm">
            <BarChart2 className="h-12 w-12 text-gray-300 mx-auto mb-4" />
            <h2 className="text-base font-semibold text-gray-900 mb-2">No data yet</h2>
            <p className="text-sm text-gray-500 mb-5">
              Apply to some jobs first to see your analytics here.
            </p>
            <Button variant="default" size="sm" onClick={() => router.push("/dashboard/matches")}>
              Go to Match Queue
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const interviewRate = funnel.applied > 0
    ? ((funnel.interviews / funnel.applied) * 100).toFixed(1)
    : "0.0";

  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="flex flex-col h-full"
    >
      <Header
        title="Analytics"
        subtitle="Track your application funnel and outcomes."
        actions={
          <div className="flex items-center gap-3">
            <AnimatePresence>
              {pendingOutcomes.length > 0 && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: "auto", opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  className="overflow-hidden"
                >
                  <button
                    onClick={startOutcomeFlow}
                    className="inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-200 transition-colors"
                  >
                    <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
                    {pendingOutcomes.length} outcome{pendingOutcomes.length !== 1 ? "s" : ""} to record
                  </button>
                </motion.div>
              )}
            </AnimatePresence>
            {rangeToggle}
          </div>
        }
      />

      <div className="flex-1 px-6 pb-8 pt-6 space-y-6">
        {/* Stat cards */}
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4"
        >
          <StatCard
            label="Applications"
            value={funnel.applied.toLocaleString()}
            sub={`Last ${rangeDays} days`}
            icon={<Briefcase className="h-4 w-4" />}
          />
          <StatCard
            label="Interviews"
            value={funnel.interviews.toLocaleString()}
            sub={`${funnel.interview_rate.toFixed(1)}% interview rate`}
            icon={<TrendingUp className="h-4 w-4" />}
          />
          <StatCard
            label="Offers"
            value={funnel.offers.toLocaleString()}
            sub={`${funnel.offer_rate.toFixed(1)}% offer rate`}
            icon={<Award className="h-4 w-4" />}
          />
          <StatCard
            label="Success Rate"
            value={`${interviewRate}%`}
            sub="Interviews per application"
            icon={<BarChart2 className="h-4 w-4" />}
          />
        </motion.div>

        {/* Applications over time */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
          <h2 className="text-sm font-semibold text-gray-900 mb-4">Applications Over Time</h2>
          <ApplicationsOverTime data={overTime} />
          <p className="text-xs text-gray-400 mt-3">Daily applications submitted — last {rangeDays} days</p>
        </div>

        {/* Two-column: funnel + top companies */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Conversion Funnel</h2>
            <FunnelChart stats={funnel} />
            <p className="text-xs text-gray-400 mt-4">
              Tap &quot;record outcome&quot; on each application to keep this accurate.
            </p>
          </div>

          <div className="bg-white rounded-2xl border border-gray-100 shadow-sm p-5">
            <h2 className="text-sm font-semibold text-gray-900 mb-4">Top Companies Applied</h2>
            {companies.length === 0 ? (
              <p className="text-sm text-gray-400 py-6 text-center">No data yet.</p>
            ) : (
              <TopCompaniesChart data={companies} />
            )}
          </div>
        </div>
      </div>

      {modalApp && (
        <OutcomeEntryModal
          application={modalApp}
          open={!!modalApp}
          onClose={handleOutcomeClose}
          onSaved={handleOutcomeSaved}
        />
      )}
    </motion.div>
  );
}
