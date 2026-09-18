import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  expect: { timeout: 5000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // 2, not 4, on the 4-core CI runner. Raising it to 4 was tried and reverted.
  //
  // Per-TEST state is isolated — an ephemeral port, an mkdtemp project dir, and
  // a `relae2e_<pid>_<n>` postgres schema — but the runner's CPU is not. Four
  // workers means roughly 8 busy processes (a Chromium and a rela-server each)
  // on 4 cores, and document `command:` renderers fork a bubblewrap sandbox per
  // request through cmdexec, which FAILS CLOSED.
  //
  // The symptom was not a timeout: document-edit-button.spec.ts rendered "No
  // document content available" — its sandboxed `cat {in}` renderer failing
  // under CPU pressure — and burned both retries. It is intermittent (one run
  // at 4 passed, the next failed all 12 of that spec's tests), so a single
  // green run at a higher count proves nothing.
  //
  // Unlike internal/transform, which bounds concurrent conversions at 4,
  // internal/dataentry's document renderer builds its cmdexec runner with no
  // WithMaxConcurrent. Bounding it there is the prerequisite for raising this.
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
