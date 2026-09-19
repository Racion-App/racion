import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  css: {
    preprocessorOptions: {
      scss: {
        // Bootstrap 5.3 и дизайн-система написаны на @import — глушим предупреждения dart-sass.
        silenceDeprecations: ["import", "global-builtin", "color-functions", "legacy-js-api"],
        quietDeps: true,
      },
    },
  },
  server: {
    port: 5173,
    proxy: { "/api": "http://localhost:8080", "/healthz": "http://localhost:8080", "/recipes": "http://localhost:8080", "/recipe": "http://localhost:8080", "/en/recipes": "http://localhost:8080", "/en/recipe": "http://localhost:8080", "/de/recipes": "http://localhost:8080", "/de/recipe": "http://localhost:8080", "/sitemap.xml": "http://localhost:8080", "/robots.txt": "http://localhost:8080" },
  },
  build: {
    sourcemap: false,
    rollupOptions: {
      output: {
        // Стабильные имена: страницы рецептов с бэкенда подключают /assets/app.css напрямую.
        entryFileNames: "assets/app.js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: (info) => (info.name && info.name.endsWith(".css") ? "assets/app.css" : "assets/[name][extname]"),
      },
    },
  },
});
