import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: { timeout: 5000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // The GitHub runner has 4 cores; 2 workers left half of them idle. Raising
  // this is safe because every shared resource is already per-worker: the
  // fixture spawns rela-server on a free ephemeral port, mkdtemps its project
  // dir, and names each postgres schema `relae2e_<pid>_<n>` — which is unique
  // precisely because Playwright runs each worker as a separate OS process.
  workers: process.env.CI ? 4 : undefined,
  reporter: process.env.CI
    ? [['line'], ['html', { open: 'never' }]]
    : [['list'], ['html', { open: 'never' }]],
  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: 5000,
    navigationTimeout: 10000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
