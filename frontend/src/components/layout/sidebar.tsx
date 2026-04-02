'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  LayoutDashboard,
  Layers,
  FileText,
  Settings,
  LogOut,
  BarChart2,
  LineChart,
  Zap,
} from 'lucide-react';
import { logout } from '@/lib/auth';
import { getPreferences } from '@/lib/api';

const navItems = [
  { href: '/dashboard/overview',     label: 'Overview',      icon: BarChart2 },
  { href: '/dashboard/matches',      label: 'Match Queue',   icon: LayoutDashboard },
  { href: '/dashboard/applications', label: 'Applications',  icon: Layers },
  { href: '/dashboard/analytics',    label: 'Analytics',     icon: LineChart },
  { href: '/dashboard/resumes',      label: 'Resumes',       icon: FileText },
  { href: '/dashboard/settings',     label: 'Settings',      icon: Settings },
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
    <motion.aside
      initial={{ x: -20, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
      className="flex flex-col w-64 min-h-screen bg-[#0f172a] border-r border-[#334155] shrink-0"
    >
      {/* Logo */}
      <div className="h-16 flex items-center px-5 border-b border-[#334155] shrink-0">
        <div className="flex items-center gap-2.5">
          <div
            className="w-8 h-8 rounded-lg flex items-center justify-center"
            style={{ background: 'var(--gradient-brand)' }}
          >
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="text-white font-semibold text-sm tracking-tight">AutoDream</span>
          <span className="text-[#6366f1] font-semibold text-sm">Applier</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 overflow-y-auto sidebar-scroll">
        {navItems.map(({ href, label, icon: Icon }) => {
          const isActive = pathname.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              className="relative flex items-center gap-3 px-3 py-2.5 mx-2 rounded-lg group"
            >
              {isActive && (
                <motion.div
                  layoutId="sidebar-active-pill"
                  className="absolute inset-0 rounded-lg border-l-2 border-indigo-500"
                  style={{ background: 'var(--gradient-sidebar-active)' }}
                  transition={{ type: 'spring', stiffness: 380, damping: 30 }}
                />
              )}
              <Icon
                className={`relative z-10 w-4 h-4 shrink-0 ${
                  isActive
                    ? 'text-indigo-400'
                    : 'text-[#64748b] group-hover:text-[#94a3b8]'
                }`}
                aria-hidden="true"
              />
              <span
                className={`relative z-10 text-sm font-medium transition-colors duration-150 ${
                  isActive
                    ? 'text-[#f1f5f9]'
                    : 'text-[#94a3b8] group-hover:text-[#f1f5f9]'
                }`}
              >
                {label}
              </span>
              {href === '/dashboard/matches' && autoApplyOn && (
                <div className="relative ml-auto flex items-center justify-center w-2 h-2">
                  <span className="absolute inline-flex w-full h-full rounded-full bg-green-400 opacity-75 animate-ping" />
                  <span className="relative inline-flex w-2 h-2 rounded-full bg-green-500" />
                </div>
              )}
            </Link>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="border-t border-[#334155] px-3 py-4">
        <button
          onClick={logout}
          className="flex w-full items-center gap-3 px-3 py-2 rounded-lg text-sm text-[#94a3b8] hover:text-red-400 hover:bg-red-500/10 transition-colors duration-150"
        >
          <LogOut className="w-4 h-4 shrink-0" aria-hidden="true" />
          Log out
        </button>
      </div>
    </motion.aside>
  );
}
