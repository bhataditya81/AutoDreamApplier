import type { Metadata } from "next";
import { Header } from "@/components/layout/header";
import { ResumeList } from "@/components/resumes/resume-list";

export const metadata: Metadata = { title: "Resumes" };

export default function ResumesPage() {
  return (
    <div className="flex flex-col h-full">
      <Header
        title="Resumes"
        subtitle="Manage the resumes AutoDreamApplier uses when applying on your behalf."
      />
      <div className="flex-1 px-6 pb-8">
        <ResumeList />
      </div>
    </div>
  );
}
