import { defineConfig } from 'vitest/config'
import { fileURLToPath, URL } from 'node:url'

// The contract tests compare the fake data-entry API with a real rela-server
// (see contract/globalSetup.ts). They talk to a network server, so they run
// apart from the unit suite, without src/test/setup.ts and its no-network
// axios adapter.
export default defineConfig({
  test: {
    environment: 'happy-dom',
    include: ['contract/**/*.contract.test.ts'],
    globalSetup: ['./contract/globalSetup.ts'],
    // 200 runs take about a minute locally; a slow CI runner gets headroom.
    testTimeout: 600_000,
    hookTimeout: 120_000,
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
