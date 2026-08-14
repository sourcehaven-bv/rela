---
id: BUG-2OXEW0
type: bug
title: DynamicForm.test.ts leaks mounted components, flaking the Frontend CI job on unrelated PRs
description: All 15 tests in DynamicForm.test.ts mount the component and none unmount it. In-flight async work in the leaked components (loadEntity, template load, dry-run affordances) logs via console.warn/error after the test file finishes, racing vitest worker teardown. Vitest reports EnvironmentTeardownError as an unhandled error and exits 1 even though every test passed.
priority: medium
effort: s
status: ready
---

## Symptom

The Frontend CI job fails with **every test passing**:

```text
Test Files  101 passed (101)
Tests       1601 passed (1601)
Errors      1 error

⎯⎯⎯ Unhandled Rejection ⎯⎯⎯
EnvironmentTeardownError: [vitest-worker]: Closing rpc while
"onUserConsoleLog" was pending
This error originated in "src/components/forms/DynamicForm.test.ts"
```

Vitest counts an unhandled error as a failure, so the process exits 1.

Observed on PR #1310 — a **docs-only** PR whose entire diff is one markdown file
under `tickets/`, touching no frontend code. It passed on re-run of the same
commit with no changes, which is what confirms it as non-deterministic.

## Cause

`DynamicForm.test.ts` mounts the component 15 times and **never unmounts**:

```bash
$ grep -c 'it(' frontend/src/components/forms/DynamicForm.test.ts
15
# no wrapper.unmount(), no afterEach teardown
```

Each leaked component keeps running its async work. `DynamicForm.vue` logs from
several async error paths — `loadEntity` (`console.error`, ~:439 and ~:1059),
template loading (`console.warn`, ~:571), the create-mode dry-run
(`console.warn`, ~:634), and auto-link (~:1016).

When one of those settles *after* the test file completes, vitest's
`onUserConsoleLog` RPC is still in flight as the worker closes. Whether it wins
that race is timing-dependent, which is why it is intermittent and why it
surfaces on PRs that changed nothing.

## Why it matters beyond the noise

It red-flags unrelated PRs, and the failure text says nothing about the real
cause — it names a test file the author never touched. That is precisely the
shape of failure that trains people to re-run without reading, which erodes the
signal from every other CI failure.

## Fix

Unmount in an `afterEach`, so no component outlives its test:

```ts
let wrappers: VueWrapper[] = []
afterEach(() => {
  wrappers.forEach((w) => w.unmount())
  wrappers = []
})
```

`mountEdit` (the shared helper at ~:108) already funnels every mount through one
place, so registering the wrapper there covers all 15 call sites.

Worth checking whether the async paths should also be awaited or stubbed in the
harness — an unmount stops the component but does not necessarily settle a
promise already in flight. If flakes persist after the unmount fix, stub the
template-load and dry-run calls in `mountEdit`.

## Provenance

The test file arrived with #1277 (BUG-MLT9DE). Not introduced by BUG-FB0LN8,
though that ticket's PR is where the flake was first observed.
