"use client";

/**
 * ApplicationsOverTime — line chart showing daily application count.
 * Uses recharts (already in package.json).
 */

import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";
import type { DailyCount } from "@/lib/types";

interface ApplicationsOverTimeProps {
  data: DailyCount[];
}

/** Format ISO date "2026-03-15" → "Mar 15" */
function fmtDate(iso: string): string {
  try {
    const d = new Date(iso + "T00:00:00");
    return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  } catch {
    return iso;
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div className="bg-white border border-gray-200 rounded-lg px-3 py-2 shadow-md text-xs">
      <p className="text-gray-500 mb-0.5">{label}</p>
      <p className="font-semibold text-gray-900">
        {payload[0].value} application{payload[0].value !== 1 ? "s" : ""}
      </p>
    </div>
  );
}

export function ApplicationsOverTime({ data }: ApplicationsOverTimeProps) {
  if (!data.length) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-gray-400">
        No data for this period.
      </div>
    );
  }

  const chartData = data.map((d) => ({
    date: fmtDate(d.date),
    count: d.count,
  }));

  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={chartData} margin={{ top: 4, right: 8, left: -24, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" vertical={false} />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 11, fill: "#9ca3af" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
        />
        <YAxis
          tick={{ fontSize: 11, fill: "#9ca3af" }}
          tickLine={false}
          axisLine={false}
          allowDecimals={false}
        />
        <Tooltip content={<CustomTooltip />} />
        <Line
          type="monotone"
          dataKey="count"
          stroke="#6366f1"
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 4, fill: "#6366f1" }}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
