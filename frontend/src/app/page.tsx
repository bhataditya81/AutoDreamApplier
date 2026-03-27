import { redirect } from "next/navigation";

// Root page: redirect to dashboard (authenticated) or login.
// Actual auth guard is handled inside the dashboard layout.
export default function RootPage() {
  redirect("/dashboard/overview");
}
