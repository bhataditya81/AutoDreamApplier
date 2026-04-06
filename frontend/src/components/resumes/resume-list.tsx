"use client";

import { useState, useEffect, useRef } from "react";
import { Upload, Star, Trash2, FileText, Trophy, FlaskConical } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { listResumes, uploadResume, setPrimaryResume, deleteResume, updateResumeAB } from "@/lib/api";
import type { Resume } from "@/lib/types";

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

/** Returns the resume with the highest interview_rate that has >= 5 total_applications. */
function getBestPerformer(resumes: Resume[]): string | null {
  const eligible = resumes.filter((r) => r.total_applications >= 5);
  if (eligible.length === 0) return null;
  const best = eligible.reduce((a, b) =>
    a.interview_rate > b.interview_rate ? a : b
  );
  return best.id;
}

export function ResumeList() {
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  async function load() {
    try {
      setError(null);
      const data = await listResumes();
      setResumes(data);
    } catch (e: unknown) {
      console.error("[ResumeList] load error:", e);
      setError(e instanceof Error ? e.message : "Failed to load resumes");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    // Reset so the same file can be re-uploaded
    e.target.value = "";
    setUploading(true);
    setError(null);
    try {
      const created = await uploadResume(file);
      setResumes((prev) => {
        // If this is primary, unset previous primaries
        const updated = created.isPrimary
          ? prev.map((r) => ({ ...r, isPrimary: false }))
          : [...prev];
        return [created, ...updated];
      });
    } catch (e: unknown) {
      console.error("[ResumeList] upload error:", e);
      setError(e instanceof Error ? e.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }

  async function handleSetPrimary(resumeId: string) {
    setActionId(resumeId);
    setError(null);
    try {
      await setPrimaryResume(resumeId);
      setResumes((prev) =>
        prev.map((r) => ({ ...r, isPrimary: r.id === resumeId }))
      );
    } catch (e: unknown) {
      console.error("[ResumeList] setPrimary error:", e);
      setError(e instanceof Error ? e.message : "Failed to set primary");
    } finally {
      setActionId(null);
    }
  }

  async function handleDelete(resumeId: string) {
    setActionId(resumeId);
    setError(null);
    try {
      await deleteResume(resumeId);
      setResumes((prev) => prev.filter((r) => r.id !== resumeId));
    } catch (e: unknown) {
      console.error("[ResumeList] delete error:", e);
      setError(e instanceof Error ? e.message : "Failed to delete resume");
    } finally {
      setActionId(null);
    }
  }

  async function handleToggleAB(resume: Resume) {
    setActionId(resume.id);
    setError(null);
    const newEnabled = !resume.ab_enabled;
    const weight = resume.ab_weight > 0 ? resume.ab_weight : 1;
    try {
      const updated = await updateResumeAB(resume.id, newEnabled, weight);
      setResumes((prev) =>
        prev.map((r) => (r.id === resume.id ? updated : r))
      );
    } catch (e: unknown) {
      console.error("[ResumeList] toggleAB error:", e);
      setError(e instanceof Error ? e.message : "Failed to update A/B settings");
    } finally {
      setActionId(null);
    }
  }

  const showABControls = resumes.length >= 2;
  const bestPerformerId = getBestPerformer(resumes);

  return (
    <div className="space-y-5 max-w-2xl">
      {/* Upload area */}
      <div
        className="flex flex-col items-center justify-center border-2 border-dashed border-gray-200 rounded-xl p-8 gap-3 cursor-pointer hover:border-brand-400 transition-colors"
        onClick={() => fileRef.current?.click()}
      >
        {uploading ? (
          <Spinner size="md" />
        ) : (
          <>
            <div className="w-12 h-12 rounded-full bg-brand-50 flex items-center justify-center">
              <Upload className="h-5 w-5 text-brand-600" />
            </div>
            <div className="text-center">
              <p className="text-sm font-medium text-gray-900">
                Click to upload a resume
              </p>
              <p className="text-xs text-gray-500 mt-0.5">PDF or DOCX · max 10 MB</p>
            </div>
          </>
        )}
        <input
          ref={fileRef}
          type="file"
          accept=".pdf,.docx"
          className="hidden"
          onChange={handleFileChange}
        />
      </div>

      {showABControls && (
        <div className="flex items-start gap-2 px-3 py-2 rounded-lg bg-sky-50 border border-sky-100">
          <FlaskConical className="h-4 w-4 text-sky-600 mt-0.5 shrink-0" />
          <p className="text-xs text-sky-700">
            <span className="font-medium">A/B Testing</span> — enable the toggle on 2 or more resumes to have AutoDreamApplier rotate between them. The resume with the highest interview rate earns a Best Performer badge once it has 5+ applications.
          </p>
        </div>
      )}

      {error && (
        <Alert variant="error" title="Error">
          {error}
        </Alert>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <Spinner size="lg" />
        </div>
      ) : resumes.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-center gap-3">
          <div className="w-16 h-16 rounded-full bg-gray-100 flex items-center justify-center">
            <FileText className="h-8 w-8 text-gray-400" />
          </div>
          <p className="text-sm font-semibold text-gray-900">No resumes yet</p>
          <p className="text-sm text-gray-500 max-w-xs">
            Upload your first resume above to start applying automatically.
          </p>
        </div>
      ) : (
        <ul className="divide-y divide-gray-100 border border-gray-200 rounded-xl overflow-hidden">
          {resumes.map((resume) => (
            <li key={resume.id} className="flex flex-col px-4 py-3 bg-white hover:bg-gray-50 transition-colors gap-2">
              {/* Top row: icon + name + badges + actions */}
              <div className="flex items-center gap-4">
                <div className="w-9 h-9 rounded-lg bg-brand-50 flex items-center justify-center shrink-0">
                  <FileText className="h-4 w-4 text-brand-600" />
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-medium text-gray-900 truncate">{resume.fileName}</span>
                    {resume.isPrimary && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-brand-50 text-brand-700">
                        <Star className="h-3 w-3" />
                        Primary
                      </span>
                    )}
                    {bestPerformerId === resume.id && (
                      <Badge variant="success">
                        <Trophy className="h-3 w-3" />
                        Best Performer
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">
                    Uploaded {formatDate(resume.createdAt)}
                  </p>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  {showABControls && (
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs text-gray-500">A/B</span>
                      <Switch
                        checked={resume.ab_enabled}
                        onCheckedChange={() => handleToggleAB(resume)}
                        disabled={actionId === resume.id}
                        aria-label="Toggle A/B testing for this resume"
                      />
                    </div>
                  )}
                  {!resume.isPrimary && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleSetPrimary(resume.id)}
                      disabled={actionId === resume.id}
                    >
                      {actionId === resume.id ? <Spinner size="sm" /> : "Set Primary"}
                    </Button>
                  )}
                  <button
                    onClick={() => handleDelete(resume.id)}
                    disabled={actionId === resume.id}
                    className="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors disabled:opacity-40"
                    aria-label="Delete resume"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* Stats row */}
              {resume.total_applications > 0 && (
                <div className="flex items-center gap-3 pl-[52px] text-xs text-gray-500">
                  <span>{resume.total_applications} app{resume.total_applications !== 1 ? "s" : ""}</span>
                  <span className="text-gray-300">·</span>
                  <span>{resume.total_interviews} interview{resume.total_interviews !== 1 ? "s" : ""}</span>
                  <span className="text-gray-300">·</span>
                  <span
                    className={
                      resume.interview_rate >= 20
                        ? "text-green-600 font-medium"
                        : resume.interview_rate >= 10
                        ? "text-yellow-600 font-medium"
                        : "text-gray-500"
                    }
                  >
                    {resume.interview_rate.toFixed(1)}% rate
                  </span>
                  {resume.total_offers > 0 && (
                    <>
                      <span className="text-gray-300">·</span>
                      <span className="text-teal-600 font-medium">{resume.total_offers} offer{resume.total_offers !== 1 ? "s" : ""}</span>
                    </>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
