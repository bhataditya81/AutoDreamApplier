'use client';
import dynamic from 'next/dynamic';
import { motion, useMotionValue, useTransform } from 'framer-motion';
import { useRef, useCallback } from 'react';
import Link from 'next/link';
import { Zap, ArrowRight, Bot, Shield, BarChart3, Globe, Layers, Bell, Check, Star } from 'lucide-react';
import MarketingNavbar from '@/components/marketing/navbar';
import FAQ from '@/components/marketing/faq';
import Contact from '@/components/marketing/contact';
import Footer from '@/components/marketing/footer';

const HeroScene = dynamic(() => import('@/components/marketing/hero-scene'), {
  ssr: false,
  loading: () => <div className="w-full h-full bg-transparent" />,
});

// ── 3D tilt card hook ─────────────────────────────────────────────────────────
function useTilt() {
  const ref = useRef<HTMLDivElement>(null);
  const x = useMotionValue(0);
  const y = useMotionValue(0);
  const rotateX = useTransform(y, [-0.5, 0.5], [8, -8]);
  const rotateY = useTransform(x, [-0.5, 0.5], [-8, 8]);

  const onMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    x.set((e.clientX - rect.left) / rect.width - 0.5);
    y.set((e.clientY - rect.top) / rect.height - 0.5);
  }, [x, y]);

  const onMouseLeave = useCallback(() => {
    x.set(0);
    y.set(0);
  }, [x, y]);

  return { ref, rotateX, rotateY, onMouseMove, onMouseLeave };
}

// ── Feature card ──────────────────────────────────────────────────────────────
const features = [
  { icon: Bot, title: 'AI-powered tailoring', desc: 'Claude rewrites your resume and cover letter for every job to maximize relevance and ATS score.', color: '#6366f1' },
  { icon: Globe, title: 'Multi-board coverage', desc: 'Discover jobs across Indeed, Glassdoor, LinkedIn and 10+ more boards simultaneously.', color: '#8b5cf6' },
  { icon: Layers, title: '8 ATS plugins', desc: 'Native support for Greenhouse, Lever, Workday, iCIMS, Taleo, SuccessFactors, SmartRecruiters & BambooHR.', color: '#a78bfa' },
  { icon: Shield, title: 'Account-safe automation', desc: 'Residential proxies, human-like browser behaviour, per-board rate limits, and CAPTCHA handling.', color: '#6366f1' },
  { icon: BarChart3, title: 'Full-funnel analytics', desc: 'Track every step from application to offer. A/B test resume versions against your real interview rate.', color: '#8b5cf6' },
  { icon: Bell, title: 'Real-time notifications', desc: "Instant Slack or Discord alerts when you're applied, screened, or invited to an interview.", color: '#a78bfa' },
];

type Feature = typeof features[0];

function FeatureCard({ icon: Icon, title, desc, color }: Feature) {
  const { ref, rotateX, rotateY, onMouseMove, onMouseLeave } = useTilt();
  return (
    <motion.div
      ref={ref}
      onMouseMove={onMouseMove}
      onMouseLeave={onMouseLeave}
      whileHover={{ scale: 1.02 }}
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ duration: 0.4 }}
      style={{
        rotateX,
        rotateY,
        transformPerspective: 800,
        background: 'rgba(255,255,255,0.03)',
        backdropFilter: 'blur(10px)',
        boxShadow: '0 0 0 1px rgba(255,255,255,0.06) inset',
      } as React.CSSProperties}
      className="rounded-2xl border border-white/10 p-6 cursor-default"
    >
      <div className="w-10 h-10 rounded-xl flex items-center justify-center mb-4" style={{ background: `${color}22` }}>
        <Icon className="w-5 h-5" style={{ color }} />
      </div>
      <h3 className="text-white font-semibold text-sm mb-2">{title}</h3>
      <p className="text-[#64748b] text-sm leading-relaxed">{desc}</p>
    </motion.div>
  );
}

// ── Pricing ───────────────────────────────────────────────────────────────────
const plans = [
  {
    name: 'Free',
    price: '$0',
    period: 'forever',
    desc: 'Try it out with no commitment.',
    features: ['3 applications / day', '2 job boards', 'Review-before-submit', 'Basic matching', '7-day history'],
    cta: 'Get started free',
    gradient: false,
    popular: false,
  },
  {
    name: 'Starter',
    price: '$19',
    period: '/month',
    desc: 'For active job seekers.',
    features: ['15 applications / day', '4 job boards', 'Auto-apply mode', 'AI keyword tailoring', '30-day history', 'Email notifications'],
    cta: 'Start 7-day trial',
    gradient: false,
    popular: true,
  },
  {
    name: 'Pro',
    price: '$49',
    period: '/month',
    desc: 'For power users who need results.',
    features: ['40 applications / day', 'All boards', 'Full AI resume rewrite', 'Resume A/B testing', 'Funnel analytics', 'Slack/Discord alerts', 'Scam detection'],
    cta: 'Start 7-day trial',
    gradient: true,
    popular: false,
  },
];

type Plan = typeof plans[0];

function PricingCard({ plan, delay }: { plan: Plan; delay: number }) {
  const { ref, rotateX, rotateY, onMouseMove, onMouseLeave } = useTilt();
  return (
    <motion.div
      ref={ref}
      style={{
        rotateX,
        rotateY,
        transformPerspective: 1000,
        background: plan.gradient
          ? 'linear-gradient(135deg, rgba(99,102,241,0.15) 0%, rgba(139,92,246,0.15) 100%)'
          : 'rgba(255,255,255,0.03)',
        backdropFilter: 'blur(10px)',
      } as React.CSSProperties}
      onMouseMove={onMouseMove}
      onMouseLeave={onMouseLeave}
      initial={{ opacity: 0, y: 30 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      transition={{ duration: 0.5, delay }}
      className={`relative rounded-2xl p-6 flex flex-col ${plan.gradient ? 'border-indigo-500/40' : 'border-white/10'} border`}
    >
      {plan.popular && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2">
          <span className="text-xs font-semibold text-white px-3 py-1 rounded-full" style={{ background: 'var(--gradient-brand)' }}>Most popular</span>
        </div>
      )}
      <div className="mb-5">
        <p className="text-[#94a3b8] text-sm font-medium mb-1">{plan.name}</p>
        <div className="flex items-baseline gap-1">
          <span className="text-white text-3xl font-bold">{plan.price}</span>
          <span className="text-[#64748b] text-sm">{plan.period}</span>
        </div>
        <p className="text-[#64748b] text-xs mt-1">{plan.desc}</p>
      </div>
      <ul className="space-y-2.5 flex-1 mb-6">
        {plan.features.map((f) => (
          <li key={f} className="flex items-center gap-2.5">
            <Check className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
            <span className="text-[#94a3b8] text-sm">{f}</span>
          </li>
        ))}
      </ul>
      <Link
        href="/signup"
        className={`text-center py-2.5 rounded-xl text-sm font-medium transition-all hover:opacity-90 hover:scale-105 ${
          plan.gradient ? 'text-white' : 'text-white border border-white/20 hover:border-white/40'
        }`}
        style={plan.gradient ? { background: 'var(--gradient-brand)' } : {}}
      >
        {plan.cta}
      </Link>
    </motion.div>
  );
}

// ── Steps ─────────────────────────────────────────────────────────────────────
const steps = [
  { n: '01', title: 'Upload your resume', desc: 'Drop your PDF. We parse it, extract your skills, and build a matching profile.' },
  { n: '02', title: 'Set preferences', desc: 'Choose titles, locations, salary range, remote preference, and companies to skip.' },
  { n: '03', title: 'Review your matches', desc: 'Browse AI-scored job matches. Approve the ones you like — or enable auto-apply.' },
  { n: '04', title: 'We apply for you', desc: 'Our cloud browsers fill and submit each application with your tailored resume and cover letter.' },
  { n: '05', title: 'Track outcomes', desc: 'See every application, status update, and interview invite in your analytics dashboard.' },
];

// ── Main page ─────────────────────────────────────────────────────────────────
export default function LandingPage() {
  return (
    <div className="min-h-screen bg-[#080d1a] text-white overflow-x-hidden">
      <MarketingNavbar />

      {/* ── Hero ────────────────────────────────────────────────────────────── */}
      <section className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden pt-16">
        {/* 3D canvas background */}
        <div className="absolute inset-0 pointer-events-none">
          <HeroScene />
        </div>

        {/* Radial gradient overlay */}
        <div className="absolute inset-0 pointer-events-none" style={{
          background: 'radial-gradient(ellipse 80% 60% at 50% 60%, rgba(99,102,241,0.12) 0%, transparent 70%)',
        }} />

        <div className="relative z-10 max-w-4xl mx-auto px-6 text-center">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 0.1 }}
            className="inline-flex items-center gap-2 bg-white/5 border border-white/10 rounded-full px-4 py-1.5 mb-8"
          >
            <Zap className="w-3 h-3 text-indigo-400" />
            <span className="text-xs text-[#94a3b8]">7-day free Pro trial — no credit card</span>
          </motion.div>

          <motion.h1
            initial={{ opacity: 0, y: 25 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="text-5xl md:text-7xl font-bold leading-[1.08] tracking-tight mb-6"
          >
            Apply to hundreds of<br />
            <span style={{ background: 'var(--gradient-brand)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent', backgroundClip: 'text' }}>
              jobs while you sleep
            </span>
          </motion.h1>

          <motion.p
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 0.35 }}
            className="text-[#94a3b8] text-lg md:text-xl max-w-2xl mx-auto mb-10 leading-relaxed"
          >
            AutoDreamApplier is a cloud-native job automation platform that discovers, matches, and submits applications across every major ATS — so you can focus on interviews, not applications.
          </motion.p>

          <motion.div
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.5 }}
            className="flex flex-col sm:flex-row items-center justify-center gap-3"
          >
            <Link
              href="/signup"
              className="flex items-center gap-2 px-6 py-3 rounded-xl text-white font-medium text-sm hover:opacity-90 hover:scale-105 transition-all shadow-lg"
              style={{ background: 'var(--gradient-brand)', boxShadow: '0 0 24px rgba(99,102,241,0.4)' }}
            >
              Start for free
              <ArrowRight className="w-4 h-4" />
            </Link>
            <a
              href="#how-it-works"
              className="flex items-center gap-2 px-6 py-3 rounded-xl text-[#94a3b8] font-medium text-sm border border-white/10 hover:border-white/20 hover:text-white transition-all"
            >
              See how it works
            </a>
          </motion.div>

          {/* Social proof */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.5, delay: 0.75 }}
            className="mt-12 flex flex-col items-center gap-3"
          >
            <div className="flex -space-x-2">
              {['#6366f1','#8b5cf6','#a78bfa','#c4b5fd','#ddd6fe'].map((c, i) => (
                <div key={i} className="w-8 h-8 rounded-full border-2 border-[#080d1a] flex items-center justify-center text-xs font-bold text-white" style={{ background: c }}>
                  {String.fromCharCode(65 + i)}
                </div>
              ))}
            </div>
            <div className="flex items-center gap-1">
              {[...Array(5)].map((_, i) => (
                <Star key={i} className="w-3.5 h-3.5 fill-yellow-400 text-yellow-400" />
              ))}
            </div>
            <p className="text-[#64748b] text-xs">Trusted by 2,000+ job seekers · 4.9/5 rating</p>
          </motion.div>
        </div>

        {/* Scroll indicator */}
        <motion.div
          animate={{ y: [0, 8, 0] }}
          transition={{ repeat: Infinity, duration: 2, ease: 'easeInOut' }}
          className="absolute bottom-8 left-1/2 -translate-x-1/2"
        >
          <div className="w-5 h-8 rounded-full border border-white/20 flex items-start justify-center pt-1.5">
            <div className="w-1 h-2 rounded-full bg-white/40" />
          </div>
        </motion.div>
      </section>

      {/* ── Stats bar ───────────────────────────────────────────────────────── */}
      <section className="py-12 border-y border-white/10 bg-white/[0.02]">
        <div className="max-w-5xl mx-auto px-6 grid grid-cols-2 md:grid-cols-4 gap-8 text-center">
          {[
            { value: '2,000+', label: 'Active users' },
            { value: '180,000+', label: 'Applications sent' },
            { value: '94%', label: 'Submission success rate' },
            { value: '8 ATS', label: 'Platforms supported' },
          ].map(({ value, label }, i) => (
            <motion.div
              key={label}
              initial={{ opacity: 0, y: 10 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.4, delay: i * 0.08 }}
            >
              <p className="text-2xl md:text-3xl font-bold text-white mb-1">{value}</p>
              <p className="text-[#64748b] text-sm">{label}</p>
            </motion.div>
          ))}
        </div>
      </section>

      {/* ── Features ────────────────────────────────────────────────────────── */}
      <section id="features" className="py-24 max-w-7xl mx-auto px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="text-center mb-14"
        >
          <span className="text-xs font-semibold uppercase tracking-widest text-indigo-400 mb-3 block">Features</span>
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">Everything you need to land your next role</h2>
          <p className="text-[#94a3b8] text-lg max-w-2xl mx-auto">From discovery to offer — our full automation stack handles the tedious parts so you don&apos;t have to.</p>
        </motion.div>
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
          {features.map((f) => <FeatureCard key={f.title} {...f} />)}
        </div>
      </section>

      {/* ── How it works ────────────────────────────────────────────────────── */}
      <section id="how-it-works" className="py-24 bg-[#0a0f1e]">
        <div className="max-w-4xl mx-auto px-6">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            className="text-center mb-14"
          >
            <span className="text-xs font-semibold uppercase tracking-widest text-indigo-400 mb-3 block">How it works</span>
            <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">Up and running in minutes</h2>
          </motion.div>
          <div className="space-y-6">
            {steps.map((step, i) => (
              <motion.div
                key={step.n}
                initial={{ opacity: 0, x: -20 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.4, delay: i * 0.08 }}
                className="flex gap-5 items-start"
              >
                <div className="shrink-0 w-12 h-12 rounded-xl border border-white/10 flex items-center justify-center text-xs font-bold text-indigo-400" style={{ background: 'rgba(99,102,241,0.08)' }}>
                  {step.n}
                </div>
                <div className="pt-1">
                  <p className="text-white font-semibold text-sm mb-1">{step.title}</p>
                  <p className="text-[#64748b] text-sm leading-relaxed">{step.desc}</p>
                </div>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Pricing ─────────────────────────────────────────────────────────── */}
      <section id="pricing" className="py-24 max-w-6xl mx-auto px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="text-center mb-14"
        >
          <span className="text-xs font-semibold uppercase tracking-widest text-indigo-400 mb-3 block">Pricing</span>
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">Simple, transparent pricing</h2>
          <p className="text-[#94a3b8] text-lg">Start free, upgrade when you&apos;re ready. Cancel anytime.</p>
        </motion.div>
        <div className="grid md:grid-cols-3 gap-6">
          {plans.map((plan, i) => <PricingCard key={plan.name} plan={plan} delay={i * 0.1} />)}
        </div>
        <p className="text-center text-[#475569] text-xs mt-6">Annual billing saves ~17%. Enterprise plan available for teams — <a href="#contact" className="text-indigo-400 hover:underline">contact us</a>.</p>
      </section>

      {/* ── FAQ ─────────────────────────────────────────────────────────────── */}
      <FAQ />

      {/* ── Contact ─────────────────────────────────────────────────────────── */}
      <Contact />

      {/* ── CTA banner ──────────────────────────────────────────────────────── */}
      <section className="py-24 max-w-4xl mx-auto px-6 text-center">
        <motion.div
          initial={{ opacity: 0, scale: 0.96 }}
          whileInView={{ opacity: 1, scale: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5 }}
          className="rounded-3xl p-12 border border-indigo-500/30 relative overflow-hidden"
          style={{ background: 'linear-gradient(135deg, rgba(99,102,241,0.12) 0%, rgba(139,92,246,0.12) 100%)' }}
        >
          <div className="absolute inset-0 pointer-events-none" style={{ background: 'radial-gradient(circle at 50% 0%, rgba(99,102,241,0.2) 0%, transparent 60%)' }} />
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4 relative z-10">
            Ready to automate your job search?
          </h2>
          <p className="text-[#94a3b8] text-lg mb-8 relative z-10">Join 2,000+ job seekers already using AutoDreamApplier. No credit card required.</p>
          <Link
            href="/signup"
            className="inline-flex items-center gap-2 px-8 py-3.5 rounded-xl text-white font-medium text-sm hover:opacity-90 hover:scale-105 transition-all relative z-10"
            style={{ background: 'var(--gradient-brand)', boxShadow: '0 0 32px rgba(99,102,241,0.5)' }}
          >
            Start your free trial
            <ArrowRight className="w-4 h-4" />
          </Link>
        </motion.div>
      </section>

      <Footer />
    </div>
  );
}
