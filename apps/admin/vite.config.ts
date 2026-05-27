import { defineConfig, loadEnv } from "vite";
// @ts-ignore
import vue from "@vitejs/plugin-vue";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const port = Number(env.VITE_PORT || "80");
  const host = env.VITE_HOST || "0.0.0.0";

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
