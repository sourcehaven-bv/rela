// Hard-coded reproducer scripts for bugs found by the fuzzer.
//
// Each entry replays the minimal failing sequence the fuzzer shrunk to,
// in a single fresh BrowserContext, N times in a row, and reports how
// many of those N runs reproduced the failure. Use this to:
//   1. Prove a fuzzer-found bug is deterministic before filing.
//   2. Smoke-test the fix before declaring it done.
//
// Not registered as a Scenario in scenarios/index.ts because it doesn't
// fit the workload-driven shape. Run via cli.ts --mode=replay if you
// need it; for now it's documentation in code form.

export interface MinimalReproducer {
  id: string
  description: string
  // Sequence of action descriptors. The runner translates them.
  actions: Array<
    | { kind: 'goto'; list: string }
    | { kind: 'click-row'; index: number }
    | { kind: 'reload' }
    | { kind: 'back' }
    | { kind: 'wait'; ms: number }
  >
  // The console error substring we expect to see.
  expectedError: string
}

export const reproducers: MinimalReproducer[] = [
  {
    id: 'firefox-loadcommands-after-reload-back',
    description:
      'Firefox-only: opening a ticket, reloading the detail page, then ' +
      'navigating back produces "Failed to load commands: Error". ' +
      'Found by frontend/stress fuzzer in 35 s, 4 examples, 6 shrinks.',
    actions: [
      { kind: 'click-row', index: 0 },
      { kind: 'reload' },
      { kind: 'back' },
    ],
    expectedError: 'Failed to load commands',
  },
]
