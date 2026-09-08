import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const target = env.RC_MONITOR_DEV_PROXY || "http://127.0.0.1:18100";
  return {
    plugins: [react()],
    server: {
      host: "127.0.0.1",
      port: 4173,
      strictPort: true,
      proxy: {
        "/api": { target, changeOrigin: false },
        "/healthz": { target, changeOrigin: false },
        "/readyz": { target, changeOrigin: false }
      }
    }
  };
});
