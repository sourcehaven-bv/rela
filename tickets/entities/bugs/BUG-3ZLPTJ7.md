---
id: BUG-3ZLPTJ7
type: bug
title: useAutoSave two-tab property test fails at random on its coverage floor
description: 'useAutoSave.twoClients.prop.test.ts > merges what can be merged and reports the rest failed on PR 2081 (job 126009304990): saves out of attempts expected 28 to be greater than or equal to 30. The property itself held; the effect floors (exhausted >= numRuns / 10 and the others) depend on fast-check''s unseeded random runs, so a run can fall short. No seed in the log: fc.assert only prints one on a property failure. Fix: pin a seed for the floor assertions, or lower the floors to a bound the run meets with margin.'
priority: low
status: backlog
---

## Description

`frontend/src/composables/useAutoSave.twoClients.prop.test.ts`, test "merges
what can be merged and reports the rest" (from #2058, TKT-V270TMQ), failed in CI
on an unrelated PR (#2081, job 126009304990):

```
AssertionError: saves out of attempts: expected 28 to be greater than or equal to 30
  at useAutoSave.twoClients.prop.test.ts:569
```

The property held. The failing line is a coverage floor on the effects the runs
produced (`effect.exhausted >= numRuns / 10`, with `numRuns` 300). fast-check
runs unseeded, so the counts vary per run and can fall below a floor. The log
has no seed, because `fc.assert` prints one only when the property fails.

## Fix options

- Pass a fixed `seed` to `fc.assert` for this test, so the floors are checked on a known run set.
- Or keep the random runs and set each floor to a bound the runs meet with margin, measured over many seeds.
