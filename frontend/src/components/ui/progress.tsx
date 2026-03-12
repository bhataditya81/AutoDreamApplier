import * as React from "react";
import { cn } from "@/lib/utils";

interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number; // 0-100; undefined = indeterminate
  size?: "sm" | "md" | "lg";
  color?: "brand" | "green" | "yellow" | "red";
}

const colorMap: Record<string, string> = {
  brand:  "bg-brand-600",
  green:  "bg-green-500",
  yellow: "bg-yellow-500",
  red:    "bg-red-500",
};

const sizeMap: Record<string, string> = {
  sm: "h-1.5",
  md: "h-2",
  lg: "h-3",
};

function Progress({ className, value, size = "md", color = "brand", ...props }: ProgressProps) {
  const isIndeterminate = value === undefined;
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value}
      className={cn(
        "w-full overflow-hidden rounded-full bg-gray-200",
        sizeMap[size],
        className
      )}
      {...props}
    >
      {isIndeterminate ? (
        <div
          className={cn(
            "h-full w-1/3 rounded-full animate-progress-indeterminate",
            colorMap[color]
          )}
        />
      ) : (
        <div
          className={cn("h-full rounded-full transition-all duration-300", colorMap[color])}
          style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
        />
      )}
    </div>
  );
}

export { Progress };
