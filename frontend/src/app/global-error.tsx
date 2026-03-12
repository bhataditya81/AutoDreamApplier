"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[Global Error Boundary]", error);
  }, [error]);

  return (
    <html>
      <body className="flex h-screen flex-col items-center justify-center gap-4 bg-background p-8 font-sans">
        <h2 className="text-xl font-semibold text-red-600">Application Error</h2>
        <p className="max-w-md text-center text-sm text-gray-500">
          {error.message || "A critical error occurred. Please refresh the page."}
        </p>
        <button
          onClick={reset}
          className="rounded bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700"
        >
          Try again
        </button>
      </body>
    </html>
  );
}
