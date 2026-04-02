'use client'

import { useEffect } from "react";
import { Check, ArrowRight } from "lucide-react";
import { motion, useAnimate } from "framer-motion";
import { Button } from "@/components/ui/button";
import Link from "next/link";

const fadeSlideUp = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: 0.22, ease: [0.16, 1, 0.3, 1] } },
};

const staggerContainer = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.06, delayChildren: 0.08 } },
};

const NEXT_STEPS = [
  "Job Discovery runs every 15 minutes across 2 boards",
  "New matches land in your Review Queue",
  "Approve a match to kick off AI prep + auto-apply",
  "Track every application in real-time",
];

const PARTICLE_COLORS = [
  '#6366f1', '#8b5cf6', '#10b981', '#f59e0b',
  '#ec4899', '#06b6d4', '#6366f1', '#8b5cf6',
];

export function StepComplete() {
  const [scope, animate] = useAnimate();

  useEffect(() => {
    const particles = Array.from({ length: 8 }, (_, i) => `.particle-${i}`);
    particles.forEach((selector, i) => {
      const angle = (i / 8) * Math.PI * 2;
      animate(selector, {
        x: [0, Math.cos(angle) * 80],
        y: [0, Math.sin(angle) * 80],
        opacity: [1, 0],
        scale: [1, 0],
      }, { duration: 0.6, delay: 0.2 });
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div ref={scope} className="flex flex-col items-center py-8 text-center">
      {/* Check icon with particles */}
      <div className="relative mb-6">
        <motion.div
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={{ type: "spring", stiffness: 500, damping: 22, delay: 0.1 }}
          className="w-20 h-20 rounded-full flex items-center justify-center mx-auto"
          style={{ background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}
        >
          <Check className="w-10 h-10 text-white" />
        </motion.div>

        {/* Confetti burst */}
        {Array.from({ length: 8 }, (_, i) => (
          <motion.span
            key={i}
            className={`particle-${i} absolute w-3 h-3 rounded-full`}
            style={{
              backgroundColor: PARTICLE_COLORS[i],
              top: "50%",
              left: "50%",
              translateX: "-50%",
              translateY: "-50%",
            }}
          />
        ))}
      </div>

      <motion.div
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.3, duration: 0.3 }}
      >
        <h2 className="text-xl font-bold text-gray-900">You&apos;re all set!</h2>
        <p className="text-sm text-gray-500 mt-2 max-w-sm">
          AutoDreamApplier is scanning job boards for you. New matches will appear
          in your queue for review. You&apos;ll be notified when applications are submitted.
        </p>
      </motion.div>

      <div className="bg-gray-50 rounded-xl p-4 text-left w-full mt-6">
        <p className="text-xs font-semibold text-gray-700 uppercase tracking-wide mb-3">
          What happens next
        </p>
        <motion.ul
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
          className="space-y-3"
        >
          {NEXT_STEPS.map((s, i) => (
            <motion.li key={i} variants={fadeSlideUp} className="flex items-start gap-2">
              <span className="w-5 h-5 rounded-full bg-brand-100 text-brand-700 text-[10px] font-bold flex items-center justify-center shrink-0 mt-0.5">
                {i + 1}
              </span>
              <p className="text-sm text-gray-600">{s}</p>
            </motion.li>
          ))}
        </motion.ul>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.5, duration: 0.3 }}
        className="w-full mt-6"
      >
        <Button asChild className="w-full" size="lg">
          <Link href="/dashboard/matches">
            Go to Match Queue
            <ArrowRight className="h-4 w-4 ml-1" />
          </Link>
        </Button>
      </motion.div>
    </div>
  );
}
