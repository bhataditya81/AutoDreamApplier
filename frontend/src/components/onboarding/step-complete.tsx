import { CheckCircle2, Zap, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import Link from "next/link";

export function StepComplete() {
  return (
    <div className="flex flex-col items-center py-8 text-center space-y-6">
      <div className="relative">
        <div className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center">
          <CheckCircle2 className="h-10 w-10 text-green-500" />
        </div>
        <div className="absolute -top-1 -right-1 w-8 h-8 bg-brand-600 rounded-full flex items-center justify-center shadow-lg">
          <Zap className="h-4 w-4 text-white" />
        </div>
      </div>

      <div>
        <h2 className="text-xl font-bold text-gray-900">You&apos;re all set!</h2>
        <p className="text-sm text-gray-500 mt-2 max-w-sm">
          AutoDreamApplier is scanning job boards for you. New matches will appear
          in your queue for review. You&apos;ll be notified when applications are submitted.
        </p>
      </div>

      <div className="bg-gray-50 rounded-xl p-4 text-left w-full space-y-2">
        <p className="text-xs font-semibold text-gray-700 uppercase tracking-wide">
          What happens next
        </p>
        {[
          "Job Discovery runs every 15 minutes across 2 boards",
          "New matches land in your Review Queue",
          "Approve a match to kick off AI prep + auto-apply",
          "Track every application in real-time",
        ].map((step, i) => (
          <div key={i} className="flex items-start gap-2">
            <span className="w-5 h-5 rounded-full bg-brand-100 text-brand-700 text-[10px] font-bold flex items-center justify-center shrink-0 mt-0.5">
              {i + 1}
            </span>
            <p className="text-sm text-gray-600">{step}</p>
          </div>
        ))}
      </div>

      <Button asChild className="w-full" size="lg">
        <Link href="/dashboard/matches">
          Go to Match Queue
          <ArrowRight className="h-4 w-4 ml-1" />
        </Link>
      </Button>
    </div>
  );
}
