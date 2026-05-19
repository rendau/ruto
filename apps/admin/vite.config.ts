import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const port = Number(env.VITE_PORT || "5173");
  const host = env.VITE_HOST || "127.0.0.1";

  return {
    plugins: [vue()],
    server: {
      host,
      port: Number.isFinite(port) && port > 0 ? port : 5173
    },
    preview: {
      host,
      port: Number.isFinite(port) && port > 0 ? port : 5173
    }
  };
});
