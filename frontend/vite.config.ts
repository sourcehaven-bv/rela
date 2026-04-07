import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Get API base URL from environment variable or default to localhost:8080
// For e2e tests, VITE_API_BASE is set by global-setup.ts
const apiBase = process.env.VITE_API_BASE || 'http://localhost:8080'

export default defineConfig(() => {
  // Log at config evaluation time so we can see what port is being used
  console.error(`[vite.config] API proxy target: ${apiBase}`)

  return {
    plugins: [vue()],
    base: '/',
    build: {
      outDir: '../internal/dataentry/static/v2',
      emptyOutDir: true,
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: apiBase,
          changeOrigin: true,
        },
      },
    },
  }
})
