import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        // GitHub Dark palette, named so the components read clearly.
        ink: {
          50:  "#f0f6fc",
          100: "#c9d1d9",
          200: "#8b949e",
          300: "#6e7681",
          400: "#484f58",
          500: "#30363d",
          600: "#21262d",
          700: "#161b22",
          800: "#0d1117",
          900: "#010409",
        },
        accent: {
          blue:   "#58a6ff",
          green:  "#3fb950",
          red:    "#f85149",
          yellow: "#d29922",
          purple: "#bc8cff",
        },
      },
      fontFamily: {
        mono: ['ui-monospace', '"SF Mono"', 'Menlo', 'Consolas', 'monospace'],
        sans: ['-apple-system', 'system-ui', '"Segoe UI"', 'sans-serif'],
      },
      animation: {
        "pulse-fast": "pulse 1s cubic-bezier(0.4, 0, 0.6, 1) infinite",
      },
    },
  },
  plugins: [],
} satisfies Config;
