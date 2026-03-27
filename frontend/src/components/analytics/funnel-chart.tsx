"use client";

/**
 * FunnelChart — horizontal bar chart showing conversion funnel:
 *   Applied → Viewed → Interview → Offer
 */

import type { FunnelStats } from "@/lib/types";

interface FunnelChartProps {
  stats: FunnelStats;
}

interface FunnelRow {
  label: string;
  count: number;
  pct: number | null; // null for top row (applied — 100% baseline)
  color: string;
}

export function FunnelChart({ stats }: FunnelChartProps) {
  const rows: FunnelRow[] = [
    {
      label: "Applied",
      count: stats.applied,
      pct: null,
      color: "bg-brand-500",
    },
    {
      label: "Viewed",
      count: stats.viewed,
      pct: stats.view_rate,
      color: "bg-blue-400",
    },
    {
      label: "Interview",
      count: stats.interviews,
      pct: stats.interview_rate,
      color: "bg-amber-400",
    },
    {
      label: "Offer",
      count: stats.offers,
      pct: stats.offer_rate,
      color: "bg-green-500",
    },
  ];

  const maxCount = Math.max(1, stats.applied);

  return (
    <div className="space-y-3">
      {rows.map((row) => (
        <div key={row.label} className="flex items-center gap-3">
          {/* Label */}
          <span className="w-20 text-right text-xs text-gray-500 shrink-0">
            {row.label}
          </span>

          {/* Bar */}
          <div className="flex-1 flex items-center gap-2">
            <div className="flex-1 h-6 bg-gray-100 rounded-md overflow-hidden">
              <div
                className={`h-full ${row.color} rounded-md transition-all duration-700`}
                style={{
                  width: `${Math.max(2, (row.count / maxCount) * 100)}%`,
                }}
              />
            </div>
          </div>

          {/* Count + rate */}
          <div className="w-28 shrink-0 flex items-center gap-1.5">
            <span className="text-sm font-semibold text-gray-900 tabular-nums">
              {row.count.toLocaleString()}
            </span>
            {row.pct !== null && (
              <span className="text-xs text-gray-400">
                ({row.pct.toFixed(1)}%)
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
