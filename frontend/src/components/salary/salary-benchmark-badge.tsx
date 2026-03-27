"use client";

import { useCallback, useEffect, useState } from "react";
import { getSalaryBenchmark } from "@/lib/api";
import type { SalaryBenchmarkResponse, MarketPosition } from "@/lib/types";

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Formats a number to a compact dollar string: 125000 → "$125k", 1200000 → "$1.2M" */
function formatK(value: number): string {
  if (value >= 1_000_000) {
    const m = value / 1_000_000;
    return `$${m % 1 === 0 ? m.toFixed(0) : m.toFixed(1)}M`;
  }
  const k = value / 1_000;
  return `$${k % 1 === 0 ? k.toFixed(0) : k.toFixed(1)}k`;
}

// ── Position config ───────────────────────────────────────────────────────────

interface PositionConfig {
  containerClass: string;
  arrow: string;
  label: string;
  pillClass: string;
}

const positionConfig: Record<Exclude<MarketPosition, 'unknown'>, PositionConfig> = {
  above: {
    containerClass: 'bg-green-50 border border-green-100 text-green-800',
    arrow: '↑',
    label: 'Above market',
    pillClass: 'text-green-700 bg-green-100',
  },
  at: {
    containerClass: 'bg-blue-50 border border-blue-100 text-blue-800',
    arrow: '→',
    label: 'At market',
    pillClass: 'text-blue-700 bg-blue-100',
  },
  below: {
    containerClass: 'bg-amber-50 border border-amber-100 text-amber-800',
    arrow: '↓',
    label: 'Below market',
    pillClass: 'text-amber-700 bg-amber-100',
  },
};

// ── Component ─────────────────────────────────────────────────────────────────

interface SalaryBenchmarkBadgeProps {
  title: string;
  location: string;
  salaryMin?: number;
  salaryMax?: number;
}

export function SalaryBenchmarkBadge({
  title,
  location,
  salaryMin,
  salaryMax,
}: SalaryBenchmarkBadgeProps) {
  const [data, setData] = useState<SalaryBenchmarkResponse | null>(null);

  const load = useCallback(async () => {
    try {
      const result = await getSalaryBenchmark(title, location, salaryMin, salaryMax);
      setData(result);
    } catch {
      // Silently swallow — render nothing on error
    }
  }, [title, location, salaryMin, salaryMax]);

  useEffect(() => {
    load();
  }, [load]);

  // Render nothing while loading, on error, or when no benchmark data
  if (!data || !data.benchmark || data.market_position === 'unknown') {
    return null;
  }

  const { benchmark, market_position } = data;
  const config = positionConfig[market_position as Exclude<MarketPosition, 'unknown'>];

  // Should not happen, but guard for type safety
  if (!config) return null;

  return (
    <div className={`mt-2 rounded-lg px-3 py-2 text-xs ${config.containerClass}`}>
      <p>
        💰 Market: {formatK(benchmark.p25)} – {formatK(benchmark.p75)} median · {benchmark.sample_size} jobs
      </p>
      <p className="mt-1">
        <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium ${config.pillClass}`}>
          {config.arrow} {config.label}
        </span>
      </p>
    </div>
  );
}
