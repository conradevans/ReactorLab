import { defineConfig, loadEnv } from "vite"
import react from "@vitejs/plugin-react"

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "")

  return {
    plugins: [react()],
    server: {
      host: "127.0.0.1",
      port: 4173,
      strictPort: true,
      proxy: {
        "/api": env.REACTORLAB_API_TARGET || "http://127.0.0.1:9200",
        "/health": env.REACTORLAB_API_TARGET || "http://127.0.0.1:9200",
      },
    },
  }
})
