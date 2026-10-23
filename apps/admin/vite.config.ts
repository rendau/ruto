import { fileURLToPath, URL } from "node:url";
import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const port = Number(env.VITE_PORT || "5173");
  const host = env.VITE_HOST || "0.0.0.0";

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        "@": fileURLToPath(new URL("./src", import.meta.url))
      }
    },
    server: {
      host,
      port: Number.isFinite(port) && port > 0 ? port : 5173
    },
    preview: {
      host,
      port: Number.isFinite(port) && port > 0 ? port : 5173
    },
    // One js and one css file: every route is imported statically and nothing is
    // split off or preloaded, so the admin loads in a single round of requests.
    build: {
      chunkSizeWarningLimit: 3000,
      cssCodeSplit: false,
      modulePreload: false,
      rollupOptions: {
        output: {
          inlineDynamicImports: true
        }
      }
    }
  };
});
