import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Bind to loopback only: the real UI will control Docker (threat model T4).
export default defineConfig({
  plugins: [react()],
  server: { host: "127.0.0.1", strictPort: true },
  preview: { host: "127.0.0.1", strictPort: true },
});
