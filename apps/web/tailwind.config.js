/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Redesign semantic tokens (styles/tokens.css)
        surface: {
          page: 'hsl(var(--surface-page) / <alpha-value>)',
          base: 'hsl(var(--surface-base) / <alpha-value>)',
          raised: 'hsl(var(--surface-raised) / <alpha-value>)',
          sunken: 'hsl(var(--surface-sunken) / <alpha-value>)',
          overlay: 'hsl(var(--surface-overlay) / <alpha-value>)',
        },
        fg: {
          DEFAULT: 'hsl(var(--fg-primary) / <alpha-value>)',
          secondary: 'hsl(var(--fg-secondary) / <alpha-value>)',
          muted: 'hsl(var(--fg-muted) / <alpha-value>)',
          faint: 'hsl(var(--fg-faint) / <alpha-value>)',
          'on-accent': 'hsl(var(--fg-on-accent) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent) / <alpha-value>)',
          hover: 'hsl(var(--accent-hover) / <alpha-value>)',
          soft: 'hsl(var(--accent-soft) / <alpha-value>)',
          fg: 'hsl(var(--accent-fg) / <alpha-value>)',
        },
        success: {
          DEFAULT: 'hsl(var(--success) / <alpha-value>)',
          soft: 'hsl(var(--success-soft) / <alpha-value>)',
        },
        warning: {
          DEFAULT: 'hsl(var(--warning) / <alpha-value>)',
          soft: 'hsl(var(--warning-soft) / <alpha-value>)',
        },
        danger: {
          DEFAULT: 'hsl(var(--danger) / <alpha-value>)',
          soft: 'hsl(var(--danger-soft) / <alpha-value>)',
        },
        info: {
          DEFAULT: 'hsl(var(--info) / <alpha-value>)',
          soft: 'hsl(var(--info-soft) / <alpha-value>)',
        },
        risk: {
          critical: 'hsl(var(--risk-critical) / <alpha-value>)',
          high: 'hsl(var(--risk-high) / <alpha-value>)',
          medium: 'hsl(var(--risk-medium) / <alpha-value>)',
          low: 'hsl(var(--risk-low) / <alpha-value>)',
          none: 'hsl(var(--risk-none) / <alpha-value>)',
        },
        line: {
          DEFAULT: 'hsl(var(--border-default) / <alpha-value>)',
          strong: 'hsl(var(--border-strong) / <alpha-value>)',
          focus: 'hsl(var(--border-focus) / <alpha-value>)',
        },

        // Legacy aliases (pre-redesign pages)
        border: 'hsl(var(--border))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
      },
      boxShadow: {
        'token-sm': 'var(--shadow-sm)',
        'token-md': 'var(--shadow-md)',
        'token-lg': 'var(--shadow-lg)',
      },
      fontSize: {
        '2xs': ['0.6875rem', { lineHeight: '1rem' }],
      },
    },
  },
  plugins: [],
}
