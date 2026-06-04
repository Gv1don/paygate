import react from "@vitejs/plugin-react"
import { resolve } from "path"
import { defineConfig } from "vite"

export default defineConfig({
  plugins: [react()],
  root: "src",
  build: {
    outDir: resolve(__dirname, "dist"),
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "react-native": "react-native-web",
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8081",
      "/health": "http://localhost:8081",
    },
  },
})
