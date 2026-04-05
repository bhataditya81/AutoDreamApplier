/**
 * Typed API client.
 *
 * All requests are proxied through Next.js's rewrite rule:
 *   /api/* -> NEXT_PUBLIC_API_URL/api/*
 *
 * The Go api-gateway expects  Authorization: Bearer <jwt>  on protected routes.
 */

import { getToken } from "@/lib/auth";
import type {
  Application,
  CompanyStat,
  DailyCount,
  FunnelStats,
  Match,
  MatchStatus,
  PaginatedResponse,
  Resume,
  ResumeStats,
  SalaryBenchmarkResponse,
  User,
  UserPreferences,
} from "@/lib/types";

// ── Low-level fetch wrapper ───────────────────────────────────────────────────

interface FetchOptions extends RequestInit {
  skipAuth?: boolean;
}

async function apiFetch<T>(path: string, opts: FetchOptions = {}): Promise<T> {
  const { skipAuth, ...rest } = opts;

  const headers: Record<string, string> = {
    ...(rest.headers as Record<string, string> | undefined),
  };

  if (!skipAuth) {
    const token = getToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
  }

  // Don't set Content-Type for FormData (browser sets it with boundary).
  if (!(rest.body instanceof FormData)) {
    headers["Content-Type"] = headers["Content-Type"] ?? "application/json";
  }

  const res = await fetch(`/api/v1${path}`, { ...rest, headers });

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as {
        message?: string;
        error?: string | { message?: string };
      };
      const errField = body.error;
      const msg =
        body.message ??
        (typeof errField === "string" ? errField : errField?.message);
      if (typeof msg === "string" && msg) message = msg;
    } catch {
      // use status text
    }
    throw new Error(message);
  }

  // 204 No Content
  if (res.status === 204) return undefined as unknown as T;
  return res.json() as Promise<T>;
}

// ── Matches ───────────────────────────────────────────────────────────────────

export interface MatchStatusUpdateResponse {
  id: string;
  status: MatchStatus;
}

export async function listMatches(
  params?: Record<string, string | number>
): Promise<PaginatedResponse<Match>> {
  const qs = new URLSearchParams();
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      qs.set(k, String(v));
    }
  }
  const query = qs.toString() ? `?${qs}` : "";
  return apiFetch<PaginatedResponse<Match>>(`/matches${query}`);
}

export async function updateMatchStatus(
  matchId: string,
  status: "approved" | "rejected",
  feedback?: "thumbs_up" | "thumbs_down"
): Promise<MatchStatusUpdateResponse> {
  return apiFetch<MatchStatusUpdateResponse>(`/matches/${matchId}`, {
    method: "PATCH",
    body: JSON.stringify({ status, userFeedback: feedback }),
  });
}

export async function bulkUpdateMatches(
  matchIds: string[],
  action: "approve" | "reject"
): Promise<void> {
  await apiFetch<{ action: string; updated: number }>("/matches/bulk-action", {
    method: "POST",
    body: JSON.stringify({ action, match_ids: matchIds }),
  });
}

export async function setMatchFeedback(
  matchId: string,
  feedback: "thumbs_up" | "thumbs_down"
): Promise<void> {
  await apiFetch<{ feedback: string }>(`/matches/${matchId}/feedback`, {
    method: "PATCH",
    body: JSON.stringify({ feedback }),
  });
}

// ── Applications ──────────────────────────────────────────────────────────────

export async function listApplications(
  params?: Record<string, string | number>
): Promise<PaginatedResponse<Application>> {
  const qs = new URLSearchParams();
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      qs.set(k, String(v));
    }
  }
  const query = qs.toString() ? `?${qs}` : "";
  return apiFetch<PaginatedResponse<Application>>(`/applications${query}`);
}

export async function getApplication(id: string): Promise<Application> {
  return apiFetch<Application>(`/applications/${id}`);
}

export async function withdrawApplication(applicationId: string): Promise<void> {
  await apiFetch<void>(`/applications/${applicationId}`, {
    method: "PATCH",
    body: JSON.stringify({ status: "withdrawn" }),
  });
}

export async function updateApplicationOutcome(
  applicationId: string,
  outcome: import("@/lib/types").ApplicationOutcome,
  notes?: string
): Promise<void> {
  await apiFetch<void>(`/applications/${applicationId}/outcome`, {
    method: "PATCH",
    body: JSON.stringify({ outcome, notes }),
  });
}

// ── Resumes ───────────────────────────────────────────────────────────────────

export async function listResumes(): Promise<Resume[]> {
  return apiFetch<Resume[]>("/users/me/resumes");
}

export async function uploadResume(file: File): Promise<Resume> {
  const fd = new FormData();
  fd.append("file", file);
  return apiFetch<Resume>("/users/me/resumes", { method: "POST", body: fd });
}

export async function setPrimaryResume(resumeId: string): Promise<Resume> {
  return apiFetch<Resume>(`/users/me/resumes/${resumeId}/primary`, { method: "PUT" });
}

export async function deleteResume(resumeId: string): Promise<void> {
  return apiFetch<void>(`/users/me/resumes/${resumeId}`, { method: "DELETE" });
}

export async function updateResumeAB(
  resumeId: string,
  abEnabled: boolean,
  abWeight: number
): Promise<Resume> {
  return apiFetch<Resume>(`/users/me/resumes/${resumeId}/ab-test`, {
    method: "PUT",
    body: JSON.stringify({ abEnabled, abWeight }),
  });
}

// ── User ──────────────────────────────────────────────────────────────────────

export async function getMe(): Promise<User> {
  return apiFetch<User>("/users/me");
}

export async function updateMe(data: Partial<User>): Promise<User> {
  return apiFetch<User>("/users/me", {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

// ── Preferences ───────────────────────────────────────────────────────────────

export async function getPreferences(): Promise<UserPreferences> {
  return apiFetch<UserPreferences>("/users/me/preferences");
}

export async function savePreferences(prefs: UserPreferences): Promise<UserPreferences> {
  return apiFetch<UserPreferences>("/users/me/preferences", {
    method: "PUT",
    body: JSON.stringify(prefs),
  });
}

// ── Board credentials ─────────────────────────────────────────────────────────

export async function saveBoardCredentials(
  boardName: string,
  email: string,
  password: string
): Promise<void> {
  return apiFetch<void>("/users/me/credentials", {
    method: "POST",
    body: JSON.stringify({ boardName, email, password }),
  });
}

// ── Analytics ─────────────────────────────────────────────────────────────────

export interface AnalyticsRange { from: string; to: string } // YYYY-MM-DD

export async function getAnalyticsFunnel(range: AnalyticsRange): Promise<FunnelStats> {
  return apiFetch<FunnelStats>(`/analytics/funnel?from=${range.from}&to=${range.to}`);
}

export async function getAnalyticsOverTime(range: AnalyticsRange): Promise<DailyCount[]> {
  return apiFetch<DailyCount[]>(`/analytics/over-time?from=${range.from}&to=${range.to}`);
}

export async function getAnalyticsByResume(): Promise<ResumeStats[]> {
  return apiFetch<ResumeStats[]>("/analytics/by-resume");
}

export async function getAnalyticsTopCompanies(limit = 10): Promise<CompanyStat[]> {
  return apiFetch<CompanyStat[]>(`/analytics/top-companies?limit=${limit}`);
}

// ── Dashboard stats ───────────────────────────────────────────────────────────

export interface DashboardStats {
  pendingMatches: number;
  applicationsThisWeek: number;
  interviewsThisMonth: number;
  successRate: number; // 0-1
  appliedToday: number;
  dailyLimit: number;
  remainingToday: number; // dailyLimit - appliedToday, clamped to 0
}

// The backend returns a full UserDashboard object; we extract the stats field.
interface UserDashboard {
  user: {
    daily_limit: number;
  } | null;
  stats: {
    total_applications: number;
    applied_today: number;
    pending_matches: number;
    interviews_received: number;
  } | null;
}

// ── Salary benchmarking ───────────────────────────────────────────────────────

export async function getSalaryBenchmark(
  title: string,
  location: string,
  salaryMin?: number,
  salaryMax?: number,
): Promise<SalaryBenchmarkResponse> {
  const qs = new URLSearchParams({ title, location });
  if (salaryMin !== undefined) qs.set('salary_min', String(salaryMin));
  if (salaryMax !== undefined) qs.set('salary_max', String(salaryMax));

  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`/api/v1/salary/benchmark?${qs}`, { headers });

  if (res.status === 404) {
    return { benchmark: null, market_position: 'unknown' };
  }

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { message?: unknown; error?: unknown };
      const msg = body.message ?? body.error;
      if (typeof msg === "string" && msg) message = msg;
    } catch {
      // use status text
    }
    throw new Error(message);
  }

  return res.json() as Promise<SalaryBenchmarkResponse>;
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const dashboard = await apiFetch<UserDashboard>("/users/me/dashboard");
  const s = dashboard.stats;
  const appliedToday = s?.applied_today ?? 0;
  const dailyLimit = dashboard.user?.daily_limit ?? 5;
  return {
    pendingMatches: s?.pending_matches ?? 0,
    applicationsThisWeek: s?.applied_today ?? 0,
    interviewsThisMonth: s?.interviews_received ?? 0,
    successRate: 0,
    appliedToday,
    dailyLimit,
    remainingToday: Math.max(0, dailyLimit - appliedToday),
  };
}
