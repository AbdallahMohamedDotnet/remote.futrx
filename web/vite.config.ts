import { defineConfig } from "vite";
import preact from "@preact/preset-vite";
import { resolve } from "node:path";

// Vite owns /opt/term-multi-go/public/ at build time.
// Go's //go:embed public picks it up at the next `go build`.
export default defineConfig({
  plugins: [preact()],
  resolve: {
    alias: {
      // react-markdown imports from "react" / "react-dom"; route to preact/compat.
      react: "preact/compat",
      "react-dom": "preact/compat",
      "react/jsx-runtime": "preact/jsx-runtime",
    },
  },
  build: {
    outDir: resolve(__dirname, "../public"),
    emptyOutDir: true,
    sourcemap: false,
    target: "es2020",
    chunkSizeWarningLimit: 600,
  },
  server: {
    port: 5174,
    proxy: {
      // Dev: Vite serves the SPA, Go (on :7682 locally) handles API + WS.
      "/api": "http://127.0.0.1:7682",
      "/ws": { target: "ws://127.0.0.1:7682", ws: true },
    },
  },
});
