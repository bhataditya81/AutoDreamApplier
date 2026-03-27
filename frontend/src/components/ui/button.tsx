import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
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
          "bg-brand-600 text-white shadow hover:bg-brand-700 active:bg-brand-800",
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
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        disabled={disabled ?? loading}
        {...props}
      >
        {asChild ? children : (
          <>
            {loading && (
              <svg
                className="animate-spin h-4 w-4"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
            )}
            {children}
          </>
        )}
      </Comp>
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
