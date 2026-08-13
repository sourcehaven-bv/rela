import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  // Mirror the vite `define` so components referencing the compile-time
  // __E2E_TEST_HOOKS__ flag don't ReferenceError under vitest. Off in unit
  // tests — test hooks are an E2E concern (issue #890).
  define: {
    __E2E_TEST_HOOKS__: 'false',
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{js,ts,vue}'],
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
