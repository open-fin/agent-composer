import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// During development the Vite server proxies /api, /healthz and /readyz to
// composer-server, so the browser sees a single origin and no CORS is involved.
// In the compose stack nginx does the same job.
//
// 8088 rather than 8080: the latter is commonly already taken on a developer machine.
const API_TARGET = 'http://localhost:8088'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: API_TARGET, changeOrigin: true },
      '/healthz': { target: API_TARGET, changeOrigin: true },
      '/readyz': { target: API_TARGET, changeOrigin: true },
    },
  },
  build: { outDir: 'dist', sourcemap: false },
})
