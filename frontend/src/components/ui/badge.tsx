'use client';

import * as React from "react";
import { motion, type HTMLMotionProps } from "framer-motion";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default:     "bg-gray-100 text-gray-700",
        brand:       "bg-brand-100 text-brand-700",
        success:     "bg-green-100 text-green-700",
        warning:     "bg-yellow-100 text-yellow-700",
        danger:      "bg-red-100 text-red-700",
        info:        "bg-sky-100 text-sky-700",
        outline:     "border border-gray-200 text-gray-600",
        // Application statuses
        queued:      "bg-gray-100 text-gray-600",
        ai_preparing:"bg-purple-100 text-purple-700",
        ai_ready:    "bg-indigo-100 text-indigo-700",
        applying:    "bg-yellow-100 text-yellow-700",
        applied:     "bg-green-100 text-green-700",
        failed:      "bg-red-100 text-red-700",
        withdrawn:   "bg-gray-100 text-gray-500",
        // Outcomes
        interview:   "bg-emerald-100 text-emerald-700",
        offer:       "bg-teal-100 text-teal-700",
        rejected:    "bg-red-100 text-red-600",
        viewed:      "bg-sky-100 text-sky-600",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

// Use only safe motion props (no conflicting HTML event props like onDrag)
type BadgeMotionProps = Pick<
  HTMLMotionProps<'span'>,
  'style' | 'className' | 'id' | 'aria-label' | 'aria-hidden' | 'role' | 'tabIndex' | 'onClick'
>;

export interface BadgeProps extends BadgeMotionProps, VariantProps<typeof badgeVariants> {
  children?: React.ReactNode;
}

function Badge({ className, variant, children, ...props }: BadgeProps) {
  return (
    <motion.span
      initial={{ scale: 0.8, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      transition={{ type: 'spring', stiffness: 400, damping: 25 }}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    >
      {variant === 'applying' && (
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-yellow-400 mr-1.5 animate-pulse" />
      )}
      {children}
    </motion.span>
  );
}

export { Badge, badgeVariants };
