import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright configuration for rela frontend e2e tests.
 *
 * Architecture:
 * - Vite dev server: Started via webServer config (shared across all tests)
 * - Backend servers: Started per-test via fixtures (isolated, random ports)
 * - API routing: page.route() intercepts /api/* and routes to test's backend
 *
 * Run with: npm run test:e2e
 */

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Start Vite dev server before tests (shared across all tests)
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
})
