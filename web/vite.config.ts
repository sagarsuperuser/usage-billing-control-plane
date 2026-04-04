import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import path from "path";

/**
 * Serves /runtime-config as JSON for dev and tests.
 * In production, the Docker entrypoint injects window.__RUNTIME_CONFIG__
 * into index.html — no server-side handler needed.
 */
function runtimeConfigPlugin(): Plugin {
  return {
    name: "runtime-config",
    configureServer(server) {
      server.middlewares.use("/runtime-config", (_req, res) => {
        res.setHeader("Content-Type", "application/json");
        res.setHeader("Cache-Control", "no-store");
        res.end(JSON.stringify({
          apiBaseURL: process.env.VITE_API_BASE_URL?.trim() || "",
        }));
      });
    },
    configurePreviewServer(server) {
      server.middlewares.use("/runtime-config", (_req, res) => {
        res.setHeader("Content-Type", "application/json");
        res.setHeader("Cache-Control", "no-store");
        res.end(JSON.stringify({
          apiBaseURL: process.env.VITE_API_BASE_URL?.trim() || "",
        }));
      });
    },
  };
}

export default defineConfig({
  plugins: [
    TanStackRouterVite({
      routesDirectory: "./src/routes",
      generatedRouteTree: "./src/routeTree.gen.ts",
      quoteStyle: "double",
      enableRouteGeneration: true,
    }),
    react(),
    runtimeConfigPlugin(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    host: "127.0.0.1",
  },
  preview: {
    port: 3000,
    host: "127.0.0.1",
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
