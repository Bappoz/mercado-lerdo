import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Bind to loopback only: the UI controls Docker (threat model T4). In dev,
// `sesim ui --addr 127.0.0.1:7777 --no-open` serves the API and vite proxies it.
export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    strictPort: true,
    proxy: {
      // The server accepts only its own Host and Origin (T2, T3).
      "/api": { target: "http://127.0.0.1:7777", changeOrigin: true },
      "/ws": {
        target: "ws://127.0.0.1:7777",
        ws: true,
        changeOrigin: true,
        headers: { origin: "http://127.0.0.1:7777" },
      },
    },
  },
  preview: { host: "127.0.0.1", strictPort: true },
  build: { assetsInlineLimit: 0 },
});
