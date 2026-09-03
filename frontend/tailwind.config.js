/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,js}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主题色经由 CSS 变量注入（见 style.css 的 :root / html.light 两套定义），
        // 用「RGB 三元组 + <alpha-value>」以保留 bg-elevated/60 等透明度修饰符能力。
        bg: 'rgb(var(--c-bg-rgb) / <alpha-value>)',
        surface: 'rgb(var(--c-surface-rgb) / <alpha-value>)',
        elevated: 'rgb(var(--c-elevated-rgb) / <alpha-value>)',
        border: 'rgb(var(--c-border-rgb) / <alpha-value>)',
        text: 'rgb(var(--c-text-rgb) / <alpha-value>)',
        muted: 'rgb(var(--c-muted-rgb) / <alpha-value>)',
        accent: 'rgb(var(--c-accent-rgb) / <alpha-value>)',
        'accent-hover': 'rgb(var(--c-accent-hover-rgb) / <alpha-value>)',
        'accent-2': 'rgb(var(--c-accent-2-rgb) / <alpha-value>)',
        success: 'rgb(var(--c-success-rgb) / <alpha-value>)',
        danger: 'rgb(var(--c-danger-rgb) / <alpha-value>)',
        warning: 'rgb(var(--c-warning-rgb) / <alpha-value>)',
        // 半透明白/黑覆盖层的主题化替身：深色下用白系叠加，浅色下翻转为深色系叠加。
        scrim: 'var(--c-scrim)',
        line: 'var(--c-line)',
        hairline: 'var(--c-hairline)',
        overlay: 'var(--c-overlay)',
        'overlay-soft': 'var(--c-overlay-soft)',
        'overlay-hover': 'var(--c-overlay-hover)',
        'overlay-strong': 'var(--c-overlay-strong)',
      },
      fontFamily: {
        sans: ['"PingFang SC"', '"Microsoft YaHei UI"', '"Helvetica Neue"', 'system-ui', 'sans-serif'],
        mono: ['"SF Mono"', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        card: 'var(--shadow-card)',
        pop: 'var(--shadow-pop)',
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
