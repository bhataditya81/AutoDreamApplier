import Link from 'next/link';
import { Zap, Twitter, Github, Linkedin } from 'lucide-react';

const sections = [
  {
    title: 'Product',
    links: [
      { label: 'Features', href: '#features' },
      { label: 'How it works', href: '#how-it-works' },
      { label: 'Pricing', href: '#pricing' },
      { label: 'Changelog', href: '/changelog' },
      { label: 'Roadmap', href: '/roadmap' },
    ],
  },
  {
    title: 'Company',
    links: [
      { label: 'About', href: '/about' },
      { label: 'Blog', href: '/blog' },
      { label: 'Careers', href: '/careers' },
      { label: 'Press kit', href: '/press' },
    ],
  },
  {
    title: 'Legal',
    links: [
      { label: 'Privacy Policy', href: '/privacy' },
      { label: 'Terms of Service', href: '/terms' },
      { label: 'Cookie Policy', href: '/cookies' },
      { label: 'GDPR', href: '/gdpr' },
    ],
  },
  {
    title: 'Support',
    links: [
      { label: 'Documentation', href: '/docs' },
      { label: 'Status page', href: '/status' },
      { label: 'Contact us', href: '#contact' },
      { label: 'Discord community', href: '/discord' },
    ],
  },
];

export default function Footer() {
  return (
    <footer className="bg-[#080d1a] border-t border-white/10">
      <div className="max-w-7xl mx-auto px-6 py-16">
        <div className="grid grid-cols-2 md:grid-cols-6 gap-10 pb-12 border-b border-white/10">
          {/* Brand */}
          <div className="col-span-2">
            <Link href="/" className="flex items-center gap-2.5 mb-4">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'var(--gradient-brand)' }}>
                <Zap className="w-4 h-4 text-white" />
              </div>
              <span className="text-white font-semibold text-sm tracking-tight">AutoDream</span>
              <span className="text-[#6366f1] font-semibold text-sm">Applier</span>
            </Link>
            <p className="text-[#64748b] text-sm leading-relaxed max-w-xs mb-6">
              Cloud-native job application automation. Apply smarter, not harder.
            </p>
            <div className="flex items-center gap-3">
              {[
                { Icon: Twitter, href: 'https://twitter.com' },
                { Icon: Github, href: 'https://github.com' },
                { Icon: Linkedin, href: 'https://linkedin.com' },
              ].map(({ Icon, href }) => (
                <a
                  key={href}
                  href={href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="w-8 h-8 rounded-lg border border-white/10 flex items-center justify-center text-[#64748b] hover:text-white hover:border-white/20 transition"
                >
                  <Icon className="w-4 h-4" />
                </a>
              ))}
            </div>
          </div>

          {/* Links */}
          {sections.map((s) => (
            <div key={s.title}>
              <p className="text-white text-xs font-semibold uppercase tracking-widest mb-4">{s.title}</p>
              <ul className="space-y-2.5">
                {s.links.map((l) => (
                  <li key={l.label}>
                    <Link href={l.href} className="text-[#64748b] hover:text-[#94a3b8] text-sm transition-colors">
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="pt-8 flex flex-col md:flex-row items-center justify-between gap-4">
          <p className="text-[#475569] text-xs">© 2026 AutoDreamApplier. All rights reserved.</p>
          <div className="flex items-center gap-6">
            <span className="flex items-center gap-1.5 text-xs text-[#475569]">
              <span className="w-1.5 h-1.5 rounded-full bg-green-500 inline-block" />
              All systems operational
            </span>
            <Link href="/privacy" className="text-[#475569] hover:text-[#94a3b8] text-xs transition-colors">Privacy</Link>
            <Link href="/terms" className="text-[#475569] hover:text-[#94a3b8] text-xs transition-colors">Terms</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
