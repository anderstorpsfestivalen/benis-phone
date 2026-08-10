import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Local dev: forward /api and /config to wrangler dev (8787).
      "/api": { target: "http://localhost:8787", ws: true },
      "/config": { target: "http://localhost:8787", ws: true },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
