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
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.{ts,vue}'],
      exclude: [
        'src/test/**',
        'src/**/*.d.ts',
        'src/**/*.test.ts',
        'src/**/*.spec.ts',
        'src/main.ts',
        // Vue components are tested via e2e tests
        'src/views/**',
        'src/components/**',
        'src/App.vue',
        // Router config - tested via e2e
        'src/router/**',
        // API layer - thin axios wrappers, tested via e2e
        'src/api/**',
        // Re-export barrel files - no testable logic
        'src/types/index.ts',
        'src/composables/index.ts',
      ],
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
