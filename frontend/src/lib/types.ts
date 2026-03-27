// ── Domain types shared across the frontend ──────────────────────────────────

export interface Job {
  id: string;
  externalId: string;
  sourceBoard: string;
  url: string;
  title: string;
  company: string;
  location: string;
  isRemote: boolean;
  salaryMin?: number;
  salaryMax?: number;
  salaryCurrency: string;
  description: string;
  atsType: string;
  applyUrl: string;
  isScam: boolean;
  postedAt: string;
  discoveredAt: string;
}

export interface MatchBreakdown {
  title: number;
  location: number;
  salary: number;
}

export type MatchStatus = "pending" | "approved" | "rejected" | "applied";

export interface Match {
  id: string;
  userId: string;
  jobId: string;
  matchScore: number;
  matchBreakdown: MatchBreakdown;
  status: MatchStatus;
  userFeedback?: "thumbs_up" | "thumbs_down";
  createdAt: string;
  updatedAt: string;
  job: Job;
}

export type ApplicationStatus =
  | "queued"
  | "ai_preparing"
  | "ai_ready"
  | "applying"
  | "applied"
  | "failed"
  | "withdrawn";

export type ApplicationOutcome =
  | "viewed"
  | "rejected"
  | "interview"
  | "offer";

export interface Application {
  id: string;
  userId: string;
  jobId: string;
  matchId: string;
  resumeId: string;
  status: ApplicationStatus;
  outcome?: ApplicationOutcome;
  tailoredResumeS3?: string;
  coverLetterS3?: string;
  screenshotS3?: string;
  errorMessage?: string;
  appliedAt?: string;
  outcomeUpdatedAt?: string;
  createdAt: string;
  job: Job;
}

export type UserTier = "free" | "starter" | "pro" | "enterprise";
export type ApplyMode = "review" | "auto";

export interface User {
  id: string;
  email: string;
  fullName: string;
  tier: UserTier;
  applyMode: ApplyMode;
  autoThreshold: number;
  dailyLimit: number;
  isActive: boolean;
  createdAt: string;
}

export type RemotePref = "remote" | "hybrid" | "onsite" | "any";

export interface UserPreferences {
  targetTitles: string[];
  locations: string[];
  remotePref: RemotePref;
  salaryMin?: number;
  salaryMax?: number;
  salaryCurrency: string;
  exclusions: string[];
  ai_tailor_enabled?: boolean;
  // Auto-apply scheduling
  autoApplyEnabled?: boolean;
  dailyApplicationLimit?: number;
  applyStartHour?: number;
  applyEndHour?: number;
  applyTimezone?: string;
  // Notification settings
  slackWebhookUrl?: string;
  discordWebhookUrl?: string;
  webhookEvents?: string[];
  emailDigestEnabled?: boolean;
}

export interface Resume {
  id: string;
  userId: string;
  fileName: string;
  s3Key: string;
  isPrimary: boolean;
  interviewCount: number;
  // A/B testing fields
  ab_enabled: boolean;
  ab_weight: number;
  total_applications: number;
  total_views: number;
  total_interviews: number;
  total_offers: number;
  interview_rate: number; // computed: total_interviews / total_applications * 100
  createdAt: string;
}

// ── Analytics ─────────────────────────────────────────────────────────────────

export interface FunnelStats {
  applied: number;
  viewed: number;
  interviews: number;
  offers: number;
  view_rate: number;      // percent
  interview_rate: number; // percent
  offer_rate: number;     // percent
}

export interface DailyCount {
  date: string;   // YYYY-MM-DD
  count: number;
}

export interface ResumeStats {
  resume_id: string;
  file_name: string;
  total_applications: number;
  total_interviews: number;
  total_offers: number;
  interview_rate: number;
}

export interface CompanyStat {
  company: string;
  count: number;
}

// ── Pagination ────────────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
}

// ── API error ─────────────────────────────────────────────────────────────────

export interface ApiError {
  error: string;
  message: string;
  statusCode: number;
}

// ── Salary benchmarking ───────────────────────────────────────────────────────

export interface SalaryBenchmark {
  title_key: string;
  location_key: string;
  currency: string;
  min: number;
  p25: number;
  median: number;
  p75: number;
  max: number;
  sample_size: number;
  updated_at: string;
}

export type MarketPosition = 'above' | 'at' | 'below' | 'unknown';

export interface SalaryBenchmarkResponse {
  benchmark: SalaryBenchmark | null;
  job_salary_min?: number;
  job_salary_max?: number;
  market_position: MarketPosition;
}
