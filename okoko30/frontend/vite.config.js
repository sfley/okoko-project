import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { gzipSync } from "node:zlib";
const precompress = {
  name: "precompress",
  closeBundle() {
    const dir = new URL(
      "../backend/internal/web/dist/assets/",
      import.meta.url,
    );
    for (const name of readdirSync(dir))
      if (/\.(js|css)$/.test(name))
        writeFileSync(
          new URL(name + ".gz", dir),
          gzipSync(readFileSync(new URL(name, dir)), { level: 9 }),
        );
  },
};
export default defineConfig({
  plugins: [react(), precompress],
  build: {
    outDir: "../backend/internal/web/dist",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: { charts: ["recharts"], grid: ["react-grid-layout"] },
      },
    },
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8888",
      "/health": "http://127.0.0.1:8888",
    },
  },
});
