---
id: BUG-BC90GPD
type: bug
title: TestSSEACL tests flake on a fixed settle() sleep
description: sse_acl_test.go waits a fixed sseFlushInterval+80ms before reading the SSE sinks; on a loaded runner the flush lands later and the sinks are empty.
priority: low
status: backlog
---

## Symptom

`TestSSEACL_TwoPrincipalsDifferentFrames`
(`internal/dataentry/sse_acl_test.go:275`) failed in the merge queue on a
docs-only PR (#2256, run 43582373554). Both checks fired with empty sinks:

```
sse_acl_test.go:300: alice (ticket-only) frames wrong:
sse_acl_test.go:303: bob (feature-only) frames wrong:
```

The rerun passed.

## Cause

The test sends events, then calls `settle()`, which is a fixed
`time.Sleep(sseFlushInterval + 80*time.Millisecond)`, and reads both sinks. On a
loaded runner the flush lands after that window, so the sinks are still empty.

A late flush affects the other `TestSSEACL_*` tests in two different ways:

- Tests that assert a frame arrived can fail falsely, the same way as above.
- `TestSSEACL_DenyAllWithholds`, `TestSSEACL_FailClosedOnZeroVerdict` and the
first check of `TestSSEACL_MembershipChangeReGates` only assert that no
`entity:changed` frame is in the sink. A frame that lands after the window is
never seen, so these checks can pass falsely.

`TestSSEACL_NonEntityEventsPassThrough` does not use `settle()`. It sleeps a
fixed 50 ms for the ungated frames, so it can also fail falsely on a loaded
runner.

## Fix

Drive the flush deterministically instead of sleeping, for example with the
manual clock used by the live tests (`liveStepper` in `live_served_test.go`).
Waiting on the sink for the expected frames with a deadline fixes the tests that
assert a frame arrived, but not the negative checks: they expect no frame, so
there is nothing to wait for. A manual clock lets them read the sink after a
flush has provably run.
