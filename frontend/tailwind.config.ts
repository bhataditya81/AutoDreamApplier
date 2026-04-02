import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Brand colours (indigo-based)
        brand: {
          50:  "#eef2ff",
          100: "#e0e7ff",
          200: "#c7d2fe",
          300: "#a5b4fc",
          400: "#818cf8",
          500: "#6366f1",
          600: "#4f46e5",
          700: "#4338ca",
          800: "#3730a3",
          900: "#312e81",
        },
        // Sidebar tokens
        sidebar: {
          bg:         '#0f172a',
          surface:    '#1e293b',
          border:     '#334155',
          text:       '#94a3b8',
          textActive: '#f1f5f9',
        },
        // Accent gradient tokens
        accent: {
          from: '#6366f1',
          to:   '#8b5cf6',
        },
        // Semantic surface tokens
        surface: {
          DEFAULT:   '#ffffff',
          secondary: '#f8fafc',
          tertiary:  '#f1f5f9',
          overlay:   'rgba(15,23,42,0.04)',
        },
        border: "#e5e7eb",
      },
      fontFamily: {
        sans: ["var(--font-inter)", "system-ui", "sans-serif"],
      },
      borderRadius: {
        lg: "0.625rem",
        xl: "0.875rem",
        "2xl": "1.125rem",
      },
      boxShadow: {
        card: "0 1px 3px 0 rgba(0,0,0,.06), 0 1px 2px -1px rgba(0,0,0,.06)",
        "card-hover": '0 8px 24px rgba(99,102,241,.12), 0 2px 8px rgba(0,0,0,.06)',
        "card-glass": '0 8px 32px rgba(99,102,241,.10), inset 0 1px 0 rgba(255,255,255,.8)',
        "glow-indigo": '0 0 20px rgba(99,102,241,.35)',
        "sidebar-item": 'inset 0 0 0 1px rgba(99,102,241,.25)',
        dialog: "0 20px 25px -5px rgba(0,0,0,.1), 0 8px 10px -6px rgba(0,0,0,.1)",
      },
      keyframes: {
        "slide-in": {
          from: { transform: "translateX(-100%)", opacity: "0" },
          to:   { transform: "translateX(0)",    opacity: "1" },
        },
        "fade-in": {
          from: { opacity: "0" },
          to:   { opacity: "1" },
        },
        "progress-indeterminate": {
          "0%":   { transform: "translateX(-100%)" },
          "100%": { transform: "translateX(400%)" },
        },
        shimmer: {
          '0%':   { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        'pulse-ring': {
          '0%,100%': { opacity: '0.6', transform: 'scale(1)' },
          '50%':     { opacity: '0.2', transform: 'scale(1.5)' },
        },
        float: {
          '0%,100%': { transform: 'translateY(0)' },
          '50%':     { transform: 'translateY(-4px)' },
        },
        'slide-up-fade': {
          from: { opacity: '0', transform: 'translateY(12px)' },
          to:   { opacity: '1', transform: 'translateY(0)' },
        },
        'scale-in': {
          from: { opacity: '0', transform: 'scale(0.92)' },
          to:   { opacity: '1', transform: 'scale(1)' },
        },
        'gradient-shift': {
          '0%,100%': { backgroundPosition: '0% 50%' },
          '50%':     { backgroundPosition: '100% 50%' },
        },
      },
      animation: {
        "slide-in": "slide-in 0.2s ease-out",
        "fade-in":  "fade-in  0.15s ease-out",
        "progress-indeterminate": "progress-indeterminate 1.5s ease-in-out infinite",
        shimmer:          'shimmer 1.8s linear infinite',
        'pulse-ring':     'pulse-ring 2s ease-in-out infinite',
        float:            'float 3s ease-in-out infinite',
        'slide-up-fade':  'slide-up-fade 0.22s cubic-bezier(0.16,1,0.3,1) forwards',
        'scale-in':       'scale-in 0.22s cubic-bezier(0.16,1,0.3,1) forwards',
        'gradient-shift': 'gradient-shift 4s ease infinite',
      },
    },
  },
  plugins: [],
};

export default config;
