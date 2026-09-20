import { defineConfig, loadEnv, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

// Метка сборки: вшивается в бандл (__BUILD__) и пишется в dist/version.json. Приложение сверяет их и,
// если на сервере версия новее, предлагает обновиться — иначе PWA на телефоне может неделями жить на старом коде.
const BUILD = loadEnv("production", ".", "VITE_").VITE_BUILD || new Date().toISOString().slice(0, 16).replace("T", " ") + " UTC";
const versionFile = (): Plugin => ({
  name: "racion-version",
  apply: "build",
  generateBundle() {
    this.emitFile({ type: "asset", fileName: "version.json", source: JSON.stringify({ build: BUILD }) });
  },
});

export default defineConfig({
  plugins: [react(), versionFile()],
  define: { __BUILD__: JSON.stringify(BUILD) },
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
    assetsInlineLimit: 0, // флаги flag-icons — отдельными SVG по запросу, а не 400 КБ data-URI в app.css

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
