/**
 * Auth helpers.
 *
 * In production this integrates with AWS Cognito (hosted UI or custom form).
 * For MVP-B local development we use a simple JWT stored in localStorage so
 * the dashboard can be developed without standing up Cognito.
 *
 * The token is sent as  Authorization: Bearer <token>  on every API request.
 */

const TOKEN_KEY = "autodream_token";
const USER_KEY  = "autodream_user";

// ── Token storage ─────────────────────────────────────────────────────────────

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

// ── Simple session state ──────────────────────────────────────────────────────

export interface StoredUser {
  id: string;
  email: string;
  fullName: string;
}

export function getStoredUser(): StoredUser | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StoredUser;
  } catch {
    return null;
  }
}

export function setStoredUser(user: StoredUser): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function isAuthenticated(): boolean {
  return !!getToken();
}

// ── Dev-only: simulate login ──────────────────────────────────────────────────
// POST /api/v1/auth/login (if api-gateway has a dev login endpoint).
// In production, replace with Cognito hosted-UI redirect.

export async function devLogin(
  email: string,
  password: string
): Promise<{ token: string; user: StoredUser }> {
  const res = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error((err as { message?: string }).message ?? "Login failed");
  }
  const data = (await res.json()) as { token: string; user: StoredUser };
  setToken(data.token);
  setStoredUser(data.user);
  return data;
}

export async function devRegister(
  email: string,
  password: string,
  fullName: string
): Promise<{ token: string; user: StoredUser }> {
  const res = await fetch("/api/v1/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password, fullName }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error((err as { message?: string }).message ?? "Registration failed");
  }
  const data = (await res.json()) as { token: string; user: StoredUser };
  setToken(data.token);
  setStoredUser(data.user);
  return data;
}

export function logout(): void {
  clearToken();
  window.location.href = "/login";
}
