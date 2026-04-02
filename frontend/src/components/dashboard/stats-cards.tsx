'use client';

import { useRef, useEffect } from "react";
import { motion, useMotionValue, useTransform, animate, useInView } from "framer-motion";
import { Send, Eye, MessageSquare, Trophy, TrendingUp } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { staggerContainer, fadeSlideUp } from "@/lib/motion";
import type { FunnelStats } from "@/lib/types";

/** Animated integer counter that counts up when it enters the viewport */
function AnimatedNumber({ value }: { value: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const count = useMotionValue(0);
  const rounded = useTransform(count, Math.round);
  const inView = useInView(ref, { once: true });

  useEffect(() => {
    if (inView) {
      const controls = animate(count, value, { duration: 0.8, ease: "easeOut" });
      return controls.stop;
    }
  }, [inView, value, count]);

  return <motion.span ref={ref}>{rounded}</motion.span>;
}

interface StatCardProps {
  label: string;
  value: number;
  icon: React.ReactNode;
  suffix?: string;
  colorClass: string;
  bgClass: string;
}

function StatCard({ label, value, icon, suffix, colorClass, bgClass }: StatCardProps) {
  return (
    <motion.div
      variants={fadeSlideUp}
      whileHover={{ y: -3, boxShadow: '0 8px 24px rgba(99,102,241,.15)' }}
      whileTap={{ y: 0, scale: 0.99 }}
      transition={{ type: 'spring', stiffness: 400, damping: 30 }}
    >
      <Card className="h-full">
        <CardContent className="flex items-center gap-4 p-5">
          <div className={`w-11 h-11 rounded-xl flex items-center justify-center shrink-0 ${bgClass}`}>
            <span className={colorClass}>{icon}</span>
          </div>
          <div className="min-w-0">
            <p className="text-xs text-gray-500 font-medium mb-0.5">{label}</p>
            <p className="text-2xl font-bold text-gray-900 tabular-nums leading-none">
              <AnimatedNumber value={value} />
              {suffix && (
                <span className="text-sm font-medium text-gray-500 ml-0.5">{suffix}</span>
              )}
            </p>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  );
}

interface StatsCardsProps {
  stats: FunnelStats;
}

export function StatsCards({ stats }: StatsCardsProps) {
  const cards: StatCardProps[] = [
    {
      label: "Applications Sent",
      value: stats.applied,
      icon: <Send className="h-5 w-5" />,
      colorClass: "text-indigo-600",
      bgClass: "bg-indigo-50",
    },
    {
      label: "Profile Views",
      value: stats.viewed,
      icon: <Eye className="h-5 w-5" />,
      colorClass: "text-sky-600",
      bgClass: "bg-sky-50",
    },
    {
      label: "Interviews",
      value: stats.interviews,
      icon: <MessageSquare className="h-5 w-5" />,
      colorClass: "text-emerald-600",
      bgClass: "bg-emerald-50",
    },
    {
      label: "Offers",
      value: stats.offers,
      icon: <Trophy className="h-5 w-5" />,
      colorClass: "text-amber-600",
      bgClass: "bg-amber-50",
    },
    {
      label: "Interview Rate",
      value: Math.round(stats.interview_rate),
      suffix: "%",
      icon: <TrendingUp className="h-5 w-5" />,
      colorClass: "text-violet-600",
      bgClass: "bg-violet-50",
    },
  ];

  return (
    <motion.div
      variants={staggerContainer}
      initial="hidden"
      animate="visible"
      className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
    >
      {cards.map((card) => (
        <StatCard key={card.label} {...card} />
      ))}
    </motion.div>
  );
}
