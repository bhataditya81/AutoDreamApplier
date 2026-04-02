'use client'

import { useState, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Zap } from "lucide-react";
import { motion } from "framer-motion";
import { Label } from "@/components/ui/label";
import { Alert } from "@/components/ui/alert";
import { devRegister } from "@/lib/auth";

export default function SignupPage() {
  const router = useRouter();
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Track field group for progress dots: 0 (name), 1 (email), 2 (password+submit)
  const [currentFieldGroup, setCurrentFieldGroup] = useState(0);

  useEffect(() => {
    if (password || confirm) {
      setCurrentFieldGroup(2);
    } else if (email) {
      setCurrentFieldGroup(1);
    } else if (fullName) {
      setCurrentFieldGroup(0);
    }
  }, [fullName, email, password, confirm]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await devRegister(email, password, fullName);
      router.push("/dashboard/onboarding");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Registration failed. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-950 via-indigo-950/50 to-slate-900 p-4 overflow-hidden">
      {/* Background orbs */}
      {[0, 1, 2].map((i) => (
        <motion.div
          key={i}
          className="absolute rounded-full blur-3xl opacity-20 pointer-events-none"
          style={{
            width: [320, 256, 192][i],
            height: [320, 256, 192][i],
            background: ['#6366f1', '#8b5cf6', '#6366f1'][i],
            top: ['10%', '60%', '40%'][i],
            left: ['5%', '75%', '50%'][i],
          }}
          animate={{ y: [0, -20, 0], x: [0, 10, 0] }}
          transition={{
            duration: [7, 9, 11][i],
            repeat: Infinity,
            ease: 'easeInOut',
            delay: i * 1.5,
          }}
        />
      ))}

      <motion.div
        initial={{ opacity: 0, y: 24, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
        className="bg-slate-800/70 backdrop-blur-xl border border-indigo-500/20 rounded-2xl p-8 w-full max-w-md relative z-10 shadow-[0_25px_50px_rgba(0,0,0,.5)]"
      >
        {/* Progress dots */}
        <div className="flex items-center gap-2 mb-6">
          {[0, 1, 2].map((i) => (
            <motion.div
              key={i}
              className="w-2 h-2 rounded-full"
              animate={{
                backgroundColor: i <= currentFieldGroup ? '#6366f1' : '#334155',
                scale: i === currentFieldGroup ? 1.25 : 1,
              }}
              transition={{ duration: 0.2 }}
            />
          ))}
        </div>

        {/* Logo */}
        <div
          className="w-10 h-10 rounded-xl flex items-center justify-center mb-6"
          style={{ background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}
        >
          <Zap className="w-5 h-5 text-white" />
        </div>

        <h1 className="text-2xl font-bold text-white mb-1">Create your account</h1>
        <p className="text-slate-400 text-sm mb-8">
          Start applying on autopilot — free trial, no card required.
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <Alert variant="error">{error}</Alert>}

          <div className="space-y-2">
            <Label className="text-slate-300">Full name</Label>
            <input
              type="text"
              placeholder="Jane Smith"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              autoComplete="name"
              className="w-full h-10 rounded-lg px-3 text-sm bg-slate-800/60 border border-slate-600 text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 transition-colors"
            />
          </div>

          <div className="space-y-2">
            <Label required className="text-slate-300">Email</Label>
            <input
              type="email"
              placeholder="jane@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="w-full h-10 rounded-lg px-3 text-sm bg-slate-800/60 border border-slate-600 text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 transition-colors"
            />
          </div>

          <div className="space-y-2">
            <Label required className="text-slate-300">Password</Label>
            <input
              type="password"
              placeholder="Min 8 characters"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full h-10 rounded-lg px-3 text-sm bg-slate-800/60 border border-slate-600 text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 transition-colors"
            />
          </div>

          <div className="space-y-2">
            <Label required className="text-slate-300">Confirm password</Label>
            <input
              type="password"
              placeholder="••••••••"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full h-10 rounded-lg px-3 text-sm bg-slate-800/60 border border-slate-600 text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 transition-colors"
            />
          </div>

          <motion.button
            type="submit"
            disabled={loading}
            whileTap={{ scale: 0.98 }}
            whileHover={{ filter: 'brightness(1.1)' }}
            className="w-full h-11 rounded-lg font-medium text-white transition-all disabled:opacity-60 disabled:cursor-not-allowed"
            style={{ background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}
          >
            {loading ? 'Creating account…' : 'Create free account'}
          </motion.button>

          <p className="text-xs text-center text-slate-500">
            By signing up you agree to our Terms of Service and Privacy Policy.
          </p>
        </form>

        <p className="text-center text-sm text-slate-400 mt-6">
          Already have an account?{" "}
          <Link
            href="/login"
            className="font-medium text-indigo-400 hover:text-indigo-300 transition-colors"
          >
            Sign in
          </Link>
        </p>
      </motion.div>
    </div>
  );
}
