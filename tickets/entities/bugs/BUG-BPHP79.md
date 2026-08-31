---
id: BUG-BPHP79
type: bug
title: testIdempotencyFreed races the queue completion bookkeeping
description: testIdempotencyFreed waits for the handler to have RUN (ran.inc() fires inside the handler body) and then immediately re-enqueues the same idempotency key, but the key is freed only once the queue records the job COMPLETE -- strictly later. On a contended runner that window opens and the second Enqueue returns ErrDuplicateJob. Deterministic under GOMAXPROCS=1; observed twice on CI. The queue behaviour is correct; the test's synchronisation is not.
why1: 'The second Enqueue returns ErrDuplicateJob because the idempotency key is still held.'
why2: 'The test re-enqueues as soon as ran.get() == 1, but ran.inc() fires INSIDE the handler body -- that establishes "the handler started", not "the job completed".'
why3: 'The key is freed only when the queue records completion, strictly after the handler returns. Between those two points the key is held and a re-enqueue is correctly rejected; under contention that window is wide enough to lose.'
why4: 'The test asserted on a PROXY for the event it cared about. `ran` was reachable from the test and completion was not, so the reachable signal stood in for the real one. The comment ("Same key again, after completion") shows the intent was right; the observable did not match it.'
why5: 'Nothing in the test''s shape reveals the substitution. require.Eventually LOOKS like careful async handling, so a reader sees a test that already waits properly and never asks whether it waits for the right thing. A time.Sleep would have looked suspicious; this did not.'
prevention: 'When a test waits for an async state transition, the thing it polls must be the transition itself, not a signal that merely correlates with it. Here the queue exposes no completion hook to the test, so the fix polls the OPERATION whose success defines the state (Enqueue succeeding == the key is free) rather than a handler-side counter. Systemic: a polling fix must be checked for retained teeth -- verified here by blocking the handler so the key is never freed and confirming the test still fails. Also worth generalising: an intermittent CI failure that is two-for-two on one branch and zero elsewhere is NOT randomness, and treating it as a flake (which I did first) costs a rerun and delays the diagnosis. Reach for load (GOMAXPROCS=1) as well as ordering (-shuffle) when reproducing.'
priority: medium
status: done
---

## Description

`testIdempotencyFreed` (`internal/jobs/jobstest/jobstest.go:669`) races against
the queue's own completion bookkeeping and fails intermittently in CI.

It waits for the handler to have RUN, then immediately re-enqueues the same
idempotency key:

```go
require.Eventually(t, func() bool { return ran.get() == 1 }, settleTimeout, pollInterval)

// Same key again, after completion: must queue.
require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
    Kind: "cycle", IdempotencyKey: "report",
}))
```

`ran.inc()` happens INSIDE the handler, but the key is freed only once the queue
records the job complete — strictly later. The comment says "after completion",
and the assertion it rests on only establishes "after the handler body started".
On a contended runner that window opens and the second `Enqueue` returns
`ErrDuplicateJob`.

## Reproduction

Deterministic under CPU pressure:

```
GOMAXPROCS=1 go test ./internal/jobs/ -race \
  -run 'TestMemoryQueue_Conformance/IdempotencyKeyFreed' -count=20
```

fails immediately with `jobs: an identical job is already pending`. It passes
consistently at full parallelism, including 5 consecutive `-race` runs and a run
with CI's exact `-shuffle` seed — which is why it presents as a flake.

Observed twice on CI for PR #1497, a branch that does not touch `internal/jobs`
(verified: `go list -deps ./internal/jobs/` reaches neither `internal/acl` nor
`internal/dataentry`).

## Proposed fix

Assert on the OBSERVABLE the test actually needs. Either:

- poll the second `Enqueue` until it succeeds, within `settleTimeout` — the key
becoming free IS the property under test, so waiting for it is not weakening the
assertion; or
- have the handler signal completion through a channel the queue closes after
recording, so the test waits for the real event rather than a proxy.

The first is smaller and keeps the test's shape. What it must NOT do is
`time.Sleep` a guess, which would trade a visible flake for a slow invisible
one.

## Note

The behaviour under test is correct — a completed key IS freed. This is a defect
in the test's synchronisation, not in the queue.
