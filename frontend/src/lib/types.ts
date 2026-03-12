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
}

export interface Resume {
  id: string;
  userId: string;
  fileName: string;
  s3Key: string;
  isPrimary: boolean;
  interviewCount: number;
  createdAt: string;
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
