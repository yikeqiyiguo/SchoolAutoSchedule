import { defineConfig } from "vite";

// 开发时通过代理访问 Go 后端（默认 127.0.0.1:8000），避免跨域
export default defineConfig({
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8000",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    target: "es2022",
  },
});
