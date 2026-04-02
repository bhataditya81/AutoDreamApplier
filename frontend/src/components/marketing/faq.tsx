'use client';
import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown } from 'lucide-react';

const faqs = [
  {
    q: 'How does AutoDreamApplier work?',
    a: 'AutoDreamApplier crawls job boards, matches listings to your profile using AI scoring, then automates the application process through our cloud browser infrastructure — no Chrome extension needed. You review matches, approve them, and we handle the rest.',
  },
  {
    q: 'Is it safe to automate job applications?',
    a: "Yes. We apply only to jobs you've explicitly approved, respect each board's daily rate limits, randomize human-like browser behavior, and use residential IP proxies to keep your accounts safe. You stay in control at all times.",
  },
  {
    q: 'Which job boards and ATS systems do you support?',
    a: 'We currently support Indeed and Glassdoor for discovery, and can submit applications on Greenhouse, Lever, Workday, iCIMS, Taleo, SuccessFactors, SmartRecruiters, and BambooHR. More boards are added regularly.',
  },
  {
    q: 'Will my resume be tailored for each job?',
    a: 'On Starter and above, our AI rewrites your resume keywords to match each job description, and generates a customized cover letter. On Pro we do a full AI rewrite (using Claude) and A/B test resume versions based on your interview outcomes.',
  },
  {
    q: 'What happens if an application fails?',
    a: "Failed applications are logged with a screenshot and error detail so you can see exactly what went wrong. You'll receive a Slack/Discord notification (if configured) and can retry manually or let us retry automatically.",
  },
  {
    q: 'Can I set a daily application limit?',
    a: "Absolutely. You configure your daily limit in Settings and we never exceed it. Each plan tier has its own hard cap as well. Applications are spread across business hours in your timezone to appear natural.",
  },
  {
    q: 'How do I cancel my subscription?',
    a: "Cancel any time from your account settings — no questions asked. You keep access until the end of your billing period. We'll export all your application history so you never lose data.",
  },
  {
    q: 'Is there a free trial?',
    a: "Yes — new signups get a 7-day Pro trial, no credit card required. After the trial you'll move to the Free tier (3 applications/day) unless you choose to upgrade.",
  },
];

export default function FAQ() {
  const [open, setOpen] = useState<number | null>(null);

  return (
    <section id="faq" className="py-24 bg-[#080d1a]">
      <div className="max-w-3xl mx-auto px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5 }}
          className="text-center mb-14"
        >
          <span className="text-xs font-semibold uppercase tracking-widest text-indigo-400 mb-3 block">FAQ</span>
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">Everything you need to know</h2>
          <p className="text-[#94a3b8] text-lg">{"Can't find the answer you're looking for? "}<a href="#contact" className="text-indigo-400 hover:text-indigo-300 underline">Reach out to us.</a></p>
        </motion.div>

        <div className="space-y-3">
          {faqs.map((faq, i) => (
            <motion.div
              key={i}
              initial={{ opacity: 0, y: 10 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.3, delay: i * 0.05 }}
              className="rounded-xl border border-white/10 overflow-hidden"
              style={{ background: 'rgba(255,255,255,0.03)' }}
            >
              <button
                onClick={() => setOpen(open === i ? null : i)}
                className="w-full flex items-center justify-between px-6 py-4 text-left"
              >
                <span className="text-white font-medium text-sm">{faq.q}</span>
                <motion.div animate={{ rotate: open === i ? 180 : 0 }} transition={{ duration: 0.2 }}>
                  <ChevronDown className="w-4 h-4 text-[#64748b] shrink-0" />
                </motion.div>
              </button>
              <AnimatePresence>
                {open === i && (
                  <motion.div
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: 'auto', opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
                  >
                    <p className="px-6 pb-5 text-sm text-[#94a3b8] leading-relaxed">{faq.a}</p>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
