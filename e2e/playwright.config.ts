import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: { timeout: 5000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // 4 workers on a 4-core runner; 2 left half the cores idle.
  //
  // Isolated per test: an ephemeral port (with spawn retry, since picking a
  // free port and binding it is inherently racy), an mkdtemp project dir, and
  // a `relae2e_<pid>_<n>` postgres schema — unique because Playwright runs each
  // worker as a separate OS process.
  //
  // NOT isolated: the single postgres service container (every worker's server
  // opens its own pool against it), and the runner's CPU — 4 workers means
  // roughly 8 busy processes (Chromium + rela-server each) on 4 cores, against
  // the 5s action / 10s navigation timeouts below. If flake rises, suspect this
  // first, and note `retries: 2` will hide it as a slow green rather than a
  // failure.
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
