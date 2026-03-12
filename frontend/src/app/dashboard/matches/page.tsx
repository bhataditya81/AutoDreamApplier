import type { Metadata } from "next";
import { Header } from "@/components/layout/header";
import { MatchQueue } from "@/components/matches/match-queue";

export const metadata: Metadata = { title: "Match Queue" };

export default function MatchesPage() {
  return (
    <div className="flex flex-col h-full">
      <Header
        title="Match Queue"
        subtitle="Review jobs found for you and approve the ones you want to apply to."
      />
      <div className="flex-1 px-6 pb-8">
        <MatchQueue />
      </div>
    </div>
  );
}
