"use client";

import { useState, useRef } from "react";
import { Upload, FileText, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { uploadResume } from "@/lib/api";
import type { Resume } from "@/lib/types";

interface StepResumeProps {
  onComplete: (resume: Resume) => void;
}

export function StepResume({ onComplete }: StepResumeProps) {
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploaded, setUploaded] = useState<Resume | null>(null);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    const dropped = e.dataTransfer.files[0];
    if (dropped) handleFile(dropped);
  }

  function handleFile(f: File) {
    const allowed = ["application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"];
    if (!allowed.includes(f.type)) {
      setError("Only PDF and DOCX files are supported.");
      return;
    }
    if (f.size > 10 * 1024 * 1024) {
      setError("File size must be under 10 MB.");
      return;
    }
    setError(null);
    setFile(f);
  }

  async function handleUpload() {
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      const resume = await uploadResume(file);
      setUploaded(resume);
      onComplete(resume);
    } catch (e: unknown) {
      console.error("[StepResume] handleUpload error:", e);
      setError(e instanceof Error ? e.message : "Upload failed. Please try again.");
    } finally {
      setUploading(false);
    }
  }

  if (uploaded) {
    return (
      <div className="flex flex-col items-center py-8 text-center">
        <CheckCircle2 className="h-12 w-12 text-green-500 mb-3" />
        <h3 className="font-semibold text-gray-900 mb-1">Resume uploaded!</h3>
        <p className="text-sm text-gray-500">{uploaded.fileName}</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-gray-900">Upload your resume</h2>
        <p className="text-sm text-gray-500 mt-1">
          We&apos;ll parse your resume and use it to find and apply to matching jobs.
          Supported formats: PDF, DOCX (max 10 MB).
        </p>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {/* Drop zone */}
      <div
        onDragOver={(e) => e.preventDefault()}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
        className="border-2 border-dashed border-gray-200 rounded-xl p-10 flex flex-col items-center gap-3 cursor-pointer hover:border-brand-400 hover:bg-brand-50 transition-colors"
      >
        <input
          ref={inputRef}
          type="file"
          accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) handleFile(f);
          }}
        />
        {file ? (
          <>
            <FileText className="h-10 w-10 text-brand-500" />
            <p className="font-medium text-gray-900 text-sm">{file.name}</p>
            <p className="text-xs text-gray-400">
              {(file.size / 1024 / 1024).toFixed(1)} MB
            </p>
          </>
        ) : (
          <>
            <Upload className="h-10 w-10 text-gray-300" />
            <div className="text-center">
              <p className="text-sm font-medium text-gray-700">
                Drag & drop or click to browse
              </p>
              <p className="text-xs text-gray-400 mt-0.5">PDF or DOCX, up to 10 MB</p>
            </div>
          </>
        )}
      </div>

      <Button
        className="w-full"
        onClick={handleUpload}
        disabled={!file || uploading}
        loading={uploading}
      >
        {uploading ? "Uploading…" : "Upload & Continue"}
      </Button>
    </div>
  );
}
