import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: { timeout: 5000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // 2, not 4, on the 4-core CI runner. Raising it to 4 was tried and reverted
  // (TKT-FP04ZE). It measured ~50s faster, and nothing has been shown to be
  // wrong with it — the E2E failure it was first blamed for turned out to be a
  // stale branch, not contention.
  //
  // What makes 4 unproven rather than fine: per-TEST state is isolated (an
  // ephemeral port, an mkdtemp project dir, a `relae2e_<pid>_<n>` postgres
  // schema), but the CPU is not, and document `command:` renderers fork a
  // bubblewrap sandbox per request through cmdexec, which FAILS CLOSED. Unlike
  // internal/transform, which bounds concurrent conversions at 4,
  // internal/dataentry builds that runner per request with no
  // WithMaxConcurrent (TKT-LP4EE8) — so nothing caps the aggregate.
  //
  // Bound it there first, then raise this. Note `retries: 2` would mask
  // load-induced flake as a slow green, so judge a change here on repeated
  // runs, not one.
  workers: process.env.CI ? 2 : undefined,
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
