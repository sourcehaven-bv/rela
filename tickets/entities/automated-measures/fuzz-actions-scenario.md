---
id: fuzz-actions-scenario
type: automated-measure
title: Property-based fuzzer for SPA action sequences
description: 'fast-check based fuzzer that generates random sequences of UI actions (goto / click-row / reload / back / wait / touch-file), replays them in a fresh Firefox or Chromium BrowserContext, then runs a liveness oracle (a fresh navigation must complete within 5s and produce a usable list view, with no non-benign console errors). On failure fast-check shrinks to a minimal counter-example. Found BUG-6C3V in 35 seconds across 4 examples and 6 shrinks, then reproduced via a different seed in <40s. Run via: npm run stress -- --mode=fuzz --browser=firefox --num-runs=200.'
kind: test
location: frontend/stress/fuzzRunner.ts + frontend/stress/scenarios/replay-minimal.ts
status: active
---
