'use client';

import { Bell, User } from 'lucide-react';
import { motion } from 'framer-motion';

interface HeaderProps {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
}

export function Header({ title, subtitle, actions }: HeaderProps) {
  return (
    <header className="sticky top-0 z-10 flex items-center justify-between px-6 h-16 bg-white/90 backdrop-blur-sm border-b border-gray-100/80 shrink-0">
      <div>
        <h1 className="text-lg font-semibold text-gray-900 leading-tight">{title}</h1>
        {subtitle && (
          <p className="text-sm text-gray-500 mt-0.5">{subtitle}</p>
        )}
      </div>

      <div className="flex items-center gap-2">
        {actions}
        <motion.button
          whileHover={{ rotate: [0, -15, 15, -8, 0] }}
          transition={{ duration: 0.4 }}
          className="flex h-9 w-9 items-center justify-center rounded-lg hover:bg-gray-100 transition-colors"
          aria-label="Notifications"
        >
          <Bell className="w-5 h-5 text-gray-500" />
        </motion.button>
        <button
          className="w-8 h-8 rounded-full ring-2 ring-indigo-500/20 hover:ring-indigo-500/50 transition-all cursor-pointer bg-indigo-100 flex items-center justify-center"
          aria-label="Account"
        >
          <User className="w-4 h-4 text-indigo-600" />
        </button>
      </div>
    </header>
  );
}
