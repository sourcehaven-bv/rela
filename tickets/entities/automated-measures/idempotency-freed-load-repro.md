---
id: idempotency-freed-load-repro
type: automated-measure
title: 'Run the jobs conformance suite under GOMAXPROCS=1 so completion races surface deterministically'
kind: test
location: internal/jobs (conformance suite; CI job or nightly)
status: proposed
description: >-
  BUG-BPHP79 was invisible at full parallelism -- 5 consecutive -race runs and a
  run with CI's exact -shuffle seed all passed -- and deterministic under
  GOMAXPROCS=1. The suite's async assertions are therefore only exercised in the
  scheduling regime where the windows they race are narrowest, which is the
  regime least likely to catch them.

  Running the jobs conformance suite once under GOMAXPROCS=1 (a separate CI step,
  or nightly if the runtime cost matters) turns that class of defect from an
  intermittent failure on an unrelated branch into a reproducible one on the
  branch that introduces it. The cost is small: the suite is ~70s, and the
  serialized run in the reproduction took ~2s for the single affected test.

  This is a MEASURE rather than a fix because it catches the class, not the
  instance: any future test in this suite that polls a proxy for a queue-side
  state transition -- the shape of BUG-BPHP79's why4 -- fails under it.
---

## Why this class needs a load dimension

`-shuffle=on` varies test ORDER, which is the right control for
inter-test state leakage. It does nothing for intra-test races between a
handler-side signal and the queue's own bookkeeping, because those depend on how
goroutines interleave rather than on which test ran first.

The bug's diagnosis turned on exactly that distinction: reaching for another
shuffle seed reproduced nothing, and reaching for `GOMAXPROCS=1` reproduced it in
one run. A measure that only varies order would not have found it.

## Scope

Deliberately narrow — the jobs conformance suite, not the whole tree. That suite
is the one whose subject is a concurrent queue, so its tests are the ones whose
async assumptions are load-bearing. Applying this repo-wide would multiply CI
time for packages where scheduling is not part of the contract.
