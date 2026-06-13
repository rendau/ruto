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
    build: {
      chunkSizeWarningLimit: 1500,
      rollupOptions: {
        output: {
          manualChunks: {
            vue: ["vue", "vue-router", "pinia"],
            "naive-ui": ["naive-ui", "@vicons/ionicons5"]
          }
        }
      }
    }
  };
});
