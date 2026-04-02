'use client'

import { motion } from "framer-motion";
import { Header } from "@/components/layout/header";
import { MatchQueue } from "@/components/matches/match-queue";

export default function MatchesPage() {
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.15 }}
      className="flex flex-col h-full"
    >
      <Header
        title="Match Queue"
        subtitle="Review jobs found for you and approve the ones you want to apply to."
      />
      <div className="flex-1 px-6 pb-8">
        <MatchQueue />
      </div>
    </motion.div>
  );
}
