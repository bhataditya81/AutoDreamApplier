"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import {
  LayoutDashboard,
  Layers,
  FileText,
  Settings,
  Zap,
  LogOut,
  BarChart2,
  LineChart,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { logout } from "@/lib/auth";
import { getPreferences } from "@/lib/api";

const navItems = [
  { href: "/dashboard/overview",     label: "Overview",     icon: BarChart2 },
  { href: "/dashboard/matches",      label: "Match Queue",  icon: LayoutDashboard },
  { href: "/dashboard/applications", label: "Applications", icon: Layers },
  { href: "/dashboard/analytics",    label: "Analytics",    icon: LineChart },
  { href: "/dashboard/resumes",      label: "Resumes",      icon: FileText },
  { href: "/dashboard/settings",     label: "Settings",     icon: Settings },
] as const;

export function Sidebar() {
  const pathname = usePathname();
  const [autoApplyOn, setAutoApplyOn] = useState(false);

  useEffect(() => {
    getPreferences()
      .then((p) => setAutoApplyOn(p.autoApplyEnabled ?? false))
      .catch(() => {/* sidebar badge is best-effort */});
  }, []);

  return (
    <aside className="flex flex-col w-60 min-h-screen border-r border-border bg-white shrink-0">
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-5 h-16 border-b border-border shrink-0">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600">
          <Zap className="h-4 w-4 text-white" />
        </div>
        <span className="font-semibold text-gray-900 text-sm tracking-tight">
          AutoDream
        </span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-0.5">
        {navItems.map(({ href, label, icon: Icon }) => {
          const active = pathname.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                active
                  ? "bg-brand-50 text-brand-700 font-medium"
                  : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
              )}
            >
              <Icon
                className={cn("h-4 w-4 shrink-0", active ? "text-brand-600" : "text-gray-400")}
                aria-hidden="true"
              />
              {label}
              {href === "/dashboard/matches" && autoApplyOn && (
                <span
                  title="Auto-Apply is active"
                  className="ml-auto flex h-4 items-center rounded-full bg-green-500 px-1.5 text-[9px] font-semibold text-white leading-none"
                >
                  AUTO
                </span>
              )}
            </Link>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="px-3 py-4 border-t border-border">
        <button
          onClick={logout}
          className="flex w-full items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-600 hover:bg-red-50 hover:text-red-600 transition-colors"
        >
          <LogOut className="h-4 w-4 text-gray-400" aria-hidden="true" />
          Log out
        </button>
      </div>
    </aside>
  );
}
