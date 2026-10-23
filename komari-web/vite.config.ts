import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { visualizer } from "rollup-plugin-visualizer";
import { VitePWA } from "vite-plugin-pwa";

// https://vite.dev/config/
import type { Plugin, UserConfig } from "vite";
import * as fs from "fs";
import * as path from "path";
import dotenv from "dotenv";
import { createRequire } from "node:module";
const filterGuestLocales = createRequire(import.meta.url)("../script/guest-locales.cjs");

function localKomariThemePlugin(): Plugin {
  const themeRequestPath = "/themes/default/komari-theme.json";
  const localThemeFile = path.resolve(__dirname, "komari-theme.json");

  return {
    name: "local-komari-theme",
    apply: "serve",
    enforce: "pre",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (!req.url) return next();

        const url = new URL(req.url, "http://localhost");
        if (!url.pathname.endsWith(themeRequestPath)) return next();

        fs.readFile(localThemeFile, (err, data) => {
          if (err) {
            res.statusCode = 404;
            res.setHeader("Content-Type", "application/json; charset=utf-8");
            res.end(
              JSON.stringify({
                error: "Local theme file not found",
                file: localThemeFile,
              })
            );
            return;
          }

          res.statusCode = 200;
          res.setHeader("Content-Type", "application/json; charset=utf-8");
          res.setHeader("Cache-Control", "no-store");
          res.end(data);
        });
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  const guest = mode === "guest";
  const buildTime = new Date().toISOString();

  // Supports configuring BASE_URL via environment variables, defaulting to the root path.
  const base: string = guest ? "/share-assets/default/" : (process.env.VITE_BASE_URL || (mode === "development" ? "/" : "/private-assets/default/"));
  const baseConfig: UserConfig = {
    base: base,
    plugins: [
      localKomariThemePlugin(),
      react(),
      tailwindcss(),
      ...(guest ? [{
        name: "viewer-locales",
        enforce: "pre" as const,
        transform(code: string, id: string) {
          if (/\/i18n\/locales\/[^/]+\.json$/.test(id.replaceAll("\\", "/"))) return JSON.stringify(filterGuestLocales(JSON.parse(code)));
          if (id.includes("/src/") && /\.tsx?$/.test(id)) return code.replaceAll("/api/", "/viewer-api/").replace(/(["'`])\/instance\//g, "$1/shared/instance/");
        },
        generateBundle(_options: unknown, bundle: Record<string, unknown>) {
          for (const item of Object.values(bundle)) {
            const chunk = item as { type?: string; moduleIds?: string[] };
            for (const id of chunk.moduleIds || []) {
              if (/\/src\/(?:pages\/(?:admin|terminal)|contexts\/AccountContext|components\/(?:Login|ChartTemplateAction)|utils\/loginRedirect)/.test(id.replaceAll("\\", "/"))) throw new Error(`Privileged module reached viewer bundle: ${id}`);
            }
          }
        },
      }] : []),
      ...(!guest && mode === "development" ? [VitePWA({
        registerType: "autoUpdate",
        includeAssets: ["favicon.ico", "assets/pwa-icon.webp"],
        manifest: {
          name: "Komari Monitor",
          short_name: "Komari Monitor",
          description: "A simple server monitor tool",
          theme_color: "#2563eb",
          background_color: "#ffffff",
          display: "standalone",
          scope: base,
          start_url: base,
          icons: [
            {
              src: `${base}assets/pwa-icon.webp`,
              sizes: "192x192",
              type: "image/webp",
              purpose: "maskable any",
            },
            {
              src: `${base}assets/pwa-icon.webp`,
              sizes: "512x512",
              type: "image/webp",
              purpose: "maskable any",
            },
          ],
        },
        workbox: {
          // HTML is rendered dynamically with theme, plugin, and site settings.
          // Cache only immutable assets so every navigation reaches the server.
          globPatterns: ["**/*.{js,css,ico,png,svg}"],
          // Linux globs are case-sensitive (e.g. chunk-FileEditorDialog-*.js).
          globIgnores: ["**/*[Mm]onaco*", "**/*[Ee]ditor*", "**/*[Ww]orker*"],
          maximumFileSizeToCacheInBytes: 1024 * 1024,
          navigateFallback: null,
          runtimeCaching: [],
        },
      })] : []),
      visualizer({
        open: false,
        filename: "bundle-analysis.html",
        gzipSize: true,
        brotliSize: true,
      }),
    ],
    define: {
      __BUILD_TIME__: JSON.stringify(buildTime),
    },
      resolve: {
        alias: [
          ...(guest ? [{ find: "@/components/ChartTemplateAction", replacement: path.resolve(__dirname, "src/guest/NoAction.tsx") }] : []),
          { find: "@", replacement: path.resolve(__dirname, "./src") },
          {
            find: /^monaco-editor-codicon\.css$/,
            replacement: path.resolve(
              __dirname,
              "node_modules/monaco-editor/esm/vs/base/browser/ui/codicons/codicon/codicon.css",
            ),
          },
        // Force xterm to use the CJS build to avoid a rollup bug where `||=` in
        // xterm.mjs is incorrectly lowered to `void 0||(i={})` with an undeclared `i`,
        // causing `ReferenceError: i is not defined` at requestMode when vi sends DECRQM sequences.
        // Regex to match only the bare specifier, not subpaths like @xterm/xterm/css/xterm.css.
        { find: /^@xterm\/xterm$/, replacement: path.resolve(__dirname, "node_modules/@xterm/xterm/lib/xterm.js") },
      ],
    },
    build: {
      assetsDir: "assets",
      outDir: guest ? "dist-guest" : "dist",
      chunkSizeWarningLimit: 800,
      rollupOptions: {
        ...(guest ? { input: path.resolve(__dirname, "guest.html") } : {}),
        output: {
          // go embed ignore files start with '_'
          chunkFileNames: "assets/chunk-[name]-[hash].js",
          entryFileNames: "assets/entry-[name]-[hash].js",
          // Do not use manualChunks, use React.lazy() and <Suspense> instead
        }
      },
    },
  };

  if (mode === "development") {
    const envPath = path.resolve(process.cwd(), ".env.development");
    if (fs.existsSync(envPath)) {
      const envConfig = dotenv.parse(fs.readFileSync(envPath));
      for (const k in envConfig) {
        process.env[k] = envConfig[k];
      }
    }
    if (!process.env.VITE_API_TARGET) {
      process.env.VITE_API_TARGET = "http://127.0.0.1:25774";
    }
    baseConfig.server = {
      proxy: {
        "/api": {
          target: process.env.VITE_API_TARGET,
          changeOrigin: true,
          rewriteWsOrigin: true,
          ws: true,
        },
        "/themes": {
          target: process.env.VITE_API_TARGET,
          changeOrigin: true,
        },
      },
    };
  }

  return baseConfig;
});
