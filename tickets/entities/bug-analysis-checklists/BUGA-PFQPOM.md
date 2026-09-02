---
id: BUGA-PFQPOM
type: bug-analysis-checklist
title: 'Analysis: testIdempotencyFreed races the queue completion bookkeeping'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Deterministic under CPU pressure:

```
GOMAXPROCS=1 go test ./internal/jobs/ -race \
  -run 'TestMemoryQueue_Conformance/IdempotencyKeyFreed' -count=20
```

fails immediately with `jobs: an identical job is already pending`.

**Environment matters here, and misled me first.** At full parallelism it passes
consistently: 5 consecutive `-race` runs (341s) and a run with CI's exact
`-shuffle` seed all green. That is why it presented as a flake and why I
initially reported it as one — a conclusion the second CI failure disproved.

Observed twice on CI for PR #1497, a branch that does not touch `internal/jobs`.
Verified structurally rather than by inspection: `go list -deps
./internal/jobs/` reaches neither `internal/acl` nor `internal/dataentry`, so
the changed packages are unreachable from the failing one.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

**why1** — The second `Enqueue` returns `ErrDuplicateJob` because the
idempotency key is still held.

**why2** — The test re-enqueues as soon as `ran.get() == 1`, but `ran.inc()`
fires INSIDE the handler body. That establishes "the handler started", not "the
job completed".

**why3** — The key is freed only when the queue records completion, which is
strictly after the handler returns. Between those two points the key is held and
a re-enqueue is correctly rejected. Under contention that window is wide enough
to lose.

**why4** — The test asserted on a PROXY for the event it cared about. `ran` was
reachable from the test, and completion was not, so the reachable signal stood
in for the real one. The comment says "Same key again, after completion" — the
intent was right; the observable did not match it.

**why5** — Nothing in the test's shape reveals the substitution.
`require.Eventually` looks like careful async handling, so a reader sees a test
that already waits properly and does not ask whether it waits for the RIGHT
thing. A `time.Sleep` would have looked suspicious; this did not.

The queue behaviour is CORRECT throughout. A completed key is freed. This is a
defect in the test's synchronisation only, which is why it had to be diagnosed
rather than fixed by changing the code under test.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach.** Poll the second `Enqueue` inside `require.Eventually` until it
succeeds. The key becoming free IS the property under test, so waiting for it is
the assertion rather than a weakening of it.

Explicitly rejected: `time.Sleep` before the re-enqueue (trades a visible flake
for a slow invisible one) and ignoring `ErrDuplicateJob` (that error is the
failure this test exists to catch).

**Regression test.** The fixed test IS the regression test, so the risk is that
polling makes it unable to fail. Verified by mutation: blocking the handler so
the job never completes — and the key therefore never frees — still fails, after
the full `settleTimeout`. So the fix removes the race without removing the
assertion.

**Related areas.** Checked the sibling `testIdempotencyCollapses`: it holds the
handler open on a channel and asserts the duplicate IS rejected while the first
is still pending. That direction has no race — it asserts the state that exists
during the window rather than after it. No other test in `jobstest.go` waits on
a handler-side counter to infer a queue-side state transition.
