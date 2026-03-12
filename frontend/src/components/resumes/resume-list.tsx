"use client";

import { useState, useEffect, useRef } from "react";
import { Upload, Star, Trash2, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Alert } from "@/components/ui/alert";
import { listResumes, uploadResume, setPrimaryResume, deleteResume } from "@/lib/api";
import type { Resume } from "@/lib/types";

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
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
            <li key={resume.id} className="flex items-center gap-4 px-4 py-3 bg-white hover:bg-gray-50 transition-colors">
              <div className="w-9 h-9 rounded-lg bg-brand-50 flex items-center justify-center shrink-0">
                <FileText className="h-4 w-4 text-brand-600" />
              </div>

              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-900 truncate">{resume.fileName}</span>
                  {resume.isPrimary && (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-brand-50 text-brand-700">
                      <Star className="h-3 w-3" />
                      Primary
                    </span>
                  )}
                </div>
                <p className="text-xs text-gray-500 mt-0.5">
                  Uploaded {formatDate(resume.createdAt)}
                </p>
              </div>

              <div className="flex items-center gap-2 shrink-0">
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
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
