---
id: component-test-unmount-teardown
type: automated-measure
title: Unit tests fail closed on any real HTTP request
description: Ensures a mounted component makes no real HTTP request and cannot keep running async work after its test finishes. An unstubbed child that fetches on mount fails ECONNREFUSED against happy-dom's default origin and logs via console.error; when that lands during vitest worker teardown it is reported as an unhandled error, failing CI with every test passing.
kind: test
location: frontend/src/test/setup.ts (axios adapter guard - the enforcing mechanism) plus stubs/teardown in frontend/src/components/forms/DynamicForm*.test.ts
status: active
---

Regression pin for BUG-2OXEW0.

**The enforcing mechanism is the guard, not the stub lists.**
`src/test/setup.ts` replaces the axios adapter with one that throws
synchronously on any unmocked request. Synchronous matters: the failure lands
at the call site in the test that caused it, rather than as a teardown crash
naming an unrelated file. Axios uses the Node http adapter under happy-dom, so
patching fetch/XMLHttpRequest would catch nothing.

Verified: full suite from 21 stray requests to 0, all 1662 tests still passing.

Two supporting invariants:

**1. No test may issue a real HTTP request.** `SidePanel` renders only when
`isEdit && entityId` and fetches on mount, so an edit-mode `DynamicForm` test
that does not stub it makes a live request per mount — 42 of them in this one
file before the fix. `getTemplates` is stubbed through `@/api` for the same
reason, matching the sibling form tests.

**2. Every mount is unmounted in `afterEach`.** The shared mount helper
registers each wrapper; because all mounts funnel through it, no individual test
has to remember.

The ordering was learned twice, the hard way. First: **unmounting alone does
not fix this.** Unmount stops the component but does not cancel an in-flight
request, so the rejection still settles and still logs. Measured at 2 failures
in 10 full-suite runs with teardown in place and `SidePanel` still unstubbed.
Invariant 2 is real hygiene — it removes the latent version of this bug for any
future unstubbed call — but invariant 1 is the fix.

Second: **stubbing the named file does not fix this either.** With SidePanel
stubbed, `EntityList.test.ts` was still issuing 15 real requests via an
unstubbed `ExportMenu` — found by code review, not by 20 green runs. A stub
list is a denylist maintained by whoever last read a stack trace; it can only
ever cover the file that happened to lose the race. That is why the guard, and
not the stubs, is the measure.

Why a measure rather than "retry the flaky job": the failure mode is silent and
misattributed. Every test passes, the exit code is 1, and the error names a file
the PR author never touched. Retrying hides it; the guard removes the race.
