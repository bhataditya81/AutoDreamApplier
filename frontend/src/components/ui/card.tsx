'use client';

import * as React from "react";
import { motion, type HTMLMotionProps } from "framer-motion";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const cardVariants = cva(
  "rounded-xl",
  {
    variants: {
      variant: {
        default: 'bg-white border border-gray-100 shadow-[0_1px_2px_rgba(0,0,0,.04),0_1px_4px_rgba(0,0,0,.04)]',
        glass:   'glass shadow-[0_8px_32px_rgba(99,102,241,.10),inset_0_1px_0_rgba(255,255,255,.8)]',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
);

type CardVariantProps = VariantProps<typeof cardVariants>;

// Merge motion.div props with our variant props, excluding the HTML `ref` since motion.div manages its own.
type CardProps = Omit<HTMLMotionProps<'div'>, 'ref'> & CardVariantProps & {
  className?: string;
};

const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant, ...props }, ref) => (
    <motion.div
      ref={ref}
      className={cn(cardVariants({ variant }), className)}
      whileHover={
        variant !== 'glass'
          ? { boxShadow: '0 8px 24px rgba(99,102,241,.12), 0 2px 8px rgba(0,0,0,.06)' }
          : undefined
      }
      transition={{ duration: 0.2 }}
      {...props}
    />
  )
);
Card.displayName = "Card";

const CardHeader = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("flex flex-col gap-1.5 p-5 pb-0", className)}
    {...props}
  />
));
CardHeader.displayName = "CardHeader";

const CardTitle = React.forwardRef<
  HTMLHeadingElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h3
    ref={ref}
    className={cn("font-semibold leading-none tracking-tight text-gray-900", className)}
    {...props}
  />
));
CardTitle.displayName = "CardTitle";

const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p ref={ref} className={cn("text-sm text-gray-500", className)} {...props} />
));
CardDescription.displayName = "CardDescription";

const CardContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn("p-5", className)} {...props} />
));
CardContent.displayName = "CardContent";

const CardFooter = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      "flex items-center gap-3 px-5 py-4 border-t border-border bg-gray-50/60 rounded-b-xl",
      className
    )}
    {...props}
  />
));
CardFooter.displayName = "CardFooter";

export { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter };
