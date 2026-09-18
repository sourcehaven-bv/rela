---
id: BUG-7ZG8Q6
type: bug
title: 'The worlds-manual CI step is flaky: a 30s per-capture wall-clock deadline on a loaded runner'
description: 'The E2E job''s "Build the worlds manual (executable doc assertions)" step fails intermittently with a chromedp capture timing out, while the code under test is correct. Observed twice on PR #1593 at `manual:479: lua: <string>:2: context deadline exceeded` (the first screenshot{} of the manual), then PASSING on a re-run of the same commit with no code change. A local run with CI-equivalent settings also passed. Same class as BUG-TIMEFLAKE: perCaptureTimeout is a fixed 30s sized against fast, uncontended runs, and the manual step runs last in the E2E job, after the whole Playwright suite has loaded the runner.'
priority: medium
status: backlog
---

## Evidence

| Run | Commit | Result |
|---|---|---|
| 35185633991 | 48d4c13b | fail — `manual:479 … context deadline exceeded` |
| 35185989373 | 5c45f7cf | fail — same line, same message |
| 35185989373 (re-run, no code change) | 5c45f7cf | **pass** |

A re-run passing with no code change is what distinguishes this from a real
break. The pre-merge branch tip and `develop` both pass the step consistently,
which is why the first two failures looked deterministic.

## Why it is the BUG-TIMEFLAKE class

`internal/docscapture/capture.go` bounds one navigate+render+capture with
`perCaptureTimeout = 30 * time.Second`. That is a wall-clock deadline sized
against a fast, uncontended machine. The manual step is the LAST step of the E2E
job, so it runs on a runner that has just executed the full Playwright suite
plus a postgres container — precisely the loaded condition the measure already
names.

The failing island is the first `screenshot{}` in the document, so it is also
the first time the browser navigates to the SPA in that process: it pays
first-load cost (bundle parse, initial queries) inside the same 30s budget as
every later capture, which only pay a warm re-navigate.

## Two different timeout lines

Reproducing locally produced a failure at a DIFFERENT island (`manual:936`,
`waited for 1 version rows on POL-4@draft … the version sweep had not captured
them in time`) when the sweep env vars were omitted. So the manual has at least
two independent wall-clock budgets that can expire under load: the capture
deadline and `versionAwaitTimeout`. Both should be reconsidered together.

## Remedy direction

Per AM-fixture-timeouts-are-not-wall-clock, wait on the condition rather than a
fixed deadline, or make the budget explicit and generous for a CI build the way
`RELA_VERSION_SWEEP_*` already is for the sweep. Note the capture already polls
for an observable signal (`renderabilityGate` waits for `page-state-*` to leave
`pending`) — what is fixed is only the ceiling on that poll, so the cheapest
correct fix is likely an env-tunable ceiling used by the CI step, not a rewrite
of the gate.

## Not a blocker for #1593

Investigated during that PR's merge with develop. Two hypotheses were raised and
disproved: per-request query-scope recompilation (measured at 0.38ms per list
request, four orders of magnitude short of the timeout) and the SPA now sending
`list_id` unconditionally (the manual project's `policies` list declares no
`condition:`, so the resolved matcher is nil and the read path is unchanged).
