import type { Metadata } from "next";
import { Header } from "@/components/layout/header";
import { ApplicationList } from "@/components/applications/application-list";

export const metadata: Metadata = { title: "Applications" };

export default function ApplicationsPage() {
  return (
    <div className="flex flex-col h-full">
      <Header
        title="Applications"
        subtitle="Track every application AutoDreamApplier has submitted on your behalf."
      />
      <div className="flex-1 px-6 pb-8">
        <ApplicationList />
      </div>
    </div>
  );
}
