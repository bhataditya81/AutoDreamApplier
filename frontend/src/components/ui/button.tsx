'use client'
import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg",
    "text-sm font-medium transition-colors",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2",
    "disabled:pointer-events-none disabled:opacity-50",
  ].join(" "),
  {
    variants: {
      variant: {
        default:
          "gradient-bg gradient-bg-hover text-white shadow",
        secondary:
          "bg-brand-100 text-brand-700 hover:bg-brand-200",
        outline:
          "border border-border bg-white text-gray-700 hover:bg-gray-50 hover:border-gray-300",
        ghost:
          "text-gray-600 hover:bg-gray-100 hover:text-gray-900",
        destructive:
          "bg-red-600 text-white shadow hover:bg-red-700",
        success:
          "bg-green-600 text-white shadow hover:bg-green-700",
        link:
          "text-brand-600 underline-offset-4 hover:underline p-0 h-auto",
      },
      size: {
        sm:   "h-8  px-3  text-xs",
        md:   "h-9  px-4  text-sm",
        lg:   "h-10 px-6  text-sm",
        xl:   "h-12 px-8  text-base",
        icon: "h-9  w-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "md",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
  loading?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, loading, children, disabled, ...props }, ref) => {
    if (asChild) {
      return (
        <Slot
          className={cn(buttonVariants({ variant, size, className }))}
          ref={ref}
          {...(props as React.HTMLAttributes<HTMLElement>)}
        >
          {children}
        </Slot>
      );
    }

    return (
      <motion.div
        className="inline-flex"
        whileTap={{ scale: 0.97 }}
        whileHover={variant === "default" ? { filter: "brightness(1.08)" } : undefined}
        transition={{ duration: 0.1 }}
        style={{ display: "inline-flex" }}
      >
        <button
          className={cn(buttonVariants({ variant, size, className }))}
          ref={ref}
          disabled={disabled ?? loading}
          {...props}
        >
          <AnimatePresence initial={false}>
            {loading && (
              <motion.svg
                key="spinner"
                initial={{ opacity: 0, scale: 0.6 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.6 }}
                transition={{ duration: 0.15 }}
                className="h-4 w-4"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </motion.svg>
            )}
          </AnimatePresence>
          {children}
        </button>
      </motion.div>
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
