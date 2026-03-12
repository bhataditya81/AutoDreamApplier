import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/** Merge Tailwind classes safely (handles conflicts). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

/** Format a salary range for display. Returns e.g. "$80k – $120k". */
export function formatSalary(
  min?: number,
  max?: number,
  currency = "USD"
): string {
  if (!min && !max) return "Salary not listed";
  const sym = currency === "USD" ? "$" : currency + " ";
  const fmt = (n: number) =>
    n >= 1000 ? `${sym}${Math.round(n / 1000)}k` : `${sym}${n}`;
  if (min && max) return `${fmt(min)} – ${fmt(max)}`;
  if (min) return `From ${fmt(min)}`;
  return `Up to ${fmt(max!)}`;
}

/** Format a UTC ISO string as a human-readable relative time (e.g. "2h ago"). */
export function timeAgo(isoDate: string): string {
  const diff = Date.now() - new Date(isoDate).getTime();
  const s = Math.floor(diff / 1000);
  if (s < 60) return "just now";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d ago`;
  return new Date(isoDate).toLocaleDateString();
}

/** Format a UTC ISO string as a local date string. */
export function formatDate(isoDate?: string): string {
  if (!isoDate) return "—";
  return new Date(isoDate).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

/** Convert an ApplicationStatus to a human-readable label. */
export function statusLabel(status: string): string {
  return {
    queued: "Queued",
    ai_preparing: "AI Preparing",
    ai_ready: "AI Ready",
    applying: "Applying",
    applied: "Applied",
    failed: "Failed",
    withdrawn: "Withdrawn",
  }[status] ?? status;
}

/** Return a match-score colour class (Tailwind). */
export function scoreColor(score: number): string {
  if (score >= 0.8) return "text-green-600";
  if (score >= 0.6) return "text-yellow-600";
  return "text-red-500";
}

/** Convert match score (0-1) to percent string. */
export function scorePercent(score: number): string {
  return `${Math.round(score * 100)}%`;
}
