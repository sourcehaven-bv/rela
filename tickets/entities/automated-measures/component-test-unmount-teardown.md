---
id: component-test-unmount-teardown
type: automated-measure
title: Component tests stub network-calling children and unmount every mount
description: Ensures a mounted component makes no real HTTP request and cannot keep running async work after its test finishes. An unstubbed child that fetches on mount fails ECONNREFUSED against happy-dom's default origin and logs via console.error; when that lands during vitest worker teardown it is reported as an unhandled error, failing CI with every test passing.
kind: test
location: frontend/src/components/forms/DynamicForm*.test.ts (SidePanel/@api stubs + afterEach teardown); the suite-wide network guard lives in frontend/src/test/setup.ts and is pinned by AM-frontend-tests-no-network
status: active
---

Regression pin for BUG-2OXEW0.

**The enforcing mechanism is the suite-wide adapter stub in
`src/test/setup.ts`**, which BUG-762I34 landed independently on develop while
this work was in flight. Both bugs are the same class seen from different
files — BUG-2OXEW0 via SidePanel, BUG-762I34 via ExportMenu — and develop's
resolving stub is authoritative. It is pinned by AM-frontend-tests-no-network;
this measure covers the per-file hygiene that sits on top of it.

Verified while diagnosing: the full suite went from 21 stray requests to 0.

Two invariants this measure pins:

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
