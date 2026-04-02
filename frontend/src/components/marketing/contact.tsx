'use client';
import { useState } from 'react';
import { motion } from 'framer-motion';
import { Send, Mail, MessageSquare, User } from 'lucide-react';

export default function Contact() {
  const [sent, setSent] = useState(false);
  const [form, setForm] = useState({ name: '', email: '', subject: '', message: '' });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSent(true);
  };

  return (
    <section id="contact" className="py-24 bg-[#0a0f1e]">
      <div className="max-w-5xl mx-auto px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5 }}
          className="text-center mb-14"
        >
          <span className="text-xs font-semibold uppercase tracking-widest text-indigo-400 mb-3 block">Contact</span>
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">Get in touch</h2>
          <p className="text-[#94a3b8] text-lg max-w-xl mx-auto">
            Have a question or want a demo? We typically reply within a few hours.
          </p>
        </motion.div>

        <div className="grid md:grid-cols-2 gap-12 items-start">
          {/* Info */}
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.1 }}
            className="space-y-8"
          >
            {[
              { icon: Mail, title: 'Email us', desc: 'hello@autodreamapplier.com', sub: 'For support & general questions' },
              { icon: MessageSquare, title: 'Live chat', desc: 'Chat in the dashboard', sub: 'Available Mon–Fri 9am–6pm UTC' },
              { icon: User, title: 'Book a demo', desc: '30-min product walkthrough', sub: 'See the full pipeline live' },
            ].map(({ icon: Icon, title, desc, sub }) => (
              <div key={title} className="flex gap-4">
                <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0" style={{ background: 'rgba(99,102,241,0.15)' }}>
                  <Icon className="w-5 h-5 text-indigo-400" />
                </div>
                <div>
                  <p className="text-white font-medium text-sm">{title}</p>
                  <p className="text-indigo-300 text-sm">{desc}</p>
                  <p className="text-[#64748b] text-xs mt-0.5">{sub}</p>
                </div>
              </div>
            ))}
          </motion.div>

          {/* Form */}
          <motion.div
            initial={{ opacity: 0, x: 20 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.15 }}
          >
            {sent ? (
              <motion.div
                initial={{ scale: 0.9, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                className="rounded-2xl border border-green-500/30 bg-green-500/10 p-8 text-center"
              >
                <div className="w-12 h-12 rounded-full bg-green-500/20 flex items-center justify-center mx-auto mb-4">
                  <Send className="w-5 h-5 text-green-400" />
                </div>
                <h3 className="text-white font-semibold mb-2">Message sent!</h3>
                <p className="text-[#94a3b8] text-sm">{"We'll get back to you within a few hours."}</p>
              </motion.div>
            ) : (
              <form onSubmit={handleSubmit} className="rounded-2xl border border-white/10 p-6 space-y-4" style={{ background: 'rgba(255,255,255,0.03)' }}>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs text-[#94a3b8] mb-1.5 font-medium">Name</label>
                    <input
                      value={form.name}
                      onChange={(e) => setForm({ ...form, name: e.target.value })}
                      required
                      placeholder="Jane Smith"
                      className="w-full px-3 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white text-sm placeholder:text-[#475569] focus:outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40 transition"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[#94a3b8] mb-1.5 font-medium">Email</label>
                    <input
                      type="email"
                      value={form.email}
                      onChange={(e) => setForm({ ...form, email: e.target.value })}
                      required
                      placeholder="jane@example.com"
                      className="w-full px-3 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white text-sm placeholder:text-[#475569] focus:outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40 transition"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-[#94a3b8] mb-1.5 font-medium">Subject</label>
                  <input
                    value={form.subject}
                    onChange={(e) => setForm({ ...form, subject: e.target.value })}
                    placeholder="How can we help?"
                    className="w-full px-3 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white text-sm placeholder:text-[#475569] focus:outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40 transition"
                  />
                </div>
                <div>
                  <label className="block text-xs text-[#94a3b8] mb-1.5 font-medium">Message</label>
                  <textarea
                    value={form.message}
                    onChange={(e) => setForm({ ...form, message: e.target.value })}
                    required
                    rows={4}
                    placeholder="Tell us about your job search..."
                    className="w-full px-3 py-2.5 rounded-lg bg-white/5 border border-white/10 text-white text-sm placeholder:text-[#475569] focus:outline-none focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/40 transition resize-none"
                  />
                </div>
                <button
                  type="submit"
                  className="w-full py-2.5 rounded-lg text-white text-sm font-medium flex items-center justify-center gap-2 hover:opacity-90 transition-opacity"
                  style={{ background: 'var(--gradient-brand)' }}
                >
                  <Send className="w-4 h-4" />
                  Send message
                </button>
              </form>
            )}
          </motion.div>
        </div>
      </div>
    </section>
  );
}
