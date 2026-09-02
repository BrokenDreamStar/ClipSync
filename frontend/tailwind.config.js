/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,js}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        bg: '#242321',
        surface: '#2E2D2B',
        elevated: '#3A3936',
        border: '#3C3B38',
        text: '#E9E7E2',
        muted: '#8E8C87',
        accent: '#2DD4BF',
        'accent-hover': '#5EEAD4',
        'accent-2': '#818CF8',
        success: '#34D399',
        danger: '#F87171',
        warning: '#FBBF24',
      },
      fontFamily: {
        sans: ['"PingFang SC"', '"Microsoft YaHei UI"', '"Helvetica Neue"', 'system-ui', 'sans-serif'],
        mono: ['"SF Mono"', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 0 rgba(255,255,255,0.03) inset, 0 10px 34px rgba(0,0,0,0.38)',
        pop: '0 12px 44px rgba(0,0,0,0.52)',
        glow: '0 6px 24px rgba(45,212,191,0.22)',
        'glow-indigo': '0 6px 24px rgba(129,140,248,0.22)',
      },
      keyframes: {
        'pulse-soft': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '.35' },
        },
        'fade-up': {
          from: { opacity: '0', transform: 'translateY(8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '200% 0' },
          '100%': { backgroundPosition: '-200% 0' },
        },
      },
      animation: {
        'pulse-soft': 'pulse-soft 2.4s ease-in-out infinite',
        'fade-up': 'fade-up .38s ease both',
        shimmer: 'shimmer 2.2s linear infinite',
      },
    },
  },
  plugins: [],
}
